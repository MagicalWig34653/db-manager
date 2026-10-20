// SPDX-License-Identifier: AGPL-3.0-only
package sqlitesandbox

import (
	"bufio"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sqlite "github.com/ncruces/go-sqlite3"
	"github.com/ncruces/go-sqlite3/vfs"
)

func TestLegitimateSQL(t *testing.T) {
	root := t.TempDir()
	db, cleanup, err := Open(root, "probe")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	defer db.Close()
	for _, sql := range []string{
		"CREATE TABLE items(id INTEGER PRIMARY KEY, value TEXT)",
		"BEGIN; INSERT INTO items VALUES(1,'Grüße'); COMMIT",
		"BEGIN; INSERT INTO items VALUES(2,'discard'); ROLLBACK",
		"CREATE INDEX by_value ON items(value)",
		"CREATE VIEW visible AS SELECT * FROM items",
	} {
		if _, err := db.Exec(sql); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	var value string
	if err := db.QueryRow("SELECT value FROM visible").Scan(&value); err != nil || value != "Grüße" {
		t.Fatalf("readback %q %v", value, err)
	}
	var name string
	if err := db.QueryRow("SELECT name FROM sqlite_schema WHERE type='table'").Scan(&name); err != nil || name != "items" {
		t.Fatalf("schema %q %v", name, err)
	}
	rows, err := db.Query("PRAGMA table_info(items)")
	if err != nil {
		t.Fatal(err)
	}
	rows.Close()
	db.Close()
	reopened, closeRoot, err := Open(root, "probe")
	if err != nil {
		t.Fatal(err)
	}
	defer closeRoot()
	defer reopened.Close()
	if err := reopened.QueryRow("SELECT value FROM items").Scan(&value); err != nil || value != "Grüße" {
		t.Fatalf("persisted %q %v", value, err)
	}
}

func TestRejectFileSQL(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	target := filepath.Join(outside, "escaped.sqlite")
	db, cleanup, err := Open(root, "probe")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE items(value TEXT)"); err != nil {
		t.Fatal(err)
	}
	statements := []string{
		"ATTACH DATABASE '" + target + "' AS outside",
		"/* leading comment */ ATTACH DATABASE 'file:" + target + "?mode=rwc' AS outside",
		"VACUUM INTO '" + target + "'",
		"ATTACH DATABASE ('" + outside + "/'||'escaped.sqlite') AS outside",
		"PRAGMA temp_store_directory='" + outside + "'",
		"PRAGMA data_store_directory='" + outside + "'",
		"PRAGMA journal_mode=WAL",
		"PRAGMA writable_schema=ON",
		"PRAGMA temp_store=FILE",
		"SELECT load_extension('" + target + "')",
		"SELECT writefile('" + target + "','escaped')",
		"SELECT readfile('" + target + "')",
		"CREATE VIRTUAL TABLE external USING csv(filename='" + target + "')",
	}
	for _, sql := range statements {
		t.Run(sql, func(t *testing.T) {
			if _, err := db.Exec(sql); err == nil {
				t.Fatal("file SQL unexpectedly allowed")
			}
		})
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("outside file was created: %v", err)
	}
	if _, err := db.Exec("INSERT INTO items VALUES('retained')"); err != nil {
		t.Fatalf("usable after rejected query: %v", err)
	}
}

