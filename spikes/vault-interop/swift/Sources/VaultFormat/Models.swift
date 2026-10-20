// SPDX-License-Identifier: AGPL-3.0-only
import Foundation

public struct KDF: Codable, Sendable {
    public var name: String
    public var version: UInt32
    public var memory_kib: UInt32
    public var iterations: UInt32
    public var parallelism: UInt32
    public var salt: String
}
public struct PasswordSlot: Codable, Sendable {
    public var kdf: KDF
    public var nonce: String
    public var wrapped_root: String
}
public struct RecoverySlot: Codable, Sendable {
    public var nonce: String
    public var wrapped_root: String
}
public struct Entry: Codable, Sendable {
    public var id: String
    public var revision: UInt32
    public var key_nonce: String
    public var wrapped_key: String
    public var payload_nonce: String
    public var payload: String
}
public struct VaultFile: Codable, Sendable {
    public var format: String
    public var version: UInt32
    public var vault_id: String
    public var epoch: UInt32
    public var generation: UInt32
    public var password: PasswordSlot
    public var recovery: RecoverySlot
    public var entries: [Entry]
    public var mac: String
}
public enum VaultError: Error { case invalid, unlock }
