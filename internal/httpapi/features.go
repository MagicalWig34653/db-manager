// SPDX-License-Identifier: AGPL-3.0-only
package httpapi

import (
	"context"
	"net/http"
)

// Feature registers the routes of one feature package. New backend packages
// append their own Feature in their own file (init-free, see featureList) so
// that server.go does not change for every package.
type Feature func(mux *http.ServeMux, a *API)

// RouteClass groups routes by request-body size needs.
type RouteClass string

const (
	// ClassStandard is the default class for all JSON routes (1 MiB).
	ClassStandard RouteClass = "standard"
	// ClassUpload is reserved for later import routes. It keeps the standard
	// limit until an import package raises it explicitly.
	ClassUpload RouteClass = "upload"
)

// classBodyLimits holds the request-body limit in bytes per route class.
// Import packages raise ClassUpload here (one reviewed line), never per route.
var classBodyLimits = map[RouteClass]int64{
	ClassStandard: MaxRequestBytes,
	ClassUpload:   MaxRequestBytes,
}

// BodyLimit returns the request-body limit of a route class. Unknown classes
// fall back to the standard limit.
func BodyLimit(class RouteClass) int64 {
	if limit, ok := classBodyLimits[class]; ok && limit > 0 {
		return limit
	}
	return MaxRequestBytes
}

type bodyLimitKey struct{}

// WithRouteClass wraps a handler so that decode applies the limit of class.
func WithRouteClass(class RouteClass, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), bodyLimitKey{}, BodyLimit(class))))
	})
}

// requestBodyLimit is the limit decode applies; routes without a class keep
// the standard limit.
func requestBodyLimit(r *http.Request) int64 {
	if limit, ok := r.Context().Value(bodyLimitKey{}).(int64); ok {
		return limit
	}
	return MaxRequestBytes
}

// featureList is the ordered registration list. The order is the historical
// registration order of server.go; do not reorder existing entries.
var featureList = []Feature{
	registerV1,
	func(mux *http.ServeMux, a *API) { a.registerTables(mux) },
	func(mux *http.ServeMux, a *API) { a.registerMutations(mux) },
	func(mux *http.ServeMux, a *API) { a.registerInserts(mux) },
	func(mux *http.ServeMux, a *API) { a.registerStructure(mux) },
	func(mux *http.ServeMux, a *API) { a.registerCreate(mux) },
	func(mux *http.ServeMux, a *API) { a.registerOverview(mux) },
	func(mux *http.ServeMux, a *API) { a.registerObjectOps(mux) },
	func(mux *http.ServeMux, a *API) { a.registerColumns(mux) },
	func(mux *http.ServeMux, a *API) { a.registerMaintenance(mux) },
	func(mux *http.ServeMux, a *API) { a.registerExport(mux) },
	func(mux *http.ServeMux, a *API) { a.registerSQLBatch(mux) },
}

func (a *API) registerFeatures(mux *http.ServeMux) {
	for _, feature := range featureList {
		feature(mux, a)
	}
}

func registerV1(mux *http.ServeMux, a *API) {
	mux.Handle("GET /api/v1/capabilities", a.auth(http.HandlerFunc(a.capabilities)))
	mux.Handle("POST /api/v1/connections/test", a.auth(http.HandlerFunc(a.test)))
	mux.Handle("POST /api/v1/catalog", a.auth(http.HandlerFunc(a.catalog)))
	mux.Handle("POST /api/v1/query", a.auth(http.HandlerFunc(a.query)))
}
