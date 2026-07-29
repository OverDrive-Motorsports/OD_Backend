# shared/apierror

Standard error type shared by every service (`gateway`, `auth-service`, `user-data-service`,
`championship-service`, `race-data-service`, `ingestion-service`, ...).

One `Error` type, one set of codes/status, one log format.

<br>

## Usage

In a controller: build the error with a constructor, then respond to the client with `Write` —
it logs the technical detail and sends the standardized JSON envelope in one call.

```go
if payload == nil {
    apierror.Write(w, r.URL.Path, apierror.NotFound("SESSION", "No session found for this ID.", nil))
    return
}
```

`Write` sends:
```json
{"error": {"code": "SESSION_NOT_FOUND", "status": 404, "message": "No session found for this ID."}}
```

and logs to stderr: `status=404 code=SESSION_NOT_FOUND path=/sessions/42 err=<technical detail>`

- The constructor's 3rd argument (`err`) is the internal technical error — it goes to the log only, never to the client response.
- `core`/`repository` layers keep returning plain `error` — only the controller builds an `*apierror.Error`, at the point of responding.
- `Write` replaces each service's own `writeJSON(w, status, map[string]any{...})` call for errors — that helper can stay for success responses, it's just no longer needed for errors.

<br>

## Adding a new error message

**1. Reuse an existing constructor first.** If the cause already has one (e.g. `NotFound("RACE", ...)`), just call it with the right message — no new code needed.

**2. No matching constructor? Add one in `apierror.go`:**

```go
// AuthServiceUnreachable reports the auth service being unreachable (502).
func AuthServiceUnreachable(message string, err error) *Error {
    return New(CodeUpstreamUnavailable, StatusBadGateway, message, err)
}
```

**3. No matching `Code`/`Status` either?** Add the `Code` constant to the `const` block at the top (`SCREAMING_SNAKE_CASE`), and a `Status` constant if the HTTP status is new too.

**4. Never reuse `INTERNAL_ERROR` (or an unrelated code) out of convenience** — one code per cause, not per endpoint.


## Available constructors

| Constructor | Code | Status |
|---|---|---|
| `Validation(message, err)` | `VALIDATION_ERROR` | 400 |
| `InvalidField(field, message, err)` | `INVALID_<FIELD>` | 400 |
| `Unauthorized(message, err)` | `UNAUTHORIZED` | 401 |
| `InvalidToken(message, err)` | `INVALID_TOKEN` | 401 |
| `TokenExpired(message, err)` | `TOKEN_EXPIRED` | 401 |
| `Forbidden(message, err)` | `FORBIDDEN` | 403 |
| `NotFound(resource, message, err)` | `<RESOURCE>_NOT_FOUND` | 404 |
| `MethodNotAllowed(message, err)` | `METHOD_NOT_ALLOWED` | 405 |
| `Conflict(resource, message, err)` | `<RESOURCE>_CONFLICT` | 409 |
| `AlreadyExists(resource, message, err)` | `<RESOURCE>_ALREADY_EXISTS` | 409 |
| `RateLimitExceeded(message, err)` | `RATE_LIMIT_EXCEEDED` | 429 |
| `Internal(message, err)` | `INTERNAL_ERROR` | 500 |
| `UpstreamUnavailable(message, err)` | `UPSTREAM_UNAVAILABLE` | 502 |
| `ServiceUnavailable(message, err)` | `SERVICE_UNAVAILABLE` | 503 |
| `UpstreamTimeout(message, err)` | `UPSTREAM_TIMEOUT` | 504 |

<br>

## Adding apierror to a service

`go.mod`:
```
require overdrive/shared/apierror v0.0.0
replace overdrive/shared/apierror => ../../shared/apierror
```

Root `go.work`: `./shared/apierror` is already in the `use` block.
