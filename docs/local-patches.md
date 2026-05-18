# VOCSAP local patches over upstream c-premus/mcp-gate

This document tracks patches applied locally in `VOCSAP/mcp-gate` that diverge
from upstream. Each entry justifies the divergence, summarizes the diff, and
records the upstream PR status (if any).

Policy (from operator, 2026-05-18): MIX -- prefer minimizing divergence vs
upstream, patch in-fork only when runtime configuration cannot express the
needed behavior, or when the bug blocks interoperability with our deployment
target.

---

## v0.16.3+vocsap.1 -- fosite/Authelia "scp" claim fallback and double-slash fix

**Date:** 2026-05-18
**Upstream baseline:** v0.16.3 (commit upstream c-premus/mcp-gate)
**VOCSAP HEAD before patch:** da8c21a ("ci: drop VOCSAP GHCR workflow")
**Files touched:** `internal/auth/auth.go`, `internal/auth/auth_test.go`
**Upstream PR status:** not submitted (decision: validate in prod first; if
the issue affects others we will upstream later).

### Problem 1 -- "scp" vs "scope" JWT claim

RFC 9068 §2.2.3 mandates the JWT access-token "scope" claim as a
space-separated string. Authelia 4.39 (and its underlying fosite library)
emits scopes under the legacy "scp" key as a JSON array. The upstream
`Claims` struct only unmarshals from `"scope"`, so tokens from
Authelia/fosite arrive with `claims.Scope == nil`. Combined with the default
`REQUIRED_SCOPES=openid` (cmd/mcp-gate/main.go), every Authelia-issued token
is rejected with 403 `insufficient_scope` regardless of the actual scopes
carried under `"scp"`.

No runtime workaround: `getenvDefault` returns the fallback when the env
variable is set to an empty string, so `REQUIRED_SCOPES=""` cannot disable
the check. A space-only or comma-only value works but disables scope
validation entirely, which is unacceptable in production.

### Problem 2 -- WWW-Authenticate `resource_metadata` double slash

`internal/auth/auth.go` builds the `resource_metadata` parameter by
concatenating `m.cfg.ResourceURI + "/.well-known/oauth-protected-resource"`
in three handlers (401 no-token, 401 invalid-token, 403 insufficient-scope).
When `RESOURCE_URI` ends with a trailing slash -- mandatory for claude.ai
MCP pairing per RFC 8707 strict comparison -- the result contains a double
slash (`https://host//.well-known/...`). Cosmetic only (consumers parse it
as a URL hint, not a strict identifier) but visible in operator-facing
diagnostics.

### Patch summary

1. Add `Scp Scopes` field to `Claims` with `json:"scp,omitempty"` tag.
2. After `ParseWithClaims` succeeds in `(*Middleware).ServeHTTP`, fold
   `claims.Scp` into `claims.Scope` only when `claims.Scope` is empty. RFC
   9068 `"scope"` therefore wins when both claims are present.
3. Wrap `m.cfg.ResourceURI` in `strings.TrimSuffix(..., "/")` at the three
   `WWW-Authenticate` build sites.
4. Add tests covering (a) scp-only fallback, (b) scope wins over scp, (c)
   no double slash in any of the three error responses when `ResourceURI`
   ends with `/`.

### Verification

Local Go toolchain unavailable on the operator workstation; tests are run
in the deployment LXC via the standard Docker multi-stage build. The CI
workflow `.github/workflows/ci.yaml` runs `go test -race ./...` on push.
End-to-end validation is the claude.ai MCP pairing test on LXC 125 Gateway
(Phase 5.7).
