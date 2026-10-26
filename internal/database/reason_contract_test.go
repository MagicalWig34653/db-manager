// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// latestContract loads the highest contracts/api-v0.N.0.json.
func latestContract(t *testing.T) map[string]any {
	t.Helper()
	files, _ := filepath.Glob("../../contracts/api-v0.*.0.json")
	best, bestN := "", -1
	for _, f := range files {
		n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(filepath.Base(f), "api-v0."), ".0.json"))
		if err == nil && n > bestN {
			best, bestN = f, n
		}
	}
	raw, err := os.ReadFile(best)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err = json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

// reasonEnum returns the union of enum values of every "reason" property below a schema.
func reasonEnum(node any, out map[string]bool) {
	switch v := node.(type) {
	case map[string]any:
		for k, child := range v {
			if k == "reason" {
				if m, ok := child.(map[string]any); ok {
					if list, ok := m["enum"].([]any); ok {
						for _, e := range list {
							out[e.(string)] = true
						}
					}
				}
			}
			reasonEnum(child, out)
		}
	case []any:
		for _, child := range v {
			reasonEnum(child, out)
		}
	}
}

// Every reason the code can emit must be in the enum of the newest contract version.
func TestEmittedReasonsAreInLatestContract(t *testing.T) {
	schemas := latestContract(t)["components"].(map[string]any)["schemas"].(map[string]any)
	enum := func(names ...string) map[string]bool {
		out := map[string]bool{}
		for _, n := range names {
			if schemas[n] == nil {
				t.Fatalf("schema %s missing", n)
			}
			reasonEnum(schemas[n], out)
		}
		return out
	}
	literal := regexp.MustCompile(`(?:Reason:\s*|\.Reason\s*=\s*|reject\()"([a-z_0-9]+)"`)
	sources := map[string][]string{
		"accounts.go":       {"AccountResult"},
		"columns.go":        {"ColumnsResult"},
		"indexes.go":        {"IndexesResult"},
		"search_replace.go": {"ReplaceResult"},
		"programs.go":       {"ProgramResult"},
		"maintenance.go":    {"MaintenanceOutcome", "MaintenancePreview"},
		"objectops.go":      {"ObjectOpResult"},
		"partitions.go":     {"StructureResult"},
		"prefix.go":         {"PrefixStructureResult"},
	}
	for file, names := range sources {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		allowed := enum(names...)
		for _, m := range literal.FindAllStringSubmatch(string(raw), -1) {
			if !allowed[m[1]] {
				t.Errorf("%s emits reason %q which is not in the %v enum of the latest contract", file, m[1], names)
			}
		}
	}
	// Classified reasons: opReason/reasonText feed accounts, programs, object ops and structure results.
	var classified []string
	for key := range reasonText {
		classified = append(classified, key)
	}
	sort.Strings(classified)
	for _, names := range [][]string{{"AccountResult"}, {"ProgramResult"}, {"ObjectOpResult"}, {"StructureResult"}, {"PrefixStructureResult"}} {
		allowed := enum(names...)
		for _, key := range classified {
			if !allowed[key] {
				t.Errorf("%v enum lacks classified reason %q", names, key)
			}
		}
	}
	// columnsFailure feeds columns and indexes results.
	for _, key := range []string{"connection_lost", "database_error", "dependency_blocks", "lock_timeout", "permission_denied"} {
		for _, names := range [][]string{{"ColumnsResult"}, {"IndexesResult"}} {
			if !enum(names...)[key] {
				t.Errorf("%v enum lacks columnsFailure reason %q", names, key)
			}
		}
	}
}