func TestRejectIDsAndVFSPaths(t *testing.T) {
	root := t.TempDir()
	for _, id := range []string{"", "../outside", "/tmp/outside", "file:outside?mode=rwc", "probe.sqlite", "a/b", "a\\b", strings.Repeat("a", 65), "Grüße", "_leading"} {
		db, cleanup, err := Open(root, id)
		if err == nil {
			db.Close()
			cleanup()
			t.Errorf("accepted ID %q", id)
		}
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	v := &rootVFS{root: r, database: "probe.sqlite"}
	for _, path := range []string{"../outside.sqlite", "/tmp/outside.sqlite", "file:probe.sqlite?mode=rwc", "sub/probe.sqlite", "probe.sqlite-wal", "other.sqlite"} {
		if _, err := v.FullPathname(path); err == nil {
			t.Errorf("FullPathname accepted %q", path)
		}
		if _, _, err := v.Open(path, vfs.OPEN_CREATE|vfs.OPEN_READWRITE|vfs.OPEN_MAIN_DB); err == nil {
			t.Errorf("Open accepted %q", path)
		}
		if _, err := v.Access(path, vfs.ACCESS_EXISTS); err == nil && path != "probe.sqlite-wal" {
			t.Errorf("Access accepted %q", path)
		}
		if err := v.Delete(path, false); err == nil {
			t.Errorf("Delete accepted %q", path)
		}
	}
}

func TestRejectSymlinkDatabaseAndJournal(t *testing.T) {
	for _, name := range []string{"probe.sqlite", "probe.sqlite-journal"} {
		t.Run(name, func(t *testing.T) {
			root, outside := t.TempDir(), filepath.Join(t.TempDir(), "sentinel")
			if err := os.WriteFile(outside, []byte("sentinel"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, filepath.Join(root, name)); err != nil {
				t.Fatal(err)
			}
			r, err := os.OpenRoot(root)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			v := &rootVFS{root: r, database: "probe.sqlite"}
			if _, _, err := v.Open(name, vfs.OPEN_CREATE|vfs.OPEN_READWRITE|vfs.OPEN_MAIN_DB); err == nil {
				t.Fatal("symlink opened")
			}
			if _, err := v.Access(name, vfs.ACCESS_EXISTS); err == nil {
				t.Fatal("symlink access accepted")
			}
			b, err := os.ReadFile(outside)
			if err != nil || string(b) != "sentinel" {
				t.Fatalf("outside changed %q %v", b, err)
			}
		})
	}
}

func TestCancellationAndLimits(t *testing.T) {
	db, cleanup, err := Open(t.TempDir(), "probe")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err = db.ExecContext(ctx, "WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n) SELECT sum(x) FROM n")
	if !errors.Is(ctx.Err(), context.DeadlineExceeded) || !errors.Is(err, sqlite.INTERRUPT) || time.Since(start) > 2*time.Second {
		t.Fatalf("deadline not respected: %v / %v", ctx.Err(), err)
	}
	if _, err := db.Exec("SELECT zeroblob(1048577)"); err == nil {
		t.Fatal("oversized cell accepted")
	}
	if _, err := db.Exec("SELECT 1"); err != nil {
		t.Fatalf("connection after rejected query: %v", err)
	}
}

func TestExclusiveOpen(t *testing.T) {
	root := t.TempDir()
	first, cleanup, err := Open(root, "probe")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	defer first.Close()
	if err := first.Ping(); err != nil {
		t.Fatal(err)
	}
	second, closeRoot, err := Open(root, "probe")
	if err != nil {
		t.Fatal(err)
	}
	defer closeRoot()
	defer second.Close()
	start := time.Now()
	err = second.Ping()
	if !errors.Is(err, sqlite.BUSY) || time.Since(start) > time.Second {
		t.Fatalf("exclusive busy: %v", err)
	}
}

func TestCrashRecovery(t *testing.T) {
	if root := os.Getenv("DBMANAGER_SANDBOX_CRASH_ROOT"); root != "" {
		db, cleanup, err := Open(root, "probe")
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		defer db.Close()
		if _, err := db.Exec("BEGIN IMMEDIATE; UPDATE items SET value=randomblob(4096)"); err != nil {
			t.Fatal(err)
		}
		os.Stdout.WriteString("transaction-ready\n")
		time.Sleep(time.Hour)
	}
	root := t.TempDir()
	db, cleanup, err := Open(root, "probe")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE items(value BLOB); WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<1000) INSERT INTO items SELECT zeroblob(4096) FROM n"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	cleanup()
	cmd := exec.Command(os.Args[0], "-test.run=^TestCrashRecovery$")
	cmd.Env = append(os.Environ(), "DBMANAGER_SANDBOX_CRASH_ROOT="+root)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { cmd.Process.Kill(); cmd.Wait() }()
	ready := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		if scanner.Scan() {
			ready <- scanner.Text()
		} else {
			ready <- ""
		}
	}()
	select {
	case message := <-ready:
		if message != "transaction-ready" {
			t.Fatalf("child not ready: %q", message)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("child timeout")
	}
	if _, err := os.Stat(filepath.Join(root, "probe.sqlite-journal")); err != nil {
		t.Fatalf("journal missing before crash: %v", err)
	}
	cmd.Process.Kill()
	cmd.Wait()
	reopened, closeRoot, err := Open(root, "probe")
	if err != nil {
		t.Fatal(err)
	}
	defer closeRoot()
	defer reopened.Close()
	var count int
	if err := reopened.QueryRow("SELECT count(*) FROM items").Scan(&count); err != nil || count != 1000 {
		t.Fatalf("recovery count %d %v", count, err)
	}
	var integrity string
	if err := reopened.QueryRow("SELECT integrity_check FROM pragma_integrity_check").Scan(&integrity); err == nil {
		t.Fatal("nonallowlisted pragma function unexpectedly allowed")
	}
	if err := reopened.QueryRow("SELECT count(*) FROM items WHERE value=zeroblob(4096)").Scan(&count); err != nil || count != 1000 {
		t.Fatalf("recovery values %d %v", count, err)
	}
}

func TestRejectHardlink(t *testing.T) {
	root, outside := t.TempDir(), filepath.Join(t.TempDir(), "sentinel")
	if err := os.WriteFile(outside, []byte("sentinel"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(outside, filepath.Join(root, "probe.sqlite")); err != nil {
		t.Fatal(err)
	}
	db, cleanup, err := Open(root, "probe")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	defer db.Close()
	if err := db.Ping(); err == nil {
		t.Fatal("hardlink accepted")
	}
	b, err := os.ReadFile(outside)
	if err != nil || string(b) != "sentinel" {
		t.Fatalf("outside changed %q %v", b, err)
	}
}

func TestRejectWALArtifacts(t *testing.T) {
	for _, suffix := range []string{"-wal", "-shm"} {
		t.Run(suffix, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "probe.sqlite"+suffix)
			if err := os.WriteFile(path, []byte("sentinel"), 0600); err != nil {
				t.Fatal(err)
			}
			db, cleanup, err := Open(root, "probe")
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()
			defer db.Close()
			if err := db.Ping(); err == nil {
				t.Fatal("WAL artifact accepted")
			}
			b, err := os.ReadFile(path)
			if err != nil || string(b) != "sentinel" {
				t.Fatalf("artifact changed %q %v", b, err)
			}
		})
	}
}

func TestRejectIndirectFileFunctions(t *testing.T) {
	db, cleanup, err := Open(t.TempDir(), "probe")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	defer db.Close()
	_, err = db.Exec("CREATE VIEW leak AS SELECT readfile('/outside') AS value")
	if err == nil {
		if _, err := db.Exec("SELECT value FROM leak"); err == nil {
			t.Fatal("file function through view accepted")
		}
	}
}

func TestNewFileSyncIncludesDirectory(t *testing.T) {
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	file, err := root.OpenFile("journal", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	wrapped := &rootFile{File: file, root: root, syncDirectory: true}
	if err := wrapped.Sync(vfs.SYNC_FULL); err != nil || wrapped.syncDirectory {
		t.Fatalf("sync %v", err)
	}
	wrapped.syncDirectory = true
	root.Close()
	if err := wrapped.Sync(vfs.SYNC_FULL); err == nil {
		t.Fatal("directory sync was skipped")
	}
}
