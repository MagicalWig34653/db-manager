// SPDX-License-Identifier: AGPL-3.0-only
package core

import "context"

type preflightKey struct{}

// WithPlanPreflight marks a plan call as an internal pre-check of an execute request.
// Such a call must not re-arm (release) the single-use plan digest.
func WithPlanPreflight(ctx context.Context) context.Context {
	return context.WithValue(ctx, preflightKey{}, true)
}

// IsPlanPreflight reports whether ctx came from WithPlanPreflight.
func IsPlanPreflight(ctx context.Context) bool {
	v, _ := ctx.Value(preflightKey{}).(bool)
	return v
}
