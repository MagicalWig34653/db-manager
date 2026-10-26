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
			{"driver": "sqlite", "state": "limited", "views_as_tables": true, "definer": []string{"omit"}, "compatibility": []string{"native", "data-only"}, "data_statements": []string{"insert", "insert-ignore", "replace"}, "reason": "Exklusive lokale Transaktion; Viewmaterialisierung ohne Typaffinität; keine Trigger/Routinen/Events."},
			{"driver": "mysql", "state": "limited", "views_as_tables": true, "definer": []string{"omit"}, "compatibility": []string{"native", "data-only"}, "data_statements": []string{"insert", "insert-ignore", "replace"}, "reason": "Tabellen nur InnoDB; Views nur lesender Best-Effort ohne Garantie über zugrunde liegende Engines. Keine DEFINER-Objektdefinitionen, Trigger/Routinen/Events."},
			{"driver": "postgres", "state": "limited", "views_as_tables": true, "definer": []string{"omit"}, "compatibility": []string{"native", "data-only"}, "data_statements": []string{"insert"}, "reason": "Viewmaterialisierung nur feste Typteilmenge; externe Viewquellen nur Best-Effort. Keine Eigentümer/Sequenzen/Trigger/Routinen/Events."},
		}, "max_bytes": core.ExportMaxBytes, "max_rows": core.ExportMaxRows, "max_timeout_ms": core.ExportMaxTimeoutMS, "max_tables": core.ExportMaxTables, "max_request_bytes": MaxRequestBytes,
	})
}
