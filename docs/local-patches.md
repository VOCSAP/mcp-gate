# VOCSAP local patches over upstream c-premus/mcp-gate

Patches carried in `VOCSAP/mcp-gate` that diverge from upstream. Policy: keep
the divergence minimal and patch only when runtime configuration cannot
express the needed behavior.

Current baseline: upstream v0.21.16, tag `v0.21.16+vocsap.1`.

## Authelia/fosite "scp" claim fallback

**Files:** `internal/auth/auth.go`, `internal/auth/auth_test.go`

RFC 9068 section 2.2.3 mandates a space-separated `scope` claim. Authelia
(fosite) emits the scopes under the legacy `scp` key as a JSON array, so
`claims.Scope` stays empty and every token is rejected with 403
`insufficient_scope` against the default `REQUIRED_SCOPES=openid`. There is no
runtime workaround: `REQUIRED_SCOPES` must now name at least one scope.

`Claims` gains `Scp Scopes` (`json:"scp"`). When `Scope` is empty after
parsing, `Scp` is used in its place, immediately before scope validation.
`scope` wins when both are present. Audience, issuer, `typ`, `cnf` and scope
hierarchy checks are untouched.

## Cache-Control no-store on RFC 9728 resource metadata

**Files:** `internal/metadata/metadata.go`, `internal/metadata/metadata_test.go`

Upstream serves `Cache-Control: max-age=300, must-revalidate`. The handler
sends `no-store` instead: intermediary caching (CDN, browser, claude.ai proxy)
has been implicated in OAuth discovery breakage
(anthropics/claude-ai-mcp#136).

## Dropped from v0.16.3+vocsap.1

The `TrimSuffix` fix on the `resource_metadata` challenge parameter is no
longer needed: the challenge uses `metadata.URLFor(cfg.ResourceURI)`, which
strips the trailing slash through `PathSuffix`.

## Known limitation

claude.ai web/desktop custom connector pairing fails client-side after a
successful OAuth flow (anthropics/claude-ai-mcp#136, anthropics/claude-code#46140).
Workaround: register the server with the Claude Code CLI
(`claude mcp add -s user --transport http <name> https://<host>/mcp`).
