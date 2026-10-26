// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

func programType(driver, value string) (string, error) {
	types := map[string]string{"integer": "INTEGER", "bigint": "BIGINT", "text": "TEXT", "boolean": "BOOLEAN", "date": "DATE", "timestamp": "TIMESTAMP", "numeric": "NUMERIC", "real": "REAL", "double": "DOUBLE PRECISION"}
	if driver == "mysql" {
		types["double"] = "DOUBLE"
		types["binary"] = "BLOB"
		types["varchar"] = "VARCHAR(255)"
		types["numeric"] = "DECIMAL(65,30)"
	} else {
		types["binary"] = "BYTEA"
		types["varchar"] = "VARCHAR(255)"
		types["trigger"] = "TRIGGER"
		types["void"] = "VOID"
	}
	if t, ok := types[value]; ok {
		return t, nil
	}
	return "", invalidProgram()
}
func programParameters(driver, kind string, params []core.ProgramParameter) (string, error) {
	out := []string{}
	seen := map[string]bool{}
	for _, p := range params {
		if !opName(driver, p.Name) || seen[p.Name] || p.Mode != "in" && p.Mode != "out" && p.Mode != "inout" || kind == "function" && driver == "mysql" && p.Mode != "in" {
			return "", invalidProgram()
		}
		seen[p.Name] = true
		q, err := dbplan.QuoteIdentifier(driver, p.Name)
		if err != nil {
			return "", err
		}
		typ, err := programType(driver, p.Type)
		if err != nil || p.Type == "trigger" || p.Type == "void" {
			return "", invalidProgram()
		}
		mode := strings.ToUpper(p.Mode) + " "
		if kind == "function" && driver == "mysql" {
			mode = ""
		}
		out = append(out, mode+q+" "+typ)
	}
	return strings.Join(out, ", "), nil
}
func programTime(value string) (string, error) {
	parsed, err := time.Parse("2006-01-02 15:04:05", value)
	if err != nil || parsed.Format("2006-01-02 15:04:05") != value {
		return "", invalidProgram()
	}
	return opLiteral(value)
}
func programSchedule(s *core.ProgramSchedule) (string, error) {
	if s == nil {
		return "", invalidProgram()
	}
	var out string
	if s.At != "" {
		if s.Every != 0 || s.Unit != "" || s.Starts != "" || s.Ends != "" {
			return "", invalidProgram()
		}
		at, err := programTime(s.At)
		if err != nil {
			return "", err
		}
		out = "AT " + at
	} else {
		if s.Every < 1 || s.Every > 1000000 {
			return "", invalidProgram()
		}
		switch s.Unit {
		case "second", "minute", "hour", "day", "week", "month", "year":
		default:
			return "", invalidProgram()
		}
		out = fmt.Sprintf("EVERY %d %s", s.Every, strings.ToUpper(s.Unit))
		if s.Starts != "" {
			v, err := programTime(s.Starts)
			if err != nil {
				return "", err
			}
			out += " STARTS " + v
		}
		if s.Ends != "" {
			v, err := programTime(s.Ends)
			if err != nil {
				return "", err
			}
			if s.Starts != "" && s.Ends <= s.Starts {
				return "", invalidProgram()
			}
			out += " ENDS " + v
		}
	}
	out += " ON COMPLETION "
	if !s.Preserve {
		out += "NOT "
	}
	return out + "PRESERVE", nil
}
func programInputFields(kind, driver string, in core.ProgramInput) error {
	routine := kind == "function" || kind == "procedure"
	if (!routine && (len(in.Parameters) > 0 || in.Returns != "" || in.Language != "" || in.Deterministic)) || kind != "function" && in.Returns != "" || !routine && kind != "view" && in.Security != "" || kind != "trigger" && (in.Timing != "" || in.Event != "" || in.Function != nil) || kind != "event" && (in.Schedule != nil || in.Status != "") || kind != "view" && in.CheckOption != "" {
		return invalidProgram()
	}
	if in.Security != "" && in.Security != "invoker" && in.Security != "definer" {
		return invalidProgram()
	}
	if in.CheckOption != "" && in.CheckOption != "local" && in.CheckOption != "cascaded" {
		return invalidProgram()
	}
	if driver == "sqlite" && (in.Definer != nil || in.Security != "" || in.CheckOption != "") || driver != "postgres" && in.Language != "" || driver == "postgres" && in.Deterministic {
		return invalidProgram()
	}
	return nil
}
func programCreate(ctx context.Context, env *opEnv, r core.ProgramChangeRequest, existing *core.Program) ([]string, error) {
	in := *r.Input
	ref := r.Object
	driver := env.c.Driver
	if err := programInputFields(ref.Kind, driver, in); err != nil {
		return nil, err
	}
	if ref.Kind != "trigger" && ref.Table != nil || ref.Kind != "function" && ref.Kind != "procedure" && ref.Identity != "" {
		return nil, invalidProgram()
	}
	q, err := opQualified(driver, ref.Schema, ref.Name)
	if err != nil {
		return nil, err
	}
	prefix := "CREATE "
	// Views and PostgreSQL routines have a native replace operation. Other
	// definitions are replaced by an explicit DROP/CREATE pair in the preview.
	nativeReplace := r.Action == "replace" && (ref.Kind == "view" && driver != "sqlite" || driver == "postgres" && (ref.Kind == "function" || ref.Kind == "procedure"))
	if nativeReplace {
		prefix += "OR REPLACE "
	}
	if in.Definer != nil && driver == "mysql" {
		if !opName(driver, in.Definer.Name) || !opName(driver, in.Definer.Host) {
			return nil, invalidProgram()
		}
		var catalogUser, catalogHost string
		current := ""
		if err = env.sess.QueryRowContext(ctx, "SELECT CURRENT_USER()").Scan(&current); err != nil {
			return nil, err
		}
		if current == in.Definer.Name+"@"+in.Definer.Host {
			catalogUser, catalogHost = in.Definer.Name, in.Definer.Host
		} else {
			if err = env.sess.QueryRowContext(ctx, "SELECT User,Host FROM mysql.user WHERE BINARY User=BINARY ? AND BINARY Host=BINARY ?", in.Definer.Name, in.Definer.Host).Scan(&catalogUser, &catalogHost); err != nil {
				return nil, opNotFoundOr(err)
			}
		}
		user, _ := dbplan.QuoteIdentifier(driver, catalogUser)
		host, _ := dbplan.QuoteIdentifier(driver, catalogHost)
		prefix += "DEFINER = " + user + "@" + host + " "
	}
	body := strings.TrimSpace(in.Body)
	var statement string
	switch ref.Kind {
	case "view":
		if body == "" {
			return nil, invalidProgram()
		}
		if driver == "mysql" && in.Security != "" {
			prefix += "SQL SECURITY " + strings.ToUpper(in.Security) + " "
		}
		statement = prefix + "VIEW " + q
		if driver == "postgres" && in.Security != "" {
			flag := "false"
			if in.Security == "invoker" {
				flag = "true"
			}
			statement += " WITH (security_invoker=" + flag + ")"
		}
		statement += " AS " + body
		if in.CheckOption != "" {
			statement += " WITH " + strings.ToUpper(in.CheckOption) + " CHECK OPTION"
		}
	case "function", "procedure":
		if body == "" {
			return nil, invalidProgram()
		}
		params, err := programParameters(driver, ref.Kind, in.Parameters)
		if err != nil {
			return nil, err
		}
		statement = prefix + strings.ToUpper(ref.Kind) + " " + q + "(" + params + ")"
		if ref.Kind == "function" {
			returns, err := programType(driver, in.Returns)
			if err != nil {
				return nil, err
			}
			statement += " RETURNS " + returns
		}
		if driver == "postgres" {
			if in.Language != "sql" && in.Language != "plpgsql" {
				return nil, invalidProgram()
			}
			statement += " LANGUAGE " + in.Language
			if in.Security != "" {
				statement += " SECURITY " + strings.ToUpper(in.Security)
			}
			literal, err := opLiteral(body)
			if err != nil {
				return nil, err
			}
			statement += " AS " + literal
			if existing != nil {
				// Match input identity types, not OUT parameters, to prevent replacing a
				// different overload when the client changes the parameter structure.
				// Require the exact existing parameter names/modes/types after resolving
				// the structured type choices through PostgreSQL's catalog formatter.
				for _, param := range existing.Parameters {
					if param.Mode != "in" && param.Mode != "out" && param.Mode != "inout" {
						return nil, public("unsupported_action", "Diese Parametersignatur kann strukturiert nicht ersetzt werden.")
					}
				}
				if len(existing.Parameters) != len(in.Parameters) {
					return nil, invalidProgram()
				}
				for i, param := range in.Parameters {
					typ, _ := programType(driver, param.Type)
					var canonical string
					if err = env.sess.QueryRowContext(ctx, "SELECT format_type(to_regtype($1),NULL)", typ).Scan(&canonical); err != nil {
						return nil, err
					}
					old := existing.Parameters[i]
					if old.Type != canonical || old.Name != param.Name || old.Mode != param.Mode {
						return nil, invalidProgram()
					}
				}
			}
		} else {
			if in.Deterministic {
				statement += " DETERMINISTIC"
			} else {
				statement += " NOT DETERMINISTIC"
			}
			if in.Security != "" {
				statement += " SQL SECURITY " + strings.ToUpper(in.Security)
			}
			statement += " " + body
		}
	case "trigger":
		if ref.Table == nil {
			return nil, invalidProgram()
		}
		if ref.Table.Schema != "" && ref.Table.Schema != ref.Schema {
			return nil, invalidProgram()
		}
		tableRef := *ref.Table
		tableRef.Schema = ref.Schema
		desc, err := describe(ctx, env.sess, env.c, tableRef)
		if err != nil {
			return nil, err
		}
		if desc.Table.Name != ref.Table.Name || desc.Kind != "table" && !(desc.Kind == "view" && in.Timing == "instead_of" && driver != "mysql") {
			return nil, invalidProgram()
		}
		ref.Table = &desc.Table
		table, err := opQualified(driver, desc.Table.Schema, desc.Table.Name)
		if err != nil {
			return nil, err
		}
		timing := strings.ToUpper(in.Timing)
		if in.Timing == "instead_of" {
			timing = "INSTEAD OF"
		}
		if timing != "BEFORE" && timing != "AFTER" && timing != "INSTEAD OF" || in.Event != "insert" && in.Event != "update" && in.Event != "delete" || driver == "mysql" && timing == "INSTEAD OF" {
			return nil, invalidProgram()
		}
		triggerName := q
		if driver == "postgres" {
			triggerName, _ = dbplan.QuoteIdentifier(driver, ref.Name)
		}
		statement = prefix + "TRIGGER " + triggerName + " " + timing + " " + strings.ToUpper(in.Event) + " ON " + table + " FOR EACH ROW "
		if driver == "postgres" {
			if body != "" || in.Function == nil || in.Function.Kind != "function" || !validateProgramRef(*in.Function) {
				return nil, invalidProgram()
			}
			functionRef := *in.Function
			functionRef.Schema, err = programSchema(ctx, env, functionRef.Schema)
			if err != nil {
				return nil, err
			}
			fn, err := programFind(ctx, env, functionRef)
			if err != nil {
				return nil, err
			}
			if fn.Properties["returns"] != "trigger" || fn.Object.Identity != "" {
				return nil, invalidProgram()
			}
			fq, err := opQualified(driver, fn.Object.Schema, fn.Object.Name)
			if err != nil {
				return nil, err
			}
			statement += "EXECUTE FUNCTION " + fq + "()"
		} else {
			if in.Function != nil || body == "" {
				return nil, invalidProgram()
			}
			statement += body
		}
	case "event":
		if body == "" || in.Status != "enabled" && in.Status != "disabled" {
			return nil, invalidProgram()
		}
		schedule, err := programSchedule(in.Schedule)
		if err != nil {
			return nil, err
		}
		status := "DISABLE"
		if in.Status == "enabled" {
			status = "ENABLE"
		}
		statement = prefix + "EVENT " + q + " ON SCHEDULE " + schedule + " " + status + " DO " + body
	}
	statements := []string{}
	if existing != nil && !nativeReplace {
		drop, err := programDrop(env, existing.Object)
		if err != nil {
			return nil, err
		}
		statements = append(statements, drop)
	}
	statements = append(statements, statement)
	if driver == "postgres" && in.Definer != nil {
		if in.Definer.Host != "" || ref.Kind == "trigger" {
			return nil, invalidProgram()
		}
		var role string
		if err = env.sess.QueryRowContext(ctx, "SELECT rolname FROM pg_roles WHERE rolname=$1", in.Definer.Name).Scan(&role); err != nil {
			return nil, opNotFoundOr(err)
		}
		owner, _ := dbplan.QuoteIdentifier(driver, role)
		target := q
		if ref.Kind == "function" || ref.Kind == "procedure" {
			types := []string{}
			for _, param := range in.Parameters {
				if param.Mode != "out" {
					typ, _ := programType(driver, param.Type)
					types = append(types, typ)
				}
			}
			target += "(" + strings.Join(types, ", ") + ")"
		}
		statements = append(statements, "ALTER "+strings.ToUpper(ref.Kind)+" "+target+" OWNER TO "+owner)
	}
	return statements, nil
}
