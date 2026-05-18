# VOCSAP local patches over upstream c-premus/mcp-gate

This document tracks patches applied locally in `VOCSAP/mcp-gate` that diverge
from upstream. Each entry justifies the divergence, summarizes the diff, and
records the upstream PR status (if any).

Policy (from operator, 2026-05-18): MIX -- prefer minimizing divergence vs
upstream, patch in-fork only when runtime configuration cannot express the
needed behavior, or when the bug blocks interoperability with our deployment
target.

---

## v0.16.3+vocsap.2 -- Cache-Control no-store on RFC 9728 metadata

**Date:** 2026-05-18
**Upstream baseline:** v0.16.3 + vocsap.1
**VOCSAP HEAD before patch:** e2135dd ("fix(auth): accept fosite-style scp...")
**Files touched:** `internal/metadata/metadata.go`,
`internal/metadata/metadata_test.go`
**Upstream PR status:** not submitted.

The RFC 9728 Protected Resource Metadata handler previously emitted
`Cache-Control: max-age=300, must-revalidate`. Replace with `Cache-Control:
no-store`. Rationale: Anthropic's claude.ai MCP troubleshooting thread
(anthropics/claude-ai-mcp #136) lists no-store as a required header for
this endpoint, on the basis that intermediary caching (Cloudflare, browser,
claude.ai proxy layer) has been observed interfering with the OAuth
discovery flow. The metadata is small and infrequently fetched, so no-store
costs nothing operationally.

Test updated to assert the new header value.

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

---

## Known limitations

### claude.ai web/desktop custom MCP connector pairing -- blocked client-side

**Status (2026-05-18):** unresolved upstream by Anthropic.
**Tracking issues:**
 - <https://github.com/anthropics/claude-ai-mcp/issues/136> (umbrella)
 - <https://github.com/anthropics/claude-ai-mcp/issues/254> (closed -> #136)
 - <https://github.com/anthropics/claude-code/issues/46140> (same bug in
   the claude-code repo)

**Symptom:** OAuth 2.1 + PKCE flow completes end-to-end. DCR (or pre-
registered client) succeeds, `/authorize` returns the auth code, `POST
/token` returns 200 with a valid RS256 JWT (correct `aud`, `iss`, `exp`,
`scope`/`scp`, `typ=at+jwt`). claude.ai never sends a subsequent request
to the MCP endpoint with the Bearer token. The connector lands on the
opaque "Authorization with the MCP server failed" page with an
`ofid_...` reference.

**Scope of impact:** any MCP server reached via the claude.ai web or
desktop custom connector path. Confirmed by multiple independent reports
including:
 - Vercel's first-party `mcp.vercel.com`
 - clauth (Node, Cloudflare tunnel)
 - Django + Streamable HTTP + nginx + Cloudflare
 - VOCSAP `mcp-crawl4ai.coreleap.com.br` (this deployment)

**Server-side workarounds applied but not sufficient:**
 - Absolute `resource_metadata` URL in `WWW-Authenticate` (vocsap.1).
 - `Cache-Control: no-store` on RFC 9728 metadata (vocsap.2).
 - `aud` claim with trailing-slash alignment to claude.ai's
   `resource=...` indicator.
 - PKCE S256 enforced upstream by Authelia 4.39.

None of these, individually or together, has been reported to resolve
the bug. Anthropic support has acknowledged the issue as client-side and
has not announced a fix timeline.

**Operational workaround that does work:** bypass the claude.ai web
connector path and register the MCP server directly with Claude Code
CLI:

```bash
claude mcp add -s user --transport http <name> https://<host>/sse
# In a fresh Claude Code session: /mcp -> <name> -> OAuth completes locally.
```

This routes the OAuth flow through Claude Code's local implementation,
which is not affected by the claude.ai proxy bug. Tools then appear as
`mcp__<name>__*` instead of `mcp__claude_ai_<name>__*`.

**Resolution path:** monitor the upstream issues. Re-test the claude.ai
web connector after each Anthropic platform release. Remove this section
once the upstream bug is fixed and a passing E2E pairing is observed.
