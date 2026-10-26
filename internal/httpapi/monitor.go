// SPDX-License-Identifier: AGPL-3.0-only
package httpapi

import (
	"errors"
	"github.com/MagicalWig34653/db-manager/internal/core"
	"net/http"
)

func (a *API) registerMonitor(mux *http.ServeMux) {
	mux.Handle("GET /api/v20/capabilities", a.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, available := a.engine.(core.MonitorEngine)
		out := core.MonitorCapabilities{ContractVersion: core.MonitorContractVersion, Available: available, ReadOnly: true, MaxLimit: 200, MaxOffset: 10000, MaxTimeoutMS: 30000, MaxResponseBytes: 8388608, Drivers: []core.MonitorCapability{}}
		for _, driver := range []string{"sqlite", "mysql", "postgres"} {
			for _, action := range core.MonitorActions {
				support, reason := core.MonitorSupport(driver, action)
				out.Drivers = append(out.Drivers, core.MonitorCapability{Driver: driver, Action: action, Support: support, Reason: reason})
			}
		}
		writeJSON(w, 200, out)
	})))
	for _, action := range core.MonitorActions {
		mux.Handle("POST /api/v20/monitor/"+action, a.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			input := core.MonitorRequest{Connection: core.Connection{TLSMode: "verify-full"}}
			if !decode(w, r, &input) || !validConnection(w, input.Connection) {
				return
			}
			engine, ok := a.engine.(core.MonitorEngine)
			if !ok {
				fail(w, 422, "unsupported_feature", "Monitor ist nicht verfügbar.")
				return
			}
			out, err := engine.Monitor(r.Context(), action, input)
			if err != nil {
				var p *core.Error
				if errors.As(err, &p) && p.Code == "invalid_monitor" {
					fail(w, 400, p.Code, p.Message)
				} else {
					engineError(w, err)
				}
				return
			}
			writeJSON(w, 200, out)
		})))
	}
}
