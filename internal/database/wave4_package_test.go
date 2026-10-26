// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

// W-P28 F1: DEL, C1 (incl. NEL) and U+FFFE/U+FFFF are escaped in YAML output and
// round-trip losslessly through a strict JSON/YAML-flow decoder.
func TestYAMLSafeJSONEscapesUnreadableCharacters(t *testing.T) {
	cells := []core.Cell{{Kind: "text", Value: "a\u007fb\u0085c\u0090d\u009fe￾f￿g hä\U0001F600"}}
	raw, _ := json.Marshal(cells)
	out := yamlSafeJSON(raw)
	for _, r := range string(out) {
		if r == 0x7f || r >= 0x80 && r <= 0x9f || r == 0xfffe || r == 0xffff {
			t.Fatalf("unsafe rune %U left in %q", r, out)
		}
	}
	if !utf8.Valid(out) || !strings.Contains(string(out), `\u0085`) || !strings.Contains(string(out), `\u007f`) {
		t.Fatalf("escapes missing: %s", out)
	}
	var back []core.Cell
	if err := json.Unmarshal(out, &back); err != nil || back[0] != cells[0] {
		t.Fatalf("round trip: %v %#v", err, back)
	}
	plain := []byte(`["abc"]`)
	if string(yamlSafeJSON(plain)) != `["abc"]` {
		t.Fatal("plain text changed")
	}
}

// W-P28: XML 1.0 forbidden characters switch to base64; allowed characters stay text.
func TestXMLExportFieldCharacterRange(t *testing.T) {
	for _, v := range []string{"a\x01b", "a\x0bb", "a￾b", "a￿b", "a￾b"} {
		if got := xmlExportField("v", v); !strings.Contains(got, `encoding="base64"`) {
			t.Errorf("%q not base64 encoded: %s", v, got)
		}
	}
	for _, v := range []string{"a\tb", "a\nb", "a\u007fb", "a\u0085b", "a�b", "a\U0001F600b", "a퟿b", "ab"} {
		if got := xmlExportField("v", v); strings.Contains(got, "base64") {
			t.Errorf("%q needlessly base64 encoded", v)
		}
	}
}

// W-P28: excel-csv prefixes every field and replaces control characters.
func TestExcelCSVValueControls(t *testing.T) {
	f := &exportFormat{r: core.ExportRequest{Format: "excel-csv"}}
	for in, want := range map[string]string{"=1+1": "'=1+1", "": "'", "a\x00b": "'a�b", "a\x1bb": "'a�b", "a\u0085b": "'a�b", "a\tb\r\n": "'a\tb\r\n"} {
		if got := f.csvValue(in); got != want {
			t.Errorf("csvValue(%q)=%q want %q", in, got, want)
		}
	}
	if (&exportFormat{r: core.ExportRequest{Format: "csv"}}).csvValue("=1") != "=1" {
		t.Fatal("plain csv must stay verbatim")
	}
}

// W-P17: scope/filter/limit validation runs before any connection is made.
func TestServerAdminRejectsBeforeConnecting(t *testing.T) {
	e := testEngine(t)
	c := core.Connection{Driver: "mysql", Host: "127.0.0.1", Port: 1, Username: "x", Database: "x", TLSMode: "verify-full"}
	bad := []core.ServerAdminReadRequest{
		{Connection: c, Scope: "GLOBAL; DROP TABLE x"},
		{Connection: c, Scope: "global"},
		{Connection: c, Limit: 201},
		{Connection: c, Offset: 10001},
		{Connection: c, Filter: strings.Repeat("x", 129)},
		{Connection: c, TimeoutMS: 10001},
	}
	for _, r := range bad {
		_, err := e.ServerAdminRead(context.Background(), "variables", r)
		requireMutationCode(t, err, "invalid_server_admin")
	}
	_, err := e.ServerAdminRead(context.Background(), "status", core.ServerAdminReadRequest{Connection: c, Scope: "GLOBAL"})
	requireMutationCode(t, err, "invalid_server_admin")
	_, err = e.ServerAdminRead(context.Background(), "kill", core.ServerAdminReadRequest{Connection: c})
	requireMutationCode(t, err, "invalid_server_admin")
}

// W-P17: sensitive-name block and value length cap.
func TestAdminSafeValueSensitiveNamesAndLength(t *testing.T) {
	for _, name := range []string{"ssl_cert", "tls_version", "keyring_x", "datadir", "secure_file_priv", "my_password_x", "api_token", "tmpdir", "Connection_errors_accept", "authentication_secret"} {
		if v := adminSafeValue(name, "1", false); v.Value != nil || !v.Masked {
			t.Errorf("%s leaked", name)
		}
	}
	if v := adminSafeValue("Threads_running", strings.Repeat("9", 64), false); v.Value == nil {
		t.Error("64 digits are within the limit")
	}
	if v := adminSafeValue("Threads_running", strings.Repeat("9", 65), false); v.Value != nil {
		t.Error("65 digits must stay masked")
	}
}

// W-P29: multibyte prefixes match by characters, not bytes; names are validated.
func TestSQLitePrefixMultibyteAndNames(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	h := &opHarness{t: t, e: e, c: c}
	for _, n := range []string{"äö_one", "äö_two", "äx_other", "plain"} {
		h.exec("CREATE TABLE " + h.q(n) + " (id INTEGER)")
	}
	list, err := e.PrefixTables(context.Background(), core.PrefixRequest{Connection: c, Prefix: "äö_"})
	if err != nil || len(list.Tables) != 2 {
		t.Fatalf("%v %v", list, err)
	}
	if !opName("sqlite", "ok") || opName("sqlite", "") || opName("sqlite", " x") || opName("sqlite", "a\x00b") || opName("sqlite", "a\nb") || opName("sqlite", "sqlite_x") || opName("mysql", strings.Repeat("ä", 65)) || !opName("mysql", strings.Repeat("ä", 64)) || opName("postgres", strings.Repeat("a", 64)) {
		t.Fatal("opName rules changed")
	}
	r := core.PrefixRequest{Connection: c, Prefix: "äö_", Action: "replace", NewPrefix: "q_"}
	p, err := e.PlanPrefix(context.Background(), r)
	if err != nil || p.Confirmation.TargetName == "" {
		t.Fatalf("plan: %v typed name %q", err, p.Confirmation.TargetName)
	}
	if _, err = e.ExecutePrefix(context.Background(), core.PrefixExecuteRequest{PrefixRequest: r, Confirm: dbplan.Confirmation{Action: p.Action}, ExpectedPlanSHA256: p.SHA256}); err == nil {
		t.Fatal("destructive plan without typed name must be refused")
	}
	_, err = e.PlanPartitions(context.Background(), core.PartitionRequest{Connection: c, Table: core.TableRef{Name: "plain"}, Action: "add", Definitions: []core.PartitionDefinition{{Name: "a b\n"}}})
	if err == nil {
		t.Fatal("partitions on sqlite must be rejected")
	}
}
