// SPDX-License-Identifier: AGPL-3.0-only
import CArgon2
import CryptoKit
import Foundation
import Security

public enum VaultCodec {
    public static let maxFileBytes = 8 << 20
    public static let maxEntries = 64
    public static let maxPayloadBytes = 64 << 10

    static func random(_ count: Int) throws -> Data {
        var bytes = Data(repeating: 0, count: count)
        let status = bytes.withUnsafeMutableBytes { SecRandomCopyBytes(kSecRandomDefault, count, $0.baseAddress!) }
        guard status == errSecSuccess else { throw VaultError.invalid }
        return bytes
    }
    static func decode(_ text: String, count: Int) throws -> Data {
        guard let data = Data(base64Encoded: text), data.count == count,
              data.base64EncodedString() == text else { throw VaultError.invalid }
        return data
    }
    static func identifier(_ text: String) throws -> Data {
        let bytes = Array(text.utf8)
        guard bytes.count == 32, bytes.allSatisfy({ (48...57).contains($0) || (97...102).contains($0) }) else { throw VaultError.invalid }
        var result = Data()
        for i in stride(from: 0, to: 32, by: 2) {
            guard let byte = UInt8(String(decoding: bytes[i...i+1], as: UTF8.self), radix: 16) else { throw VaultError.invalid }
            result.append(byte)
        }
        return result
    }
    static func hex(_ data: Data) -> String { data.map { String(format: "%02x", $0) }.joined() }
    public static func canonical(_ vault: VaultFile) throws -> Data {
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.sortedKeys, .withoutEscapingSlashes]
        return try encoder.encode(vault)
    }
    public static func parse(_ data: Data) throws -> VaultFile {
        guard !data.isEmpty, data.count <= maxFileBytes else { throw VaultError.invalid }
        var quoted = false; var depth = 0
        for byte in data {
            guard byte < 128, byte != 92 else { throw VaultError.invalid }
            if byte == 34 { quoted.toggle(); continue }
            if !quoted {
                if byte == 123 || byte == 91 { depth += 1 }
                if byte == 125 || byte == 93 { depth -= 1 }
                guard depth >= 0, depth <= 4 else { throw VaultError.invalid }
            }
        }
        guard !quoted, depth == 0 else { throw VaultError.invalid }
        let vault = try JSONDecoder().decode(VaultFile.self, from: data)
        guard try canonical(vault) == data else { throw VaultError.invalid }
        try validate(vault)
        return vault
    }
    static func validate(_ v: VaultFile) throws {
        guard v.format == "db-manager-vault", v.version == 1, v.epoch > 0,
              v.generation > 0, v.entries.count <= maxEntries else { throw VaultError.invalid }
        _ = try identifier(v.vault_id)
        let k = v.password.kdf
        guard k.name == "argon2id", k.version == 19, k.memory_kib == 65536,
              k.iterations == 3, k.parallelism == 4 else { throw VaultError.invalid }
        for (s, n) in [(k.salt,16), (v.password.nonce,12), (v.password.wrapped_root,48),
                       (v.recovery.nonce,12), (v.recovery.wrapped_root,48), (v.mac,32)] {
            _ = try decode(s, count: n)
        }
        var seen = Set<String>()
        var nonces = Set<String>()
        for e in v.entries {
            _ = try identifier(e.id)
            guard e.revision > 0, seen.insert(e.id).inserted else { throw VaultError.invalid }
            guard nonces.insert(e.key_nonce).inserted else { throw VaultError.invalid }
            for (s,n) in [(e.key_nonce,12), (e.wrapped_key,48), (e.payload_nonce,12)] { _ = try decode(s, count: n) }
            guard e.payload.utf8.count <= 87404, let data = Data(base64Encoded: e.payload),
                  data.count >= 16, data.count <= maxPayloadBytes + 16,
                  data.base64EncodedString() == e.payload else { throw VaultError.invalid }
        }
    }
    static func u32(_ value: UInt32) -> Data {
        Data([UInt8(truncatingIfNeeded: value >> 24), UInt8(truncatingIfNeeded: value >> 16),
              UInt8(truncatingIfNeeded: value >> 8), UInt8(truncatingIfNeeded: value)])
    }
    static func aad(_ v: VaultFile, purpose: UInt8, entry: Entry? = nil) throws -> Data {
        var result = Data("DBMV1\0".utf8) + Data([purpose]) + (try identifier(v.vault_id)) + u32(v.epoch)
        if purpose == 1 {
            let k = v.password.kdf
            result += u32(k.memory_kib) + u32(k.iterations) + u32(k.parallelism) + (try decode(k.salt, count: 16))
        }
        if let entry { result += (try identifier(entry.id)) + u32(entry.revision) }
        return result
    }
    static func derived(_ secret: Data, vault: VaultFile, purpose: String) throws -> SymmetricKey {
        HKDF<SHA256>.deriveKey(inputKeyMaterial: SymmetricKey(data: secret), salt: try identifier(vault.vault_id),
                              info: Data(("DBMV1\0" + purpose + "\0").utf8) + u32(vault.epoch), outputByteCount: 32)
    }
    static func seal(_ key: SymmetricKey, aad: Data, plaintext: Data) throws -> (String, String) {
        guard key.bitCount == 256 else { throw VaultError.invalid }
        let nonce = try AES.GCM.Nonce(data: random(12))
        let box = try AES.GCM.seal(plaintext, using: key, nonce: nonce, authenticating: aad)
        return (box.nonce.withUnsafeBytes { Data($0).base64EncodedString() }, (box.ciphertext + box.tag).base64EncodedString())
    }
    static func unseal(_ key: SymmetricKey, aad: Data, nonce: String, ciphertext: String) throws -> Data {
        guard key.bitCount == 256 else { throw VaultError.unlock }
        guard let data = Data(base64Encoded: ciphertext), data.count >= 16 else { throw VaultError.unlock }
        let n = try AES.GCM.Nonce(data: decode(nonce, count: 12))
        let box = try AES.GCM.SealedBox(nonce: n, ciphertext: data.dropLast(16), tag: data.suffix(16))
        return try AES.GCM.open(box, using: key, authenticating: aad)
    }
    static func passwordKey(_ password: String, kdf: KDF) throws -> SymmetricKey {
        let bytes = Data(password.utf8)
        guard !bytes.isEmpty, bytes.count <= 1024 else { throw VaultError.unlock }
        let salt = try decode(kdf.salt, count: 16)
        var output = Data(repeating: 0, count: 32)
        let code = output.withUnsafeMutableBytes { out in
            bytes.withUnsafeBytes { password in
                salt.withUnsafeBytes { salt in
                    argon2id_hash_raw(kdf.iterations, kdf.memory_kib, kdf.parallelism,
                                      password.baseAddress, password.count, salt.baseAddress, salt.count,
                                      out.baseAddress, out.count)
                }
            }
        }
        guard code == ARGON2_OK.rawValue else { throw VaultError.unlock }
        return SymmetricKey(data: output)
    }
    static func manifest(_ v: VaultFile) throws -> Data {
        var out = try aad(v, purpose: 5) + u32(v.generation)
        out += try aad(v, purpose: 1).dropFirst(27)
        for s in [v.password.nonce, v.password.wrapped_root, v.recovery.nonce, v.recovery.wrapped_root] {
            guard let data = Data(base64Encoded: s) else { throw VaultError.invalid }; out += data
        }
        out += u32(UInt32(v.entries.count))
        for e in v.entries {
            out += (try identifier(e.id)) + u32(e.revision)
            for s in [e.key_nonce, e.wrapped_key, e.payload_nonce] {
                guard let data = Data(base64Encoded: s) else { throw VaultError.invalid }; out += data
            }
            guard let payload = Data(base64Encoded: e.payload) else { throw VaultError.invalid }
            out += u32(UInt32(payload.count)) + payload
        }
        return out
    }
    static func mac(_ vault: VaultFile, root: Data) throws -> Data {
        Data(HMAC<SHA256>.authenticationCode(for: try manifest(vault), using: try derived(root, vault: vault, purpose: "manifest")))
    }
    public static func create(password: String, payloads: [Data]) throws -> (VaultFile, Data) {
        guard payloads.count <= maxEntries, payloads.allSatisfy({ $0.count <= maxPayloadBytes }) else { throw VaultError.invalid }
        let kdf = KDF(name: "argon2id", version: 19, memory_kib: 65536, iterations: 3, parallelism: 4,
                      salt: try random(16).base64EncodedString())
        var v = VaultFile(format: "db-manager-vault", version: 1, vault_id: hex(try random(16)), epoch: 1, generation: 1,
                          password: PasswordSlot(kdf: kdf, nonce: "", wrapped_root: ""),
                          recovery: RecoverySlot(nonce: "", wrapped_root: ""), entries: [], mac: "")
        let root = try random(32); let recovery = try random(32)
        (v.password.nonce, v.password.wrapped_root) = try seal(passwordKey(password, kdf: kdf), aad: aad(v, purpose: 1), plaintext: root)
        (v.recovery.nonce, v.recovery.wrapped_root) = try seal(derived(recovery, vault: v, purpose: "recovery"), aad: aad(v, purpose: 2), plaintext: root)
        for payload in payloads {
            var e = Entry(id: hex(try random(16)), revision: 1, key_nonce: "", wrapped_key: "", payload_nonce: "", payload: "")
            let key = try random(32)
            (e.key_nonce, e.wrapped_key) = try seal(SymmetricKey(data: root), aad: aad(v, purpose: 3, entry: e), plaintext: key)
            (e.payload_nonce, e.payload) = try seal(SymmetricKey(data: key), aad: aad(v, purpose: 4, entry: e), plaintext: payload)
            v.entries.append(e)
        }
        v.mac = try mac(v, root: root).base64EncodedString()
        try validate(v)
        return (v, recovery)
    }
    public static func unlock(_ data: Data, password: String = "", recovery: Data? = nil,
                              minimumGeneration: UInt32 = 0) throws -> [Data] {
        let v = try parse(data)
        guard v.generation >= minimumGeneration else { throw VaultError.invalid }
        let root: Data
        do {
            if let recovery {
                guard recovery.count == 32 else { throw VaultError.unlock }
                root = try unseal(derived(recovery, vault: v, purpose: "recovery"), aad: aad(v, purpose: 2), nonce: v.recovery.nonce, ciphertext: v.recovery.wrapped_root)
            } else {
                root = try unseal(passwordKey(password, kdf: v.password.kdf), aad: aad(v, purpose: 1), nonce: v.password.nonce, ciphertext: v.password.wrapped_root)
            }
            guard HMAC<SHA256>.isValidAuthenticationCode(try decode(v.mac, count: 32), authenticating: try manifest(v),
                                                        using: try derived(root, vault: v, purpose: "manifest")) else { throw VaultError.unlock }
            return try v.entries.map { e in
                let key = try unseal(SymmetricKey(data: root), aad: aad(v, purpose: 3, entry: e), nonce: e.key_nonce, ciphertext: e.wrapped_key)
                return try unseal(SymmetricKey(data: key), aad: aad(v, purpose: 4, entry: e), nonce: e.payload_nonce, ciphertext: e.payload)
            }
        } catch { throw VaultError.unlock }
    }
}
