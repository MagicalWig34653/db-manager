// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"github.com/MagicalWig34653/db-manager/internal/core"
)

func sqlitePrograms(ctx context.Context, env *opEnv, r core.ProgramsRequest) ([]core.Program, bool, error) {
	return readPrograms(ctx, env, r, `SELECT type AS kind,name,'' AS identity_args,
 CASE WHEN type='trigger' THEN 'main' ELSE '' END AS table_schema,
 CASE WHEN type='trigger' THEN tbl_name ELSE '' END AS table_name,
 sql AS definition,NULL AS body,NULL AS definer_name,NULL AS definer_host,NULL AS security_mode,'{}' AS properties
 FROM main.sqlite_schema WHERE ?='main' AND type IN ('view','trigger') AND name NOT GLOB 'sqlite_*'`)
}
