// swift-tools-version: 6.0
// SPDX-License-Identifier: AGPL-3.0-only
import PackageDescription

let package = Package(name: "VaultInterop", platforms: [.macOS(.v14), .iOS(.v17)], products: [
    .library(name: "VaultFormat", targets: ["VaultFormat"]),
    .executable(name: "vault-spike", targets: ["VaultSpike"])
], targets: [
    .target(name: "CArgon2", path: "Vendor/argon2", exclude: ["LICENSE", "upstream-lock.json"],
            sources: ["src/argon2.c", "src/core.c", "src/encoding.c", "src/ref.c", "src/thread.c", "src/blake2/blake2b.c"],
            publicHeadersPath: "include", cSettings: [.headerSearchPath("src"), .define("ARGON2_NO_THREADS")]),
    .target(name: "VaultFormat", dependencies: ["CArgon2"]),
    .executableTarget(name: "VaultSpike", dependencies: ["VaultFormat"]),
    .testTarget(name: "VaultFormatTests", dependencies: ["VaultFormat"])
])
