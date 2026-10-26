// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

func partitionValid(r core.PartitionRequest, read bool) error {
	encoded, marshalErr := json.Marshal(r)
	if marshalErr != nil || len(encoded) > 1<<20 {
		return invalidOp("Anfrage ist zu groß.")
	}
	if !validTableRef(r.Table) || r.TimeoutMS < 0 || r.TimeoutMS > 60000 || len(r.Partitions) > 100 || len(r.Definitions) > 100 || r.Count < 0 || r.Count > 100 {
		return invalidOp("Partitionierungsanfrage ist ungültig.")
	}
	action := r.Action
	if read {
		action = "inspect"
		if r.Action != "" || r.Method != "" || r.Column != "" || len(r.Partitions) > 0 || len(r.Definitions) > 0 || r.Count != 0 {
			return invalidOp("Die Leseanfrage enthält Änderungsfelder.")
		}
	}
	if !core.PartitionsCapabilities().Allows(r.Connection.Driver, action) {
		return public("unsupported_action", core.PartitionsCapabilities().Get(r.Connection.Driver, action).Reason)
	}
	return nil
}

func (e *Engine) Partitions(ctx context.Context, r core.PartitionRequest) (core.PartitionsResult, error) {
	if err := partitionValid(r, true); err != nil {
		return core.PartitionsResult{}, err
	}
	ctx, cancel := opContext(ctx, r.TimeoutMS)
	defer cancel()
	env, closeAll, err := e.openOp(ctx, r.Connection)
	if err != nil {
		return core.PartitionsResult{}, err
	}
	defer closeAll()
	d, err := opSourceTable(ctx, env, r.Table)
	if err != nil {
		return core.PartitionsResult{}, opPlanError(ctx, err)
	}
	p, err := readPartitions(ctx, env, d.Table)
	return p, opPlanError(ctx, err)
}

func readPartitions(ctx context.Context, env *opEnv, ref core.TableRef) (core.PartitionsResult, error) {
	p := core.PartitionsResult{Driver: env.c.Driver, Table: ref, Partitions: []core.PartitionInfo{}, Notes: []string{"Zeilenzahlen sind Katalogschätzungen; kein atomarer Schemasnapshot."}}
	var rows *sql.Rows
	var err error
	if env.c.Driver == "mysql" {
		rows, err = env.sess.QueryContext(ctx, `SELECT PARTITION_NAME, PARTITION_METHOD, PARTITION_EXPRESSION, COALESCE(PARTITION_DESCRIPTION,''), TABLE_ROWS, SUBPARTITION_NAME FROM information_schema.PARTITIONS WHERE TABLE_SCHEMA=? AND TABLE_NAME=? AND PARTITION_NAME IS NOT NULL ORDER BY PARTITION_ORDINAL_POSITION,SUBPARTITION_ORDINAL_POSITION LIMIT 101`, ref.Schema, ref.Name)
	} else {
		err = env.sess.QueryRowContext(ctx, `SELECT CASE pt.partstrat WHEN 'r' THEN 'RANGE' WHEN 'l' THEN 'LIST' WHEN 'h' THEN 'HASH' END, pg_get_partkeydef(c.oid) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_partitioned_table pt ON pt.partrelid=c.oid WHERE n.nspname=$1 AND c.relname=$2`, ref.Schema, ref.Name).Scan(&p.Method, &p.Expression)
		if errors.Is(err, sql.ErrNoRows) {
			return p, nil
		}
		if err != nil {
			return p, err
		}
		rows, err = env.sess.QueryContext(ctx, `SELECT n.nspname,c.relname,pg_get_expr(c.relpartbound,c.oid),CASE WHEN c.reltuples<0 THEN NULL ELSE c.reltuples::bigint END FROM pg_inherits i JOIN pg_class c ON c.oid=i.inhrelid JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_class parent ON parent.oid=i.inhparent JOIN pg_namespace pn ON pn.oid=parent.relnamespace WHERE pn.nspname=$1 AND parent.relname=$2 AND c.relispartition ORDER BY n.nspname,c.relname LIMIT 101`, ref.Schema, ref.Name)
	}
	if err != nil {
		return p, err
	}
	defer rows.Close()
	for rows.Next() {
		item := core.PartitionInfo{Table: core.TableRef{Schema: ref.Schema}}
		var estimate sql.NullInt64
		if env.c.Driver == "mysql" {
			var sub sql.NullString
			err = rows.Scan(&item.Table.Name, &p.Method, &p.Expression, &item.Bound, &estimate, &sub)
			if sub.Valid {
				return p, public("unsupported_action", "Subpartitionen werden nicht unterstützt.")
			}
		} else {
			err = rows.Scan(&item.Table.Schema, &item.Table.Name, &item.Bound, &estimate)
		}
		if err != nil {
			return p, err
		}
		if len(item.Bound)+len(p.Expression) > MaxCellBytes {
			return p, public("result_too_large", "Partitionsmetadaten sind zu groß.")
		}
		if estimate.Valid {
			item.EstimatedRows = &estimate.Int64
		}
		p.Partitions = append(p.Partitions, item)
		if len(p.Partitions) > 100 {
			return p, public("result_too_large", "Mehr als 100 Partitionen werden nicht unterstützt.")
		}
	}
	return p, rows.Err()
}

