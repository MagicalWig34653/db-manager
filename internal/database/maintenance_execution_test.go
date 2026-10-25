// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

type maintenanceProbeDriver struct{}
type maintenanceProbeConn struct{ scenario string }
type maintenanceProbeRows struct {
	scenario string
	sent     bool
}

func (maintenanceProbeDriver) Open(name string) (driver.Conn, error) {
	return &maintenanceProbeConn{scenario: name}, nil
}
func (*maintenanceProbeConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("unused") }
func (*maintenanceProbeConn) Close() error                        { return nil }
func (*maintenanceProbeConn) Begin() (driver.Tx, error)           { return nil, errors.New("unused") }
func (c *maintenanceProbeConn) ExecContext(ctx context.Context, _ string, _ []driver.NamedValue) (driver.Result, error) {
	if c.scenario == "cancel" {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if c.scenario == "exec-error" {
		return nil, errors.New("secret SQL /private/path password")
	}
	return driver.RowsAffected(0), nil
}
func (c *maintenanceProbeConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return &maintenanceProbeRows{scenario: c.scenario}, nil
}
func (r *maintenanceProbeRows) Columns() []string {
	if r.scenario == "integrity" || r.scenario == "integrity-error" {
		return []string{"integrity_check"}
	}
	if r.scenario == "checksum" || r.scenario == "checksum-null" {
		return []string{"Table", "Checksum"}
	}
	return []string{"Table", "Op", "Msg_type", "Msg_text"}
}
func (*maintenanceProbeRows) Close() error { return nil }
func (r *maintenanceProbeRows) Next(v []driver.Value) error {
	if r.scenario == "read-error" {
		return errors.New("secret SQL /private/path password")
	}
	if r.sent || r.scenario == "empty" {
		return io.EOF
	}
	r.sent = true
	switch r.scenario {
	case "integrity":
		v[0] = "ok"
	case "integrity-error":
		v[0] = "secret corruption details /private/path"
	case "checksum":
		v[0] = "table"
		v[1] = "18446744073709551615"
	case "checksum-null":
		v[0] = "table"
		v[1] = nil
	default:
		v[0], v[1], v[2], v[3] = "table", "analyze", "status", "OK"
		if r.scenario == "error-status" {
			v[2], v[3] = "error", "secret SQL /private/path password"
		}
		if r.scenario == "note-only" {
			v[2], v[3] = "note", "unsupported"
		}
		if r.scenario == "up-to-date" {
			v[3] = "Table is already up to date"
		}
	}
	return nil
}

var maintenanceProbeOnce sync.Once

func TestMaintenanceExecutionOutcomes(t *testing.T) {
	maintenanceProbeOnce.Do(func() { sql.Register("maintenance-probe", maintenanceProbeDriver{}) })
	for _, tc := range []struct {
		scenario, driver, action string
		want                     dbplan.Status
		reason                   string
	}{
		{"ok", "postgres", "analyze", dbplan.StatusCompleted, ""},
		{"exec-error", "postgres", "vacuum", dbplan.StatusOutcomeUnknown, "execution_unconfirmed"},
		{"cancel", "postgres", "vacuum", dbplan.StatusOutcomeUnknown, "execution_unconfirmed"},
		{"error-status", "mysql", "repair", dbplan.StatusOutcomeUnknown, "execution_unconfirmed"},
		{"read-error", "mysql", "analyze", dbplan.StatusOutcomeUnknown, "execution_unconfirmed"},
		{"note-only", "mysql", "analyze", dbplan.StatusOutcomeUnknown, "execution_unconfirmed"},
		{"empty", "mysql", "analyze", dbplan.StatusOutcomeUnknown, "execution_unconfirmed"},
		{"up-to-date", "mysql", "analyze", dbplan.StatusCompleted, ""},
		{"integrity", "sqlite", "check", dbplan.StatusCompleted, ""},
		{"integrity-error", "sqlite", "check", dbplan.StatusRejected, "integrity_check_failed"},
		{"checksum", "mysql", "checksum", dbplan.StatusCompleted, ""},
		{"checksum-null", "mysql", "checksum", dbplan.StatusRejected, "checksum_unavailable"},
	} {
		t.Run(tc.scenario, func(t *testing.T) {
			db, err := sql.Open("maintenance-probe", tc.scenario)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			outcome, reason, checksum := runMaintenance(ctx, db, tc.driver, tc.action, "server-generated")
			if outcome != tc.want || reason != tc.reason {
				t.Fatalf("%s %s", outcome, reason)
			}
			if tc.scenario == "checksum" && (checksum == nil || *checksum != "18446744073709551615") {
				t.Fatal("checksum lost precision")
			}
		})
	}
}
func TestMaintenanceRejectsDefaultSystemSchema(t *testing.T) {
	p := maintenancePreview(context.Background(), nil, core.Connection{Driver: "mysql", Database: "mysql"}, core.MaintenanceItem{Table: core.TableRef{Name: "user"}, Action: "analyze"})
	if p.Reason != "system_table" || p.Plan != nil {
		t.Fatalf("system schema %#v", p)
	}
}
