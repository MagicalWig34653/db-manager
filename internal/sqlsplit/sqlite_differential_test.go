// SPDX-License-Identifier: AGPL-3.0-only
// Reviewer addition: differential test of Split against the SQLite tokenizer. It fails only when Split accepts an
// input but yields a different statement count; stricter rejections are safe and only counted.
package sqlsplit

import (
	"math/rand"
	"strings"
	"testing"

	sqlite "github.com/ncruces/go-sqlite3"
	_ "github.com/ncruces/go-sqlite3/embed"
)

// native splits like SQLite's own tokenizer; ok=false when any piece fails to compile.
func native(c *sqlite.Conn, in string) (n int, ok bool) {
	for strings.TrimSpace(in) != "" {
		st, tail, err := c.Prepare(in)
		if err != nil {
			return n, false
		}
		if st != nil {
			n++
			st.Close()
		}
		in = tail
	}
	return n, true
}

func TestSplitMatchesSQLiteTokenizer(t *testing.T) {
	c, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	frags := []string{"SELECT 1", "SELECT 'a;b'", "SELECT 'it''s;'", "SELECT 1 AS \"x;y\"", "SELECT 1 /* ; */", "SELECT /* ' */ 2", "SELECT 1 -- ;\n", "SELECT 1 AS [a;b]", "SELECT 1 AS `q;`", "SELECT 'a\\'", "SELECT '--'", "SELECT '/*'", "SELECT 1 /* -- */", "SELECT x'41'", "SELECT 1 AS \"a\"\"b;\"", "SELECT '\n;\n'", "SELECT 1 /* /* ; */", "SELECT 'é;😀'", "SELECT 1 AS \"--\""}
	seps := []string{";", ";\n", " ; ", ";;", "\n;"}
	toks := []string{"SELECT", " ", "1", "'", "\"", "[", "]", "`", ";", "--", "/*", "*/", "\n", "\\", "x", ",", "'a'", "\"b\"", ";\n"}
	r := rand.New(rand.NewSource(7))
	cmp, bad, rej := 0, 0, 0
	for i := 0; i < 400000; i++ {
		var b strings.Builder
		for k := 0; k < 3+r.Intn(10); k++ {
			b.WriteString(toks[r.Intn(len(toks))])
		}
		in := b.String()
		want, ok := native(c, in)
		if !ok || want == 0 {
			continue
		}
		cmp++
		got, err := Split(in, "", "sqlite")
		if err != nil {
			rej++
			continue
		}
		if len(got) != want {
			bad++
			if bad <= 8 {
				t.Logf("MISMATCH %q native=%d split=%v err=%v", in, want, got, err)
			}
		}
	}
	for i := 0; i < 100000; i++ {
		var b strings.Builder
		for k := 0; k < 1+r.Intn(5); k++ {
			b.WriteString(frags[r.Intn(len(frags))])
			b.WriteString(seps[r.Intn(len(seps))])
		}
		in := b.String()
		want, ok := native(c, in)
		if !ok || want == 0 {
			continue
		}
		cmp++
		got, err := Split(in, "", "sqlite")
		if err != nil {
			rej++
			continue
		}
		if len(got) != want {
			bad++
			if bad <= 8 {
				t.Logf("MISMATCH %q native=%d split=%q", in, want, got)
			}
		}
	}
	t.Logf("comparable=%d rejected(safe)=%d accepted-but-different=%d", cmp, rej, bad)
	if bad != 0 {
		t.Fatalf("%d inputs split differently from SQLite", bad)
	}
	if bad != 0 {
		t.Fatalf("%d inputs split differently from SQLite", bad)
	}
}