func checkedStructure(p core.StructurePlan, hash string, confirm dbplan.Confirmation) error {
	err := dbplan.Check(p.Plan, hash, confirm, p.Action, p.Confirmation.TargetName, p.Confirmation.Destructive)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, dbplan.ErrPlanMismatch):
		return public("plan_mismatch", "Der Plan hat sich geändert. Vorschau neu laden.")
	case errors.Is(err, dbplan.ErrInvalidDigest):
		return invalidOp("Plan-Hash ist ungültig.")
	default:
		return public("confirmation_mismatch", "Aktion oder getippter Name stimmen nicht überein.")
	}
}
func structurePlan(driver, action, target string, statements, warnings, affected []string, destructive bool) (core.StructurePlan, error) {
	p, err := dbplan.NewPlan(driver, statements, warnings, affected)
	if err != nil {
		return core.StructurePlan{}, invalidOp("Plan enthält keine gültigen Anweisungen.")
	}
	out := core.StructurePlan{Plan: p, Action: action, Confirmation: core.ObjectOpConfirmation{Action: action, Destructive: destructive, TargetName: target}}
	encoded, err := json.Marshal(out)
	if err != nil || len(encoded)+1 > MaxResponseBytes || len(statements) > 200 {
		return core.StructurePlan{}, public("result_too_large", "Der Strukturplan ist zu groß.")
	}
	return out, nil
}

func (e *Engine) PlanPartitions(ctx context.Context, r core.PartitionRequest) (core.StructurePlan, error) {
	if err := partitionValid(r, false); err != nil {
		return core.StructurePlan{}, err
	}
	ctx, cancel := opContext(ctx, r.TimeoutMS)
	defer cancel()
	env, closeAll, err := e.openOp(ctx, r.Connection)
	if err != nil {
		return core.StructurePlan{}, err
	}
	defer closeAll()
	p, err := buildPartitions(ctx, env, r)
	return p, opPlanError(ctx, err)
}
func (e *Engine) ExecutePartitions(ctx context.Context, r core.StructureExecuteRequest) (core.ObjectOpResult, error) {
	if err := partitionValid(r.PartitionRequest, false); err != nil {
		return core.ObjectOpResult{}, err
	}
	ctx, cancel := opContext(ctx, r.TimeoutMS)
	defer cancel()
	env, closeAll, err := e.openOp(ctx, r.Connection)
	if err != nil {
		return core.ObjectOpResult{}, err
	}
	defer closeAll()
	p, err := buildPartitions(ctx, env, r.PartitionRequest)
	if err != nil {
		return core.ObjectOpResult{}, opPlanError(ctx, err)
	}
	if err = checkedStructure(p, r.ExpectedPlanSHA256, r.Confirm); err != nil {
		return core.ObjectOpResult{}, err
	}
	return executeStructure(ctx, env, p), nil
}

