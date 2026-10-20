// SPDX-License-Identifier: AGPL-3.0-only
import Foundation
import VaultFormat

struct SyntheticSecrets: Codable {
    let password: String
    let recovery: String
    let plaintexts: [String]
}
@main
struct Main {
    static func read(_ path: String, limit: Int) throws -> Data {
        let file = try FileHandle(forReadingFrom: URL(fileURLWithPath: path))
        defer { try? file.close() }
        return try file.read(upToCount: limit + 1) ?? Data()
    }
    static func main() {
        do {
            let args = Array(CommandLine.arguments.dropFirst())
            guard args.count >= 2 else { throw VaultError.invalid }
            let start = ProcessInfo.processInfo.systemUptime
            if args[0] == "emit" {
                let password = "synthetisch – Masterpasswort \0 😀"
                let payloads = [Data("synthetisch: Grüße \0 😀".utf8), Data(), Data([0,255,128])]
                let (vault, recovery) = try VaultCodec.create(password: password, payloads: payloads)
                let directory = URL(fileURLWithPath: args[1])
                try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])
                try VaultCodec.canonical(vault).write(to: directory.appendingPathComponent("vault.json"))
                let secrets = SyntheticSecrets(password: password, recovery: recovery.base64EncodedString(), plaintexts: payloads.map { $0.base64EncodedString() })
                try JSONEncoder().encode(secrets).write(to: directory.appendingPathComponent("synthetic-secrets.json"))
            } else if args[0] == "verify", args.count >= 4 {
                let data = try read(args[1], limit: VaultCodec.maxFileBytes)
                let secretData = try read(args[2], limit: 65536)
                guard secretData.count <= 65536 else { throw VaultError.invalid }
                let secrets = try JSONDecoder().decode(SyntheticSecrets.self, from: secretData)
                let recovery: Data?
                if args[3] == "recovery" {
                    guard let decoded = Data(base64Encoded: secrets.recovery) else { throw VaultError.invalid }
                    recovery = decoded
                } else { guard args[3] == "password" else { throw VaultError.invalid }; recovery = nil }
                let minimum: UInt32
                if args.count > 4 { guard let value = UInt32(args[4]) else { throw VaultError.invalid }; minimum = value } else { minimum = 0 }
                let plaintexts = try VaultCodec.unlock(data, password: secrets.password, recovery: recovery, minimumGeneration: minimum)
                guard plaintexts.map({ $0.base64EncodedString() }) == secrets.plaintexts else { throw VaultError.invalid }
            } else { throw VaultError.invalid }
            let duration = (ProcessInfo.processInfo.systemUptime - start) * 1000
            print("{\"ok\":true,\"duration_ms\":\(duration)}")
        } catch { print("{\"ok\":false,\"code\":\"rejected\"}"); exit(1) }
    }
}
