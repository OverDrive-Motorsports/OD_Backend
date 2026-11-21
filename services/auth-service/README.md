# auth-service

`auth-service` is the authentication and authorization service of the platform.

## Run

```bash
go run ./services/auth-service
```

Default port: `3001`

Database ownership: `auth-service` owns `AUTH_DATABASE_URL`, which should target its dedicated `overdrive_auth` database locally.

## Routes

### Get Service Health (`GET /health`)
#### Response:
```json
{
    "status": "ok",
    "service": "auth-service"
}
```

### List the caller's sessions (`GET /sessions`)
Requires `Authorization: Bearer <access token>`; the user comes from the token.
#### Response:
```json
[
    {
        "id": "4caffeeb-91b3-4112-a73e-f7f495383109",
        "createdAt": "YYYY-MM-DDThh:mm:ss.sssZ",
        "expiresAt": "YYYY-MM-DDThh:mm:ss.sssZ"
    }
]
```

### Get one of the caller's sessions (`GET /sessions/{sessionId}`)
Same object as above; `404 SESSION_NOT_FOUND` for an unknown session or another user's.

### Revoke one of the caller's sessions (`DELETE /sessions/{sessionId}`)
`204 No Content`; `404 SESSION_NOT_FOUND` for an unknown session or another user's.

### Register User (`POST /register`)
#### Body:
```json
{
    "email": "hello@test.com",
    "password": "bonjour",
    "username": "Hello Test"
}
```
#### Response:
```json
{
    "token": "<token>",
    "refreshToken": "<refresh_token>",
    "sessionId": "<session_id>",
    "expiresAt": "YYYY-MM-DDThh:mm:ss.sssZ"
}
```

### Login User (`POST /login`)
#### Body:
```json
{
    "email": "hello@test.com",
    "password": "bonjour"
}
```
#### Response:
```json
{
    "token": "<token>",
    "refreshToken": "<refresh_token>",
    "sessionId": "<session_id>",
    "expiresAt": "YYYY-MM-DDThh:mm:ss.sssZ"
}
```

### Refresh Token (`POST /refresh`)
#### Body:
```json
{
    "sessionId": "<session_id>",
    "refreshToken": "<refresh_token>"
}
```
#### Response:
```json
{
    "token": "<token>",
    "refreshToken": "<refresh_token>",
    "expiresAt": "YYYY-MM-DDThh:mm:ss.sssZ"
}
```
