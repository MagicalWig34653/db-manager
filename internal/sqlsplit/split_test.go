// SPDX-License-Identifier: AGPL-3.0-only
package sqlsplit

import (
	"reflect"
	"strings"
	"testing"
)

func TestSplitDialects(t *testing.T) {
	tests := []struct {
		name, sql, driver, delimiter string
		want                         []string
	}{
		{"strings", "SELECT ';', 'it''s;ok'; SELECT 2;", "sqlite", "", []string{"SELECT ';', 'it''s;ok'", "SELECT 2"}},
		{"identifiers", "SELECT [a;b], `c;d`, \"e;f\";", "sqlite", "", []string{"SELECT [a;b], `c;d`, \"e;f\""}},
		{"comments", "-- ;\nSELECT 1 /* ; */; -- tail", "sqlite", "", []string{"-- ;\nSELECT 1 /* ; */"}},
		{"mysql-comment", "SELECT 1--2; # ;\nSELECT 2;", "mysql", "", []string{"SELECT 1--2", "# ;\nSELECT 2"}},
		{"mysql-escape", `SELECT 'a\';b'; SELECT 2;`, "mysql", "", []string{`SELECT 'a\';b'`, "SELECT 2"}},
		{"pg-escape", `SELECT E'a\';b', 'a\'; SELECT 2;`, "postgres", "", []string{`SELECT E'a\';b', 'a\'`, "SELECT 2"}},
		{"dollar", "SELECT $tag$a;b$tag$, $$x;y$$;SELECT 2;", "postgres", "", []string{"SELECT $tag$a;b$tag$, $$x;y$$", "SELECT 2"}},
		{"dollar-identifier", "SELECT a$b$c;SELECT 2;", "postgres", "", []string{"SELECT a$b$c", "SELECT 2"}},
		{"nested", "/* a /* ; */ b */ SELECT 1;", "postgres", "", []string{"/* a /* ; */ b */ SELECT 1"}},
		{"routine", "DELIMITER $$\nCREATE PROCEDURE p() BEGIN SELECT ';'; SELECT 2; END$$\nDELIMITER ;\nCALL p();", "mysql", "", []string{"CREATE PROCEDURE p() BEGIN SELECT ';'; SELECT 2; END", "CALL p()"}},
		{"custom", "SELECT '!!'!!SELECT 2!!", "mysql", "!!", []string{"SELECT '!!'", "SELECT 2"}},
		{"directive-string", "SELECT 'DELIMITER $$\nx;y';", "postgres", "", []string{"SELECT 'DELIMITER $$\nx;y'"}},
		{"executable-comment", "/*! SELECT 1 */;", "mysql", "", []string{"/*! SELECT 1 */"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Split(tt.sql, tt.delimiter, tt.driver)
			if err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %#v %v want %#v", got, err, tt.want)
			}
		})
	}
}
func TestSplitRejectsBeforeExecution(t *testing.T) {
	for _, s := range []string{"", "-- comment", "SELECT '\"", "SELECT $$abc", "/* unclosed", "DELIMITER abc\nSELECT 1", "SELECT 1\nDELIMITER $$", "DELIMITER ; extra", "SELECT '\\", "SELECT \x00", strings.Repeat("SELECT 1;", 101), strings.Repeat("x", MaxBytes+1), string([]byte{255})} {
		if got, err := Split(s, "", "postgres"); err == nil {
			t.Fatalf("accepted %q: %#v", s[:min(80, len(s))], got)
		}
	}
	if _, err := Split("SELECT 1", "'", "sqlite"); err == nil {
		t.Fatal("quote delimiter accepted")
	}
}
func FuzzSplit(f *testing.F) {
	for _, seed := range []string{"SELECT 1;SELECT 2", "SELECT $$a;b$$;", "DELIMITER $$\nCREATE PROCEDURE p() BEGIN SELECT 1; END$$", "/* nested /* x */ */ SELECT 'a''b';", "SELECT E'x\\';y';"} {
		f.Add(seed, uint8(0))
	}
	f.Fuzz(func(t *testing.T, s string, d uint8) {
		drivers := []string{"sqlite", "mysql", "postgres"}
		a, e := Split(s, "", drivers[int(d)%3])
		b, e2 := Split(s, "", drivers[int(d)%3])
		if (e == nil) != (e2 == nil) || !reflect.DeepEqual(a, b) {
			t.Fatal("nondeterministic split")
		}
		if e == nil {
			if len(a) == 0 || len(a) > MaxStatements {
				t.Fatal("invalid count")
			}
			for _, item := range a {
				if strings.TrimSpace(item) == "" || !strings.Contains(s, item) {
					t.Fatal("invalid slice")
				}
			}
		}
	})
}

func TestUnicodeDollarAndKeywordComments(t *testing.T) {
	got, err := Split("SELECT $ä$a;b$ä$; SELECT 2;", "", "postgres")
	if err != nil || len(got) != 2 {
		t.Fatalf("unicode dollar: %#v %v", got, err)
	}
	if Keyword("/* outer /* inner */ SELECT */ BEGIN") != "BEGIN" {
		t.Fatal("nested prefix comment")
	}
	if Keyword("/*! BEGIN */") != "" {
		t.Fatal("executable comment")
	}
	for _, query := range []string{"UPDATE t SET value='RETURNING'", "UPDATE t SET value='x' /* RETURNING */", "SELECT $ä$RETURNING$ä$", "UPDATE t SET \"RETURNING\"=1"} {
		if HasKeyword(query, "RETURNING", "postgres") {
			t.Fatal("quoted keyword", query)
		}
	}
	if !HasKeyword("UPDATE t SET value='x' RETURNING *", "RETURNING", "postgres") {
		t.Fatal("missing returning")
	}
}

func TestKeywordCommentDialect(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql"} {
		if Keyword("/* /* */ BEGIN -- */ SELECT", dialect) != "BEGIN" {
			t.Fatal("transaction hidden by dialect comment", dialect)
		}
	}
	if Keyword("/* /* */ BEGIN -- */ SELECT", "postgres") != "SELECT" {
		t.Fatal("postgres nesting lost")
	}
}

func TestMariaDBExecutableComment(t *testing.T) {
	got, err := Split("/*M! SET autocommit=0 */;SELECT 1;", "", "mysql")
	if err != nil || len(got) != 2 || Keyword(got[0], "mysql") != "" {
		t.Fatalf("MariaDB executable comment silently discarded: %#v %v", got, err)
	}
}
