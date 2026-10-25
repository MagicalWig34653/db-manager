// SPDX-License-Identifier: AGPL-3.0-only
package sqlitesandbox

import "testing"

func TestMaintenanceIntegrityCheckKeepsSandbox(t *testing.T) {
	db, cleanup, err := OpenMaintenance(t.TempDir(), "maintenance")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	defer db.Close()
	if _, err = db.Exec(`CREATE TABLE "odd 'table"(id INTEGER)`); err != nil {
		t.Fatal(err)
	}
	var value string
	if err = db.QueryRow(`PRAGMA main.integrity_check("odd 'table")`).Scan(&value); err != nil || value != "ok" {
		t.Fatalf("check %q %v", value, err)
	}
	for _, statement := range []string{`VACUUM`, `VACUUM INTO 'escaped.sqlite'`, `PRAGMA writable_schema=ON`, `PRAGMA journal_mode=WAL`, `ATTACH 'escaped.sqlite' AS other`} {
		if _, err = db.Exec(statement); err == nil {
			t.Fatalf("sandbox allowed %s", statement)
		}
	}
}
