// Package v1 contains browser-facing BFF endpoints.
//
// These handlers are designed for same-origin SPA clients and rely on
// cookie session semantics (plus CSRF protections for state-changing routes).
// Keep browser-only concerns here, and do not mix external-client auth flows.
package v1