// executeStructure is conservative after any sent statement; no automatic retry.
func executeStructure(ctx context.Context, env *opEnv, p core.StructurePlan) core.ObjectOpResult {
	result := core.ObjectOpResult{Action: p.Action, StatementsTotal: len(p.Statements)}
	transactional := env.c.Driver != "mysql"
	if transactional {
		if err := env.sess.begin(ctx); err != nil {
			result.Outcome = "rejected"
			result.Message = "Transaktion konnte nicht gestartet werden."
			return result
		}
		defer env.sess.rollback()
	}
	for i, s := range p.Statements {
		if err := ctx.Err(); err != nil {
			if transactional {
				env.sess.rollback()
			}
			result.Outcome = "outcome_unknown"
			if i == 0 {
				result.Outcome = "rejected"
			}
			result.Message = "Abgebrochen. Zustand prüfen; nicht automatisch wiederholen."
			return result
		}
		if err := runStructureStatement(ctx, env, p.Action, s); err != nil {
			reason, definite := opReason(err)
			result.Outcome = "outcome_unknown"
			result.Message = "Ergebnis unbekannt. Zustand prüfen; nicht automatisch wiederholen."
			if transactional {
				env.sess.rollback()
			}
			if definite && (transactional || i == 0) {
				result.Outcome = "rejected"
				result.Reason = reason
				result.Message = reasonText[reason] + " Es wurde nichts geändert."
				result.StatementsExecuted = 0
			}
			return result
		}
		result.StatementsExecuted++
	}
	if transactional {
		if err := env.sess.commit(ctx); err != nil {
			result.Outcome = "outcome_unknown"
			result.Message = "Commit-Ergebnis unbekannt. Nicht automatisch wiederholen."
			return result
		}
		result.Atomic = true
	}
	result.Outcome = "completed"
	result.Message = "Anweisungen ausgeführt. Keine zusätzliche Zustandsbeobachtung; kein automatischer Retry."
	return result
}

