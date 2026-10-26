// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

// readPrograms applies bound filters to a catalog-only UNION. No client text is
// interpolated into SQL. The UNION's columns are the same on all dialects.
func readPrograms(ctx context.Context, env *opEnv, r core.ProgramsRequest, union string) ([]core.Program, bool, error) {
	args := []any{r.Schema}
	mark := func(value any) string {
		args = append(args, value)
		if env.c.Driver == "postgres" {
			return "$" + strconv.Itoa(len(args))
		}
		return "?"
	}
	where := ""
	if r.Kind != "" {
		where += " AND kind=" + mark(r.Kind)
	}
	if r.Object != nil {
		where += " AND name=" + mark(r.Object.Name)
		if env.c.Driver == "postgres" && (r.Kind == "function" || r.Kind == "procedure") {
			where += " AND identity_args=" + mark(r.Object.Identity)
		}
		if r.Object.Table != nil {
			where += " AND table_name=" + mark(r.Object.Table.Name)
			if r.Object.Table.Schema != "" {
				where += " AND table_schema=" + mark(r.Object.Table.Schema)
			}
		}
	} else if r.Table != nil {
		where += " AND table_name=" + mark(r.Table.Name)
		if r.Table.Schema != "" {
			where += " AND table_schema=" + mark(r.Table.Schema)
		}
	}
	limit := r.Limit + 1
	query := "SELECT kind,name,identity_args,table_schema,table_name,definition,body,definer_name,definer_host,security_mode,properties FROM (" + union + ") programs WHERE 1=1" + where + " ORDER BY kind,name,identity_args,table_schema,table_name LIMIT " + mark(limit) + " OFFSET " + mark(r.Offset)
	rows, err := env.sess.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, false, err
	}
	objects := []core.Program{}
	for rows.Next() {
		p := newProgram(core.ProgramRef{Schema: r.Schema})
		var ts, tn, props string
		var definer, host, security sql.NullString
		if err = rows.Scan(&p.Object.Kind, &p.Object.Name, &p.Object.Identity, &ts, &tn, &p.Definition, &p.Body, &definer, &host, &security, &props); err != nil {
			rows.Close()
			return nil, false, err
		}
		if p.Definition != nil && len(*p.Definition) > 1048576 || p.Body != nil && len(*p.Body) > 1048576 {
			rows.Close()
			return nil, false, public("result_too_large", "Definition überschreitet die erlaubte Größe.")
		}
		if tn != "" {
			p.Object.Table = &core.TableRef{Schema: ts, Name: tn}
		}
		if definer.Valid {
			p.Definer = &core.ProgramDefiner{Name: definer.String, Host: host.String}
		}
		p.Security = nullString(security)
		if err = json.Unmarshal([]byte(props), &p.Properties); err != nil {
			rows.Close()
			return nil, false, err
		}
		if env.c.Driver == "postgres" && p.Object.Kind == "view" {
			p.Warnings = append(p.Warnings, "Die CREATE-VIEW-Definition ist aus dem PostgreSQL-Katalog abgeleitet; Eigentümer und Rechte sind getrennte Metadaten.")
		}
		if p.Definition == nil {
			p.Warnings = append(p.Warnings, "Vollständige Definition ist nicht verfügbar; fehlende Sichtbarkeit oder Kataloggrenze.")
		}
		objects = append(objects, p)
		if err = programBound(objects); err != nil {
			rows.Close()
			return nil, false, err
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, false, err
	}
	more := len(objects) > r.Limit
	if more {
		objects = objects[:r.Limit]
	}
	if r.Object != nil {
		for _, p := range objects {
			if p.Object.Name != r.Object.Name || p.Object.Kind != r.Object.Kind || p.Object.Schema != r.Object.Schema {
				return nil, false, opNotFound()
			}
		}
		if len(objects) > 1 || more {
			return nil, false, public("ambiguous_object", "Der Objektbezug ist mehrdeutig; Tabelle beziehungsweise Identitätsargumente sind erforderlich.")
		}
		if len(objects) == 1 {
			p := &objects[0]
			if env.c.Driver == "mysql" {
				if err = mysqlProgramDetails(ctx, env, p); err != nil {
					return nil, false, err
				}
			}
			if env.c.Driver == "postgres" && (p.Object.Kind == "function" || p.Object.Kind == "procedure") {
				if err = postgresProgramParameters(ctx, env, p); err != nil {
					return nil, false, err
				}
			}
			if p.Definition != nil && len(*p.Definition) > 1048576 {
				return nil, false, public("result_too_large", "Definition überschreitet die erlaubte Größe.")
			}
			if err = programDependencies(ctx, env, p); err != nil {
				return nil, false, err
			}
		}
	}
	return objects, more, nil
}
