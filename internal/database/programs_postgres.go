// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"github.com/MagicalWig34653/db-manager/internal/core"
)

func postgresPrograms(ctx context.Context, env *opEnv, r core.ProgramsRequest) ([]core.Program, bool, error) {
	return readPrograms(ctx, env, r, `WITH target AS (SELECT $1::text AS schema_name)
 SELECT 'view'::text AS kind,c.relname::text AS name,''::text AS identity_args,''::text AS table_schema,''::text AS table_name,
 'CREATE VIEW '||quote_ident(n.nspname)||'.'||quote_ident(c.relname)||CASE WHEN array_length(c.reloptions,1)>0 THEN ' WITH ('||array_to_string(c.reloptions,',')||')' ELSE '' END||' AS '||pg_get_viewdef(c.oid,true) AS definition,
 pg_get_viewdef(c.oid,true) AS body,pg_get_userbyid(c.relowner)::text AS definer_name,NULL::text AS definer_host,
 CASE WHEN 'security_invoker=true'=ANY(COALESCE(c.reloptions,ARRAY[]::text[])) THEN 'invoker' ELSE 'definer' END AS security_mode,
 json_build_object('derived','true','options',COALESCE(array_to_string(c.reloptions,','),''))::text AS properties
 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace,target
 WHERE c.relkind='v' AND n.nspname=target.schema_name AND (pg_has_role(c.relowner,'USAGE') OR has_table_privilege(c.oid,'SELECT'))
 UNION ALL SELECT CASE p.prokind WHEN 'p' THEN 'procedure' ELSE 'function' END,p.proname::text,pg_get_function_identity_arguments(p.oid),'','',
 pg_get_functiondef(p.oid),p.prosrc,pg_get_userbyid(p.proowner)::text,NULL,CASE WHEN p.prosecdef THEN 'definer' ELSE 'invoker' END,
 json_build_object('returns',pg_get_function_result(p.oid),'language',l.lanname,'volatility',p.provolatile,'strict',p.proisstrict::text)::text
 FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace JOIN pg_language l ON l.oid=p.prolang,target
 WHERE n.nspname=target.schema_name AND p.prokind IN ('f','p') AND (pg_has_role(p.proowner,'USAGE') OR has_function_privilege(p.oid,'EXECUTE'))
 UNION ALL SELECT 'trigger',t.tgname::text,'',n.nspname::text,c.relname::text,pg_get_triggerdef(t.oid,true),NULL,pg_get_userbyid(c.relowner)::text,NULL,NULL,
 json_build_object('enabled',t.tgenabled,'function_schema',fn.nspname,'function_name',p.proname,'function_identity',pg_get_function_identity_arguments(p.oid))::text
 FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_proc p ON p.oid=t.tgfoid JOIN pg_namespace fn ON fn.oid=p.pronamespace,target
 WHERE n.nspname=target.schema_name AND NOT t.tgisinternal AND (pg_has_role(c.relowner,'USAGE') OR has_table_privilege(c.oid,'TRIGGER'))`)
}
func postgresProgramParameters(ctx context.Context, env *opEnv, p *core.Program) error {
	rows, err := env.sess.QueryContext(ctx, `SELECT COALESCE(p.proargnames[a.i],''),
 CASE COALESCE(p.proargmodes[a.i],'i'::"char") WHEN 'i' THEN 'in' WHEN 'o' THEN 'out' WHEN 'b' THEN 'inout' WHEN 'v' THEN 'variadic' WHEN 't' THEN 'table' END,
 format_type(a.type_oid,NULL)
 FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace,
 LATERAL unnest(COALESCE(p.proallargtypes,p.proargtypes::oid[])) WITH ORDINALITY a(type_oid,i)
 WHERE n.nspname=$1 AND p.proname=$2 AND pg_get_function_identity_arguments(p.oid)=$3
 ORDER BY a.i`, p.Object.Schema, p.Object.Name, p.Object.Identity)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var param core.ProgramParameter
		if err = rows.Scan(&param.Name, &param.Mode, &param.Type); err != nil {
			return err
		}
		p.Parameters = append(p.Parameters, param)
	}
	return rows.Err()
}
