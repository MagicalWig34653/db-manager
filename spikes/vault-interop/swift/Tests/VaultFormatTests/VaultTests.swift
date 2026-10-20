// SPDX-License-Identifier: AGPL-3.0-only
import CryptoKit
import Foundation
import Testing
@testable import VaultFormat

@Test func independentPasswordAndRecovery() throws {
    let (vault,recovery) = try VaultCodec.create(password: "synthetic-password", payloads: [Data("synthetic payload".utf8)])
    let data = try VaultCodec.canonical(vault)
    #expect(try VaultCodec.unlock(data, password: "synthetic-password").count == 1)
    #expect(try VaultCodec.unlock(data, recovery: recovery).count == 1)
    #expect(throws: (any Error).self) { try VaultCodec.unlock(data, password: "wrong-password") }
    #expect(throws: (any Error).self) { try VaultCodec.unlock(data, recovery: Data(repeating: 0, count: 32)) }
    #expect(throws: (any Error).self) { try VaultCodec.unlock(data, password: "synthetic-password", minimumGeneration: 2) }
}
@Test func canonicalBoundsAndManipulation() throws {
    var (vault,recovery) = try VaultCodec.create(password: "synthetic-password", payloads: [Data()])
    let data = try VaultCodec.canonical(vault)
    for size in 0..<data.count { #expect(throws: (any Error).self) { try VaultCodec.parse(data.prefix(size)) } }
    #expect(throws: (any Error).self) { try VaultCodec.parse(data + Data([32])) }
    vault.password.kdf.memory_kib = UInt32.max
    #expect(throws: (any Error).self) { try VaultCodec.parse(VaultCodec.canonical(vault)) }
    vault.password.kdf.memory_kib = 65536
    vault.entries = []
    #expect(throws: (any Error).self) { try VaultCodec.unlock(VaultCodec.canonical(vault), recovery: recovery) }
}
@Test func cryptographicPurposeBinding() throws {
    let (vault,_) = try VaultCodec.create(password: "synthetic-password", payloads: [Data("synthetic".utf8)])
    let key = SymmetricKey(size: .bits256)
    let (nonce,ciphertext) = try VaultCodec.seal(key, aad: VaultCodec.aad(vault, purpose: 3, entry: vault.entries[0]), plaintext: Data("synthetic".utf8))
    #expect(throws: (any Error).self) { try VaultCodec.unseal(key, aad: VaultCodec.aad(vault, purpose: 4, entry: vault.entries[0]), nonce: nonce, ciphertext: ciphertext) }
}

@Test func fixedAADAndAuthenticatedContextSwap() throws {
    var (vault,recovery) = try VaultCodec.create(password: "synthetic-password", payloads: [Data("synthetic".utf8)])
    var reference = vault
    reference.vault_id = "000102030405060708090a0b0c0d0e0f"; reference.epoch = 0x01020304
    var entry = vault.entries[0]
    entry.id = "101112131415161718191a1b1c1d1e1f"; entry.revision = 0x0a0b0c0d
    let expected = "44424d56310003000102030405060708090a0b0c0d0e0f01020304101112131415161718191a1b1c1d1e1f0a0b0c0d"
    #expect(try VaultCodec.hex(VaultCodec.aad(reference, purpose: 3, entry: entry)) == expected)
    let root = try VaultCodec.unseal(VaultCodec.derived(recovery, vault: vault, purpose: "recovery"), aad: VaultCodec.aad(vault, purpose: 2), nonce: vault.recovery.nonce, ciphertext: vault.recovery.wrapped_root)
    vault.entries[0].revision += 1
    vault.mac = try VaultCodec.mac(vault, root: root).base64EncodedString()
    #expect(throws: (any Error).self) { try VaultCodec.unlock(VaultCodec.canonical(vault), recovery: recovery) }
}
