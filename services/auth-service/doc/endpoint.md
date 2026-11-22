# auth-service endpoints

Through the gateway these routes are under `/v1/auth` (and `/auth` during the transition):
`POST /v1/auth/login` reaches `POST /login`, `GET /v1/auth/sessions` reaches `GET /sessions`.
Login, register, refresh and the OAuth routes are public; the session routes need the user's
access token (see `gateway/ROUTES.md`).

Every route reading a JSON body (login, register, refresh, OAuth login and register) accepts at
most 16 KiB: a larger body answers `413 PAYLOAD_TOO_LARGE`
(`{"error":{"code":"PAYLOAD_TOO_LARGE","status":413,"message":"request body exceeds the 16384 bytes limit"}}`).

## Access token

The `token` returned by login, register, OAuth and refresh is an HS256 JWT valid 15 minutes:

| Claim | Description |
| --- | --- |
| `sub` | User ID. The only source of the caller's identity in every service. |
| `role` | `user` or `admin`, copied from `User.role` at each login and refresh. |
| `email`, `iat`, `exp` | Email, issue and expiry times. |

### Roles

Accounts are created as `user`. `admin` opens the administration routes of the gateway
(`/v1/ingestion`); there is no API to change a role. Promote or demote by hand on the auth
database:

```sql
UPDATE "User" SET "role" = 'admin' WHERE "email" = 'operator@example.com';
UPDATE "User" SET "role" = 'user'  WHERE "email" = 'operator@example.com';
```

The change applies at the account's next login or refresh (at most 15 minutes for a token
already issued).

## Endpoint

`GET /health`

## Description

Returns the runtime status of the service.

## Parameters

| Parameter | Location | Mandatory | Description |
| --- | --- | --- | --- |
| None | - | No | This endpoint does not accept path parameters, query parameters, headers, or request body fields. |

## Returned Message

Status: `200 OK`

```json
{
  "status": "ok",
  "service": "auth-service"
}
```

## Error Messages

No endpoint-specific error response is currently implemented for this handler.

## OAuth flow

Providers are pluggable (see `src/adapters/oauth/registry.go`); a provider is enabled only when
its environment variables are set (e.g. `GOOGLE_OAUTH_CLIENT_ID`, `GOOGLE_OAUTH_CLIENT_SECRET`,
`GOOGLE_OAUTH_REDIRECT_URL`).

1. `GET /oauth/providers` lists the enabled providers.
2. `GET /oauth/providers/{providerId}/authorize` returns `url`, `state` and `codeVerifier`. The
   client stores `state` + `codeVerifier` (e.g. `sessionStorage`) and redirects the user to `url`.
3. The provider redirects to the configured redirect URL with `?code=...&state=...`. The client
   checks that `state` equals the stored one, then calls login or register with
   `{ code, state, codeVerifier }`.
4. If login returns `404` (no linked account), restart at step 2 and call register: an
   authorization code is single-use.

## Endpoint

`GET /oauth/providers`

## Returned Message

Status: `200 OK`

```json
[{ "id": "google", "name": "Google" }]
```

## Endpoint

`GET /oauth/providers/{providerId}/authorize`

## Returned Message

Status: `200 OK`

```json
{
  "url": "https://accounts.google.com/o/oauth2/v2/auth?...",
  "state": "<signed, expires in 10 minutes>",
  "codeVerifier": "<PKCE verifier, keep client-side>"
}
```

## Error Messages

| Status | Cause |
| --- | --- |
| `404` | Unknown or disabled `providerId`. |

## Endpoint

`POST /oauth/{providerId}/login` and `POST /oauth/{providerId}/register`

## Parameters

| Parameter | Location | Mandatory | Description |
| --- | --- | --- | --- |
| `providerId` | Path | Yes | Provider id from `GET /oauth/providers`. |
| `code` | Body | Yes | Authorization code from the provider redirect. |
| `state` | Body | Yes | State returned by the authorize endpoint. |
| `codeVerifier` | Body | Yes | PKCE verifier returned by the authorize endpoint. |
| `username` | Body | No | Register only. Defaults to the provider display name, then the email local part. |

## Returned Message

Status: `200 OK`, same body as `POST /login`.

## Error Messages

| Status | Cause |
| --- | --- |
| `400` | Malformed body. |
| `413` | Body over 16 KiB. |
| `401` | Invalid/expired `state`, or the provider rejected the code. |
| `403` | The provider does not report the email as verified. |
| `404` | Unknown provider, or (login) no account linked to this identity. |
| `409` | (register) Identity already linked, or the email belongs to an existing account. |

## Sessions

The caller's own refresh sessions (one per login). Every route requires
`Authorization: Bearer <access token>` and acts on the token's user only: no route takes a user
ID. A session of another user answers `404`, like a missing one. Responses never carry the
refresh token or its hash.

### Endpoint

`GET /sessions`

### Returned Message

Status: `200 OK`, a bare array (empty when the user has no session):

```json
[{ "id": "8d6c...", "createdAt": "YYYY-MM-DDThh:mm:ssZ", "expiresAt": "YYYY-MM-DDThh:mm:ssZ" }]
```

### Endpoint

`GET /sessions/{sessionId}`

### Returned Message

Status: `200 OK`, one session object as above.

### Endpoint

`DELETE /sessions/{sessionId}`

Revokes the session: its refresh token stops working. The current access token stays valid until
it expires.

### Returned Message

Status: `204 No Content`, empty body.

### Error Messages

| Status | Code | Cause |
| --- | --- | --- |
| `401` | `UNAUTHORIZED` | No `Authorization: Bearer` header. |
| `401` | `INVALID_TOKEN` | Invalid or expired access token (the gateway answers `TOKEN_EXPIRED` first for an expired one). |
| `404` | `SESSION_NOT_FOUND` | Unknown session, or one that belongs to another user. |
| `500` | `INTERNAL` | Database failure. |