// MySQL maintenance reports failures as result rows as well as protocol errors.
func runStructureStatement(ctx context.Context, env *opEnv, action, statement string) error {
	if env.c.Driver == "mysql" && (action == "partition_analyze" || action == "partition_check" || action == "partition_optimize" || action == "partition_repair") {
		rows, err := env.sess.QueryContext(ctx, statement)
		if err != nil {
			return err
		}
		defer rows.Close()
		failed := false
		for rows.Next() {
			var table, op, kind, message string
			if err := rows.Scan(&table, &op, &kind, &message); err != nil {
				return err
			}
			if strings.EqualFold(kind, "error") {
				failed = true
			}
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if failed {
			return errors.New("partition maintenance reported failure")
		}
		return nil
	}
	_, err := env.sess.exec(ctx, statement)
	return err
}

// partitionValue treats bounds as data. Sentinels are only accepted by range bounds.
func partitionValue(value string, sentinel bool) (string, error) {
	if sentinel && (value == "MAXVALUE" || value == "MINVALUE") {
		return value, nil
	}
	if len(value) > 1024 {
		return "", invalidOp("Partitionsgrenze ist zu lang.")
	}
	if n, err := strconv.ParseInt(value, 10, 64); err == nil {
		return strconv.FormatInt(n, 10), nil
	}
	return opLiteral(value)
}
func partitionDefinitions(driver, method string, defs []core.PartitionDefinition) ([]string, error) {
	if len(defs) == 0 || len(defs) > 100 {
		return nil, invalidOp("Partitionsdefinition fehlt.")
	}
	var out []string
	seen := map[string]bool{}
	for _, d := range defs {
		if !opName(driver, d.Name) || seen[d.Name] {
			return nil, invalidOp("Partitionsname ist ungültig oder doppelt.")
		}
		seen[d.Name] = true
		name, _ := dbplan.QuoteIdentifier(driver, d.Name)
		var bound string
		switch method {
		case "RANGE":
			if len(d.Values) > 0 || d.Modulus != 0 || d.Remainder != 0 {
				return nil, invalidOp("Fremde Grenzfelder.")
			}
			if driver == "mysql" {
				if d.LessThan == nil || d.From != nil || d.To != nil {
					return nil, invalidOp("less_than fehlt.")
				}
				v, err := partitionValue(*d.LessThan, true)
				if err != nil {
					return nil, err
				}
				bound = "VALUES LESS THAN (" + v + ")"
			} else {
				if d.From == nil || d.To == nil || d.LessThan != nil {
					return nil, invalidOp("from/to fehlen.")
				}
				a, err := partitionValue(*d.From, true)
				if err != nil {
					return nil, err
				}
				b, err := partitionValue(*d.To, true)
				if err != nil {
					return nil, err
				}
				bound = "FOR VALUES FROM (" + a + ") TO (" + b + ")"
			}
		case "LIST":
			if len(d.Values) == 0 || len(d.Values) > 100 || d.LessThan != nil || d.From != nil || d.To != nil || d.Modulus != 0 || d.Remainder != 0 {
				return nil, invalidOp("LIST-Grenzen sind ungültig.")
			}
			values := []string{}
			for _, v := range d.Values {
				lit, err := partitionValue(v, false)
				if err != nil {
					return nil, err
				}
				values = append(values, lit)
			}
			bound = "VALUES IN (" + strings.Join(values, ", ") + ")"
			if driver == "postgres" {
				bound = "FOR " + bound
			}
		case "HASH":
			if driver != "postgres" || d.Modulus < 1 || d.Modulus > 100 || d.Remainder < 0 || d.Remainder >= d.Modulus || len(d.Values) > 0 || d.LessThan != nil || d.From != nil || d.To != nil {
				return nil, invalidOp("HASH-Grenzen sind ungültig.")
			}
			bound = "FOR VALUES WITH (MODULUS " + strconv.Itoa(d.Modulus) + ", REMAINDER " + strconv.Itoa(d.Remainder) + ")"
		default:
			return nil, public("unsupported_action", "Diese Partitionsmethode wird für Definitionen nicht unterstützt.")
		}
		if driver == "mysql" {
			out = append(out, "PARTITION "+name+" "+bound)
		} else {
			out = append(out, bound)
		}
	}
	return out, nil
}

func buildPartitions(ctx context.Context, env *opEnv, r core.PartitionRequest) (core.StructurePlan, error) {
	empty := core.StructurePlan{}
	d, err := opSourceTable(ctx, env, r.Table)
	if err != nil {
		return empty, err
	}
	current, err := readPartitions(ctx, env, d.Table)
	if err != nil {
		return empty, err
	}
	driver := env.c.Driver
	source, err := opQualified(driver, d.Table.Schema, d.Table.Name)
	if err != nil {
		return empty, invalidOp("Ungültiger Tabellenname.")
	}
	method := current.Method
	if r.Action == "define" {
		method = r.Method
	} else if r.Method != "" || r.Column != "" {
		return empty, invalidOp("Methode und Spalte gehören nur zu define.")
	}
	if r.Action != "define" && method == "" {
		return empty, invalidOp("Die Tabelle ist nicht deklarativ partitioniert.")
	}
	needsNames := map[string]bool{"drop": true, "truncate": true, "reorganize": true, "analyze": true, "check": true, "optimize": true, "rebuild": true, "repair": true, "detach": true}
	if needsNames[r.Action] != (len(r.Partitions) > 0) {
		return empty, invalidOp("Partitionsauswahl fehlt oder gehört nicht zur Aktion.")
	}
	needsDefs := r.Action == "add" && (driver == "postgres" || method != "HASH" && method != "KEY") || r.Action == "attach" || r.Action == "reorganize" || r.Action == "define" && (method == "RANGE" || method == "LIST")
	if needsDefs != (len(r.Definitions) > 0) {
		return empty, invalidOp("Partitionsdefinitionen fehlen oder gehören nicht zur Aktion.")
	}
	if r.Count != 0 && !(driver == "mysql" && (r.Action == "coalesce" || r.Action == "add" && (method == "HASH" || method == "KEY") || r.Action == "define" && (method == "HASH" || method == "KEY"))) {
		return empty, invalidOp("Anzahl gehört nicht zur Aktion.")
	}
	if r.Action == "add" {
		added := len(r.Definitions)
		if driver == "mysql" && (method == "HASH" || method == "KEY") {
			added = r.Count
		}
		if len(current.Partitions)+added > 100 {
			return empty, public("result_too_large", "Der Plan würde mehr als 100 Partitionen erzeugen.")
		}
	}
	names := []string{}
	selected := []core.PartitionInfo{}
	seen := map[string]bool{}
	affected := []string{"table:" + opLabel(d.Table)}

	for _, want := range r.Partitions {
		var matches []core.PartitionInfo
		for _, part := range current.Partitions {
			if part.Table.Name == want || driver == "postgres" && opLabel(part.Table) == want {
				matches = append(matches, part)
			}
		}
		if len(matches) == 0 {
			return empty, opNotFound()
		}
		if len(matches) != 1 {
			return empty, invalidOp("Partitionsname ist mehrdeutig; qualifizierten Katalognamen verwenden.")
		}
		part := matches[0]
		label := opLabel(part.Table)
		if seen[label] {
			return empty, invalidOp("Partition ist doppelt ausgewählt.")
		}
		seen[label] = true
		if driver == "mysql" {
			seen[part.Table.Name] = true
		}
		selected = append(selected, part)
		q, err := dbplan.QuoteIdentifier(driver, part.Table.Name)
		if err != nil {
			return empty, invalidOp("Partitionsname ist ungültig.")
		}
		names = append(names, q)
		affected = append(affected, "partition:"+label)
	}

	if driver == "mysql" {
		for _, def := range r.Definitions {
			for _, part := range current.Partitions {
				if part.Table.Name == def.Name && !seen[def.Name] && r.Action != "define" {
					return empty, opExists()
				}
			}
			affected = append(affected, "partition:"+def.Name)
		}
	}
	for _, part := range selected {
		if driver == "postgres" {
			sub, err := tableSubObjects(ctx, env, part.Table)
			if err != nil {
				return empty, err
			}
			affected = append(affected, sub.labels...)
		}
	}
	warnings := []string{"Partitionierungs-DDL kann lange sperren. Abhängigkeiten und Serverrechte bleiben wirksam."}
	if driver == "mysql" {
		warnings = append(warnings, "DDL bestätigt implizit; ein Fehler nach Teiländerungen kann nicht automatisch zurückgenommen werden.")
	}
	destructive := true
	statements := []string{}
	alter := "ALTER TABLE " + source + " "
	if driver == "mysql" {
		switch r.Action {
		case "define":
			var column string
			for _, c := range d.Columns {
				if c.Name == r.Column {
					column, _ = dbplan.QuoteIdentifier(driver, c.Name)
				}
			}
			if column == "" {
				return empty, invalidOp("Partitionierungsspalte ist nicht im Katalog.")
			}
			switch method {
			case "RANGE", "LIST", "HASH", "KEY":
			default:
				return empty, invalidOp("Partitionsmethode ist ungültig.")
			}
			clause := "PARTITION BY " + method + " (" + column + ")"
			if method == "RANGE" || method == "LIST" {
				defs, err := partitionDefinitions(driver, method, r.Definitions)
				if err != nil {
					return empty, err
				}
				clause += " (" + strings.Join(defs, ", ") + ")"
			} else {
				if r.Count < 1 {
					return empty, invalidOp("Partitionsanzahl fehlt.")
				}
				clause += " PARTITIONS " + strconv.Itoa(r.Count)
			}
			statements = append(statements, alter+clause)
		case "add", "reorganize":
			if method == "HASH" || method == "KEY" {
				if r.Action != "add" || r.Count < 1 || len(r.Definitions) > 0 {
					return empty, invalidOp("HASH/KEY verlangt eine Anzahl.")
				}
				statements = append(statements, alter+"ADD PARTITION PARTITIONS "+strconv.Itoa(r.Count))
			} else {
				defs, err := partitionDefinitions(driver, method, r.Definitions)
				if err != nil {
					return empty, err
				}
				clause := "ADD PARTITION"
				if r.Action == "reorganize" {
					clause = "REORGANIZE PARTITION " + strings.Join(names, ", ") + " INTO"
					warnings = append(warnings, "REORGANIZE kann sperren und Daten umverteilen.")
				}
				statements = append(statements, alter+clause+" ("+strings.Join(defs, ", ")+")")
			}
		case "coalesce":
			if (method != "HASH" && method != "KEY") || r.Count < 1 || r.Count >= len(current.Partitions) {
				return empty, invalidOp("COALESCE verlangt HASH/KEY und eine kleinere positive Anzahl.")
			}
			statements = append(statements, alter+"COALESCE PARTITION "+strconv.Itoa(r.Count))
		case "remove":
			statements = append(statements, alter+"REMOVE PARTITIONING")
			warnings = append(warnings, "Partitionierung wird entfernt; Daten bleiben in der Tabelle.")
		default:
			if r.Action == "drop" && (method != "RANGE" && method != "LIST") {
				return empty, public("unsupported_action", "DROP PARTITION ist nur für RANGE/LIST verfügbar.")
			}
			if r.Action == "repair" && strings.EqualFold(d.StorageEngine, "InnoDB") {
				return empty, public("unsupported_action", "InnoDB unterstützt REPAIR PARTITION nicht.")
			}
			statements = append(statements, alter+strings.ToUpper(r.Action)+" PARTITION "+strings.Join(names, ", "))
			if r.Action == "optimize" {
				warnings = append(warnings, "InnoDB kann statt einzelner Partitionen die gesamte Tabelle neu aufbauen.")
			}
		}
	} else {
		switch r.Action {
		case "add", "attach":
			defs, err := partitionDefinitions(driver, method, r.Definitions)
			if err != nil {
				return empty, err
			}
			for i, def := range r.Definitions {
				ref := core.TableRef{Schema: d.Table.Schema, Name: def.Name}
				if r.Action == "attach" {
					child, err := opSourceTable(ctx, env, ref)
					if err != nil {
						return empty, err
					}
					ref = child.Table
				} else {
					exists, err := opTableExists(ctx, env.sess, env.c, ref)
					if err != nil {
						return empty, err
					}
					if exists {
						return empty, opExists()
					}
				}
				q, err := opQualified(driver, ref.Schema, ref.Name)
				if err != nil {
					return empty, invalidOp("Ungültiger Partitionsname.")
				}
				statement := "CREATE TABLE " + q + " PARTITION OF " + source + " " + defs[i]
				if r.Action == "attach" {
					statement = alter + "ATTACH PARTITION " + q + " " + defs[i]
				}
				statements = append(statements, statement)
				affected = append(affected, "partition:"+opLabel(ref))
			}
		default:
			for _, p := range selected {
				q, err := opQualified(driver, p.Table.Schema, p.Table.Name)
				if err != nil {
					return empty, invalidOp("Ungültiger Partitionsname.")
				}
				switch r.Action {
				case "detach":
					statements = append(statements, alter+"DETACH PARTITION "+q)
				case "drop":
					statements = append(statements, "DROP TABLE "+q)
				case "truncate":
					statements = append(statements, "TRUNCATE TABLE "+q)
				case "analyze":
					statements = append(statements, "ANALYZE "+q)
				default:
					return empty, public("unsupported_action", "Diese PostgreSQL-Aktion ist nicht unterstützt.")
				}
			}
		}
	}
	if r.Action == "analyze" || r.Action == "check" {
		destructive = false
	}
	if r.Action == "define" || r.Action == "remove" || r.Action == "coalesce" {
		for _, part := range current.Partitions {
			affected = append(affected, "partition:"+opLabel(part.Table))
		}
	}
	if r.Action == "drop" {
		warnings = append(warnings, "DROP PARTITION löscht alle Daten der ausgewählten Partitionen unwiderruflich.")
	}
	if r.Action == "truncate" {
		warnings = append(warnings, "Alle Daten der ausgewählten Partitionen werden unwiderruflich gelöscht.")
	}
	if r.Action == "detach" {
		warnings = append(warnings, "Die abgetrennte Tabelle bleibt mit ihren Daten erhalten; sie gehört nicht mehr zur Partitionierung.")
	}
	return structurePlan(driver, "partition_"+r.Action, d.Table.Name, statements, warnings, affected, destructive)
}
