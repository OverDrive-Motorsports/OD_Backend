# auth-service endpoints

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
| `401` | Invalid/expired `state`, or the provider rejected the code. |
| `403` | The provider does not report the email as verified. |
| `404` | Unknown provider, or (login) no account linked to this identity. |
| `409` | (register) Identity already linked, or the email belongs to an existing account. |
