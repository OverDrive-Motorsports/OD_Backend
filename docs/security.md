# OverDrive Security Notes

This document describes the current backend security hardening baseline.

## Current Scope

The backend currently exposes a mostly public HTTP API focused on race ingestion and stored race data access.

The security work added here is a hardening phase, not a full identity platform. The implemented baseline includes:

- security response headers
- configurable CORS allowlist
- in-memory per-IP rate limiting
- oversized request target rejection
- JWT validation hooks
- RBAC preparation hooks
- suspicious activity logging

## Implemented Protections

### Security Headers

Every response includes:

- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `Referrer-Policy: no-referrer`
- `Content-Security-Policy: default-src 'none'; frame-ancestors 'none'; base-uri 'none'`
- `Cache-Control: no-store`

### CORS

Browser requests are filtered through `CORS_ALLOWED_ORIGINS`.

Behavior:
- no `Origin` header: request is treated as non-browser/server-to-server and is not blocked by CORS
- allowed `Origin`: request proceeds
- blocked `Origin`: backend returns `403 origin not allowed`
- allowed `OPTIONS` preflight: backend returns `204`

### Rate Limiting

The backend applies an in-memory per-IP limiter.

Behavior:
- if a client exceeds the configured quota, the backend returns `429 rate limit exceeded`
- `Retry-After` is returned
- `/health` and CORS preflight requests are excluded

### Request Target Validation

The backend rejects obviously abusive request targets:

- path too long
- query string too long

### JWT Validation

If a client sends a token, the backend validates it.

Current behavior:
- no token: current public endpoints remain accessible
- invalid token: `401 invalid or expired token`
- valid token: auth context is attached to the request

Supported checks:
- `HS256` signature
- `exp`
- `nbf`
- `iss` if `JWT_ISSUER` is configured
- `aud` if `JWT_AUDIENCE` is configured

### RBAC Preparation

The backend includes an internal `requireRoles(...)` middleware hook.

Current state:
- no route is yet protected by role
- the hook exists to avoid redesigning middleware later when protected endpoints appear

### Suspicious Activity Logging

Structured warning logs are emitted for:

- blocked origins
- invalid JWTs
- auth attempts while JWT secret is missing
- oversized request targets
- rate limit violations

## What Is Not Done Yet

This hardening phase does **not** yet provide:

- mandatory authentication on business endpoints
- login / refresh / logout flows
- a real protected WebSocket endpoint
- role-gated admin/operator routes
- distributed/shared-store rate limiting
- automated dependency scanning in CI
- full GDPR workflow enforcement

So the correct reading is:

- the backend now has a stronger HTTP security baseline
- it does not yet have a fully productized auth/authorization system

## Local Setup

At minimum, set:

```env
DATABASE_URL=postgresql://user:pass@127.0.0.1:5432/OverDriveDB?schema=public
CORS_ALLOWED_ORIGINS=http://localhost:3000
JWT_SECRET=replace-me
JWT_ISSUER=overdrive
JWT_AUDIENCE=overdrive-client
```

## Operational Notes

- `curl` is not subject to browser CORS restrictions
- browser frontends must be explicitly allowlisted
- the rate limiter is in-memory and resets on process restart
- horizontal scaling would need a shared limiter store later
