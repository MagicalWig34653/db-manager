// SPDX-License-Identifier: AGPL-3.0-only
package dbplan

import (
	"errors"
	"strings"
)

// QuoteIdentifier quotes an identifier that comes from validated catalog
// metadata, never from free client text. mysql (also MariaDB) uses backticks;
// every other driver (postgres, sqlite) uses double quotes. The rules are
// byte-identical to internal/database.quoteIdentifier, which stays the single
// source for the existing mutation paths; TestQuoteIdentifier pins the semantics
// with the same cases. Callers must pass names that were validated against the catalog.
func QuoteIdentifier(driver, value string) (string, error) {
	if value == "" {
		return "", errors.New("dbplan: empty identifier")
	}
	if strings.ContainsRune(value, 0) {
		return "", errors.New("dbplan: identifier contains NUL")
	}
	if driver == "mysql" {
		return "`" + strings.ReplaceAll(value, "`", "``") + "`", nil
	}
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`, nil
}

// QualifiedName quotes schema.name, or only name when schema is empty.
func QualifiedName(driver, schema, name string) (string, error) {
	quotedName, err := QuoteIdentifier(driver, name)
	if err != nil {
		return "", err
	}
	if schema == "" {
		return quotedName, nil
	}
	quotedSchema, err := QuoteIdentifier(driver, schema)
	if err != nil {
		return "", err
	}
	return quotedSchema + "." + quotedName, nil
}
