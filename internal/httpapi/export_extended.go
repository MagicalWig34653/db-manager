// SPDX-License-Identifier: AGPL-3.0-only
package httpapi

import (
	"github.com/MagicalWig34653/db-manager/internal/core"
	"net/http"
)

func (a *API) registerExtendedExport(mux *http.ServeMux) {
	mux.Handle("GET /api/v21/capabilities", a.auth(http.HandlerFunc(a.extendedExportCapabilities)))
	mux.Handle("POST /api/v21/export", a.auth(WithRouteClass(ClassStandard, http.HandlerFunc(a.exportDownload))))
}
func (a *API) extendedExportCapabilities(w http.ResponseWriter, r *http.Request) {
	_, available := a.engine.(core.ExportEngine)
	writeJSON(w, 200, map[string]any{
		"contract_version": core.ExtendedExportContractVersion, "available": available, "formats": core.ExportFormats(),
		"drivers": []map[string]any{
			{"driver": "sqlite", "state": "limited", "views_as_tables": true, "definer": []string{"omit"}, "objects": []string{"views", "triggers"}, "if_not_exists_objects": []string{"views", "triggers"}, "compatibility": []string{"native", "data-only"}, "data_statements": []string{"insert", "insert-ignore", "replace"}, "reason": "Exklusive lokale Transaktion; Viewmaterialisierung ohne Typaffinität. Objekte: Sichten und Trigger aus dem Katalogtext, keine Routinen oder Events."},
			{"driver": "mysql", "state": "limited", "views_as_tables": true, "definer": []string{"omit", "preserve"}, "objects": []string{"views", "routines", "triggers", "events"}, "if_not_exists_objects": []string{"routines", "triggers", "events"}, "compatibility": []string{"native", "data-only"}, "data_statements": []string{"insert", "insert-ignore", "replace"}, "reason": "Tabellen nur InnoDB; Views nur lesender Best-Effort ohne Garantie über zugrunde liegende Engines. Objekte aus SHOW CREATE ohne Snapshotzusage; DEFINER standardmäßig weggelassen, Wiederherstellung von Sichten nur in gleichnamiger Datenbank."},
			{"driver": "postgres", "state": "limited", "views_as_tables": true, "definer": []string{"omit"}, "objects": []string{"views", "routines", "triggers"}, "if_not_exists_objects": []string{"routines"}, "compatibility": []string{"native", "data-only"}, "data_statements": []string{"insert"}, "reason": "Viewmaterialisierung nur feste Typteilmenge; externe Viewquellen nur Best-Effort. Objekte (Sichten, Funktionen, Prozeduren, Trigger) aus dem Katalog im Snapshot; keine Eigentümer, Rechte, Sequenzen, Typen, Regeln, Aggregate oder Events."},
		}, "max_bytes": core.ExportMaxBytes, "max_rows": core.ExportMaxRows, "max_timeout_ms": core.ExportMaxTimeoutMS, "max_tables": core.ExportMaxTables, "objects_contract_version": core.ExportObjectsContractVersion, "max_objects": core.ExportMaxObjects, "max_object_bytes": core.ExportMaxObjectBytes, "max_request_bytes": MaxRequestBytes,
	})
}
