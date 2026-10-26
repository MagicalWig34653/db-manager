// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"
	"strings"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func mysqlPrograms(ctx context.Context, env *opEnv, r core.ProgramsRequest) ([]core.Program, bool, error) {
	// A CTE binds the schema once; all names originate in information_schema.
	return readPrograms(ctx, env, r, `WITH target AS (SELECT ? AS schema_name)
 SELECT 'view' AS kind,TABLE_NAME AS name,'' AS identity_args,'' AS table_schema,'' AS table_name,
 NULL AS definition,VIEW_DEFINITION AS body,LEFT(DEFINER,LENGTH(DEFINER)-LENGTH(SUBSTRING_INDEX(DEFINER,'@',-1))-1) AS definer_name,SUBSTRING_INDEX(DEFINER,'@',-1) AS definer_host,
 LOWER(SECURITY_TYPE) AS security_mode,JSON_OBJECT('check_option',CHECK_OPTION,'updatable',IS_UPDATABLE) AS properties
 FROM information_schema.VIEWS,target WHERE BINARY TABLE_SCHEMA=BINARY target.schema_name
 UNION ALL SELECT LOWER(ROUTINE_TYPE),ROUTINE_NAME,'','','',NULL,ROUTINE_DEFINITION,LEFT(DEFINER,LENGTH(DEFINER)-LENGTH(SUBSTRING_INDEX(DEFINER,'@',-1))-1),SUBSTRING_INDEX(DEFINER,'@',-1),LOWER(SECURITY_TYPE),
 JSON_OBJECT('returns',DTD_IDENTIFIER,'deterministic',IS_DETERMINISTIC,'sql_data_access',SQL_DATA_ACCESS,'sql_mode',SQL_MODE,'comment',ROUTINE_COMMENT)
 FROM information_schema.ROUTINES,target WHERE BINARY ROUTINE_SCHEMA=BINARY target.schema_name
 UNION ALL SELECT 'trigger',TRIGGER_NAME,'',EVENT_OBJECT_SCHEMA,EVENT_OBJECT_TABLE,NULL,ACTION_STATEMENT,LEFT(DEFINER,LENGTH(DEFINER)-LENGTH(SUBSTRING_INDEX(DEFINER,'@',-1))-1),SUBSTRING_INDEX(DEFINER,'@',-1),NULL,
 JSON_OBJECT('timing',ACTION_TIMING,'event',EVENT_MANIPULATION,'orientation',ACTION_ORIENTATION,'sql_mode',SQL_MODE)
 FROM information_schema.TRIGGERS,target WHERE BINARY TRIGGER_SCHEMA=BINARY target.schema_name
 UNION ALL SELECT 'event',EVENT_NAME,'','','',NULL,EVENT_DEFINITION,LEFT(DEFINER,LENGTH(DEFINER)-LENGTH(SUBSTRING_INDEX(DEFINER,'@',-1))-1),SUBSTRING_INDEX(DEFINER,'@',-1),NULL,
 JSON_OBJECT('status',STATUS,'event_type',EVENT_TYPE,'execute_at',COALESCE(CAST(EXECUTE_AT AS CHAR),''),'interval_value',COALESCE(INTERVAL_VALUE,''),'interval_unit',COALESCE(INTERVAL_FIELD,''),'starts',COALESCE(CAST(STARTS AS CHAR),''),'ends',COALESCE(CAST(ENDS AS CHAR),''),'last_executed',COALESCE(CAST(LAST_EXECUTED AS CHAR),''),'on_completion',ON_COMPLETION,'time_zone',TIME_ZONE,'sql_mode',SQL_MODE)
 FROM information_schema.EVENTS,target WHERE BINARY EVENT_SCHEMA=BINARY target.schema_name`)
}
func mysqlProgramDetails(ctx context.Context, env *opEnv, p *core.Program) error {
	q, err := opQualified("mysql", p.Object.Schema, p.Object.Name)
	if err != nil {
		return err
	}
	rows, err := env.sess.QueryContext(ctx, "SHOW CREATE "+strings.ToUpper(p.Object.Kind)+" "+q)
	if err != nil {
		return err
	}
	cols, err := rows.Columns()
	if err != nil {
		rows.Close()
		return err
	}
	if rows.Next() {
		values := make([]sql.NullString, len(cols))
		ptrs := make([]any, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err = rows.Scan(ptrs...); err != nil {
			rows.Close()
			return err
		}
		for i, col := range cols {
			if strings.Contains(strings.ToLower(col), "create ") || col == "SQL Original Statement" {
				p.Definition = nullString(values[i])
			}
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if p.Object.Kind != "function" && p.Object.Kind != "procedure" {
		return nil
	}
	rows, err = env.sess.QueryContext(ctx, `SELECT COALESCE(PARAMETER_NAME,''),COALESCE(PARAMETER_MODE,''),DTD_IDENTIFIER FROM information_schema.PARAMETERS WHERE BINARY SPECIFIC_SCHEMA=BINARY ? AND BINARY SPECIFIC_NAME=BINARY ? AND ROUTINE_TYPE=? ORDER BY ORDINAL_POSITION`, p.Object.Schema, p.Object.Name, strings.ToUpper(p.Object.Kind))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var param core.ProgramParameter
		if err = rows.Scan(&param.Name, &param.Mode, &param.Type); err != nil {
			return err
		}
		param.Mode = strings.ToLower(param.Mode)
		if param.Mode != "" {
			p.Parameters = append(p.Parameters, param)
		}
	}
	return rows.Err()
}
