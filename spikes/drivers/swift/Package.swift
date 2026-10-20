// swift-tools-version: 6.1
// SPDX-License-Identifier: AGPL-3.0-only
import PackageDescription
let package = Package(
    name: "DriverSpike",
    platforms: [.macOS(.v14), .iOS(.v17)],
    dependencies: [
        .package(url: "https://github.com/vapor/mysql-nio.git", from: "1.9.1"),
        .package(url: "https://github.com/vapor/postgres-nio.git", from: "1.27.0")
    ],
    targets: [.executableTarget(name: "DriverSpike", dependencies: [
        .product(name: "MySQLNIO", package: "mysql-nio"),
        .product(name: "PostgresNIO", package: "postgres-nio")
    ])]
)
