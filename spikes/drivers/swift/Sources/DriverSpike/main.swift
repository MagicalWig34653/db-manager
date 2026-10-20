// SPDX-License-Identifier: AGPL-3.0-only
import Foundation
import MySQLNIO
import PostgresNIO
import NIOCore
import NIOPosix
import Logging
import SQLite3

do {
var database: OpaquePointer?
precondition(sqlite3_open(":memory:", &database) == SQLITE_OK)
defer { sqlite3_close(database) }
precondition(sqlite3_exec(database, "CREATE TABLE probe(value TEXT); INSERT INTO probe VALUES ('Grüße');", nil, nil, nil) == SQLITE_OK)
var statement: OpaquePointer?
precondition(sqlite3_prepare_v2(database, "SELECT value FROM probe", -1, &statement, nil) == SQLITE_OK)
defer { sqlite3_finalize(statement) }
precondition(sqlite3_step(statement) == SQLITE_ROW)
precondition(String(cString: sqlite3_column_text(statement, 0)) == "Grüße")
print("SQLite UTF-8 round-trip passed")

if ProcessInfo.processInfo.environment["SPIKE_LIVE"] == "1" {
    let group = MultiThreadedEventLoopGroup(numberOfThreads: 1)
    defer { try! group.syncShutdownGracefully() }
    let ports = (ProcessInfo.processInfo.environment["SPIKE_MYSQL_PORTS"] ?? "23306,23307").split(separator: ",").compactMap { Int($0) }
    for port in ports {
        let mysql = try MySQLConnection.connect(
            to: .makeAddressResolvingHost("127.0.0.1", port: port),
            username: "spike", database: "probe", password: "public-spike-fixture",
            tlsConfiguration: nil, on: group.next()
        ).wait()
        defer { try! mysql.close().wait() }
        let rows = try mysql.simpleQuery("SELECT 'Grüße' AS value, NULL AS empty, CAST(18446744073709551615 AS UNSIGNED) AS large").wait()
        precondition(rows.count == 1)
        precondition(rows[0].column("value")?.string == "Grüße")
        precondition(rows[0].column("large")?.uint64 == UInt64.max)
        print("MySQL protocol on port \(port): UTF-8 and UInt64 round-trip passed")
    }
    let postgres = try PostgresConnection.connect(
        on: group.next(), configuration: .init(host: "127.0.0.1", port: 25432,
        username: "postgres", password: nil, database: "probe", tls: .disable),
        id: 1, logger: Logger(label: "driver-spike")
    ).wait()
    defer { try! postgres.close().wait() }
    let result = try postgres.query("SELECT 'Grüße'::text, 9223372036854775807::bigint", logger: Logger(label: "driver-spike")).wait()
    precondition(result.rows.count == 1)
    let (text, large) = try result.rows[0].decode((String, Int64).self)
    precondition(text == "Grüße" && large == Int64.max)
    print("PostgreSQL UTF-8 and Int64 round-trip passed")
}

if ProcessInfo.processInfo.environment["SPIKE_TLS_DOWNGRADE"] == "1" {
    let group = MultiThreadedEventLoopGroup(numberOfThreads: 1)
    defer { try! group.syncShutdownGracefully() }
    let mysql = try MySQLConnection.connect(
        to: .makeAddressResolvingHost("127.0.0.1", port: 23308),
        username: "spike", database: "probe", password: "public-spike-fixture",
        serverHostname: "localhost", on: group.next()
    ).wait()
    defer { try! mysql.close().wait() }
    let rows = try mysql.simpleQuery("SHOW SESSION STATUS LIKE 'Ssl_cipher'").wait()
    precondition(rows.count == 1 && rows[0].column("Value")?.string == "")
    print("CONFIRMED: default TLS configuration accepted a server without TLS; session cipher is empty")
}

} catch {
    FileHandle.standardError.write(Data("Driver spike failed: \(error)\n".utf8))
    exit(1)
}
