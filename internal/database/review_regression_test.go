// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"bytes"
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/httpapi"
	mysql "github.com/go-sql-driver/mysql"
)

func TestSQLiteRuntimeStorageClassesOverHTTP(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	execute(t, e, c, "CREATE TABLE dynamic(value, integer_value INTEGER, numeric_value NUMERIC, text_value TEXT)")
	cases := []struct {
		sql  string
		cell core.Cell
	}{
		{"'not-a-number'", core.Cell{Kind: "text", Value: "not-a-number"}},
		{"42", core.Cell{Kind: "integer", Value: "42"}},
		{"1.25", core.Cell{Kind: "float", Value: "1.25"}},
		{"X'3132'", core.Cell{Kind: "binary", Value: "MTI="}},
		{"X'FF'", core.Cell{Kind: "binary", Value: "/w=="}},
		{"X''", core.Cell{Kind: "binary", Value: ""}},
		{"NULL", core.Cell{Kind: "null"}},
		{"''", core.Cell{Kind: "text", Value: ""}},
	}
	for _, tc := range cases {
		execute(t, e, c, "INSERT INTO dynamic VALUES("+tc.sql+",'not-an-integer','not-a-decimal',"+tc.sql+")")
	}
	payload, _ := json.Marshal(core.QueryRequest{Connection: c, SQL: "SELECT value,integer_value,numeric_value,text_value FROM dynamic ORDER BY rowid"})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/query", bytes.NewReader(payload))
	token := strings.Repeat("x", 32)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	httpapi.New(e, token).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("HTTP %d: %s", response.Code, response.Body.String())
	}
	var result core.QueryResult
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Results) != 1 || len(result.Results[0].Rows) != len(cases) {
		t.Fatalf("incomplete rows %#v", result)
	}
	for index, tc := range cases {
		row := result.Results[0].Rows[index]
		if row[0] != tc.cell {
			t.Errorf("runtime row %d: %#v want %#v", index, row[0], tc.cell)
		}
		if row[1] != (core.Cell{Kind: "text", Value: "not-an-integer"}) || row[2] != (core.Cell{Kind: "text", Value: "not-a-decimal"}) {
			t.Errorf("declared affinity coerced TEXT: %#v", row)
		}
		expected := tc.cell
		if expected.Kind == "integer" || expected.Kind == "float" {
			expected.Kind = "text"
		}
		if row[3] != expected {
			t.Errorf("TEXT affinity row %d: %#v want %#v", index, row[3], expected)
		}
	}
}
func TestSQLiteCatalogKeepsLiteralPrefixNeighbors(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	execute(t, e, c, "CREATE TABLE sqlitex_visible(value)")
	execute(t, e, c, "CREATE TABLE seq(value INTEGER PRIMARY KEY AUTOINCREMENT)")
	tables, err := e.Catalog(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, table := range tables {
		if table.Name == "sqlitex_visible" {
			found = true
		}
		if table.Name == "sqlite_sequence" {
			t.Fatal("internal table exposed")
		}
	}
	if !found {
		t.Fatal("literal-prefix neighbor missing")
	}
}
func TestMetadataEndpointPolicyUsesCanonicalAddresses(t *testing.T) {
	for _, raw := range []string{
		"fd00:ec2::254", "fd00:0ec2:0000:0000:0000:0000:0000:0254",
		"100.100.100.200", "::ffff:100.100.100.200", "::ffff:6464:64c8",
		"169.254.169.254", "::ffff:169.254.169.254",
	} {
		if permittedIP(netip.MustParseAddr(raw)) {
			t.Errorf("metadata address allowed: %s", raw)
		}
	}
	for _, raw := range []string{"127.0.0.1", "10.0.0.1", "::1", "fd00:1234::1"} {
		if !permittedIP(netip.MustParseAddr(raw)) {
			t.Errorf("ordinary explicitly allowed address rejected: %s", raw)
		}
	}
}

const authMarker = "SERVER_CONTROLLED_AUTH_MARKER"

func TestMySQLPerConnectionLoggerSuppressesServerInput(t *testing.T) {
	if mode := os.Getenv("DBMANAGER_MYSQL_LOGGER_CHILD"); mode != "" {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer listener.Close()
		done := make(chan struct{})
		go func() {
			defer close(done)
			connection, err := listener.Accept()
			if err != nil {
				return
			}
			listener.Close()
			defer connection.Close()
			connection.SetDeadline(time.Now().Add(time.Second))
			payload := []byte{10}
			payload = append(payload, []byte("8.0.0-fixture\x00")...)
			payload = append(payload, 1, 0, 0, 0)
			payload = append(payload, []byte("12345678\x00")...)
			capabilities := uint32(1 | 1<<9 | 1<<11 | 1<<15 | 1<<19)
			payload = binary.LittleEndian.AppendUint16(payload, uint16(capabilities))
			payload = append(payload, 45, 2, 0)
			payload = binary.LittleEndian.AppendUint16(payload, uint16(capabilities>>16))
			payload = append(payload, 21)
			payload = append(payload, make([]byte, 10)...)
			payload = append(payload, []byte("abcdefghijkl\x00")...)
			payload = append(payload, []byte(authMarker+"\x00")...)
			packet := []byte{byte(len(payload)), byte(len(payload) >> 8), byte(len(payload) >> 16), 0}
			packet = append(packet, payload...)
			connection.Write(packet)
			// Read only the SSLRequest, then close before any TLS session or authentication.
			header := make([]byte, 4)
			if _, err := io.ReadFull(connection, header); err == nil {
				size := int(header[0]) | int(header[1])<<8 | int(header[2])<<16
				if size <= 1024 {
					io.CopyN(io.Discard, connection, int64(size))
				}
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		port := listener.Addr().(*net.TCPAddr).Port
		if mode == "default" {
			config := mysql.NewConfig()
			config.User = "fixture"
			config.Addr = listener.Addr().String()
			config.Net = "tcp"
			config.TLS = &tls.Config{MinVersion: tls.VersionTLS12, ServerName: "localhost"}
			connector, err := mysql.NewConnector(config)
			if err != nil {
				t.Fatal(err)
			}
			database := sql.OpenDB(connector)
			defer database.Close()
			if err := database.PingContext(ctx); err == nil {
				t.Fatal("fake server unexpectedly accepted")
			}
		} else {
			engine := testEngine(t)
			if err := engine.Test(ctx, core.Connection{Driver: "mysql", Host: "127.0.0.1", Port: port, Username: "fixture"}); err == nil {
				t.Fatal("fake server unexpectedly accepted")
			}
		}
		<-done
		return
	}
	for _, mode := range []string{"default", "engine"} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestMySQLPerConnectionLoggerSuppressesServerInput$")
		child.Env = append(os.Environ(), "DBMANAGER_MYSQL_LOGGER_CHILD="+mode)
		output, err := child.CombinedOutput()
		cancel()
		if err != nil {
			t.Fatalf("%s child: %v: %s", mode, err, output)
		}
		contains := bytes.Contains(output, []byte(authMarker))
		if contains != (mode == "default") {
			t.Fatalf("%s marker logging=%s", mode, strconv.FormatBool(contains))
		}
	}
}
