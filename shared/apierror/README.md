# shared/apierror

Standardized error type shared by every OverDrive backend service (`gateway`, `auth-service`,
`user-data-service`, `championship-service`, `race-data-service`, `ingestion-service`, and future
services). One `Error` type, one set of error codes, one terminal log line - no service invents
its own. Writing the actual HTTP response stays each service's job (see below).

## Adding it to a service

`go.mod`:

```
require overdrive/shared/apierror v0.0.0

replace overdrive/shared/apierror => ../../shared/apierror
```

Root `go.work`: add `./shared/apierror` to the `use` block.

## Usage

Build an `*apierror.Error` with a constructor, log it, then write the client response yourself
with the service's existing JSON helper:

```go
func (c *SessionController) GetSession(w http.ResponseWriter, r *http.Request) {
    payload, err := c.usecase.GetSession(r.Context(), r.PathValue("sessionId"))
    if err != nil {
        apiErr := apierror.Internal("Something went wrong.", err)
        apiErr.LogDebug(r.URL.Path)
        writeJSON(w, int(apiErr.Status), map[string]any{"error": map[string]any{
            "code": apiErr.Code, "status": apiErr.Status, "message": apiErr.Message,
        }})
        return
    }
    if payload == nil {
        apiErr := apierror.NotFound("SESSION", "No session found for this ID.", nil)
        apiErr.LogDebug(r.URL.Path)
        writeJSON(w, int(apiErr.Status), map[string]any{"error": map[string]any{
            "code": apiErr.Code, "status": apiErr.Status, "message": apiErr.Message,
        }})
        return
    }
    writeJSON(w, http.StatusOK, payload)
}
```

`LogDebug(path)` prints a plain-text debug line to stderr:
`status=404 code=SESSION_NOT_FOUND path=/sessions/42 err=<technical detail>` - `Err` (the
technical cause) only ever appears here, never in the response written to the client.

`core`/`repository` layers should keep returning plain `error` - it's the HTTP-facing controller
that translates into an `*apierror.Error` at the point of response, keeping the domain layer free
of this dependency.

## Constructors

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

`err` is the technical cause (may be `nil`) - it never reaches the client, only the terminal log.
`message` is client-safe English text.

If none of these fit, add a new constructor/code rather than reusing `Internal` or an unrelated
one - see the parent ticket for the full code table.

## Logging note

`LogDebug` writes to stderr. Each service runs in its own Docker container
(`docker-compose.yml`), so `docker logs <service>` (or `docker compose logs -f <service>`) shows
only that service's error lines - no cross-service mixing, no extra wiring needed.
