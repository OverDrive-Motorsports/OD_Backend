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

### Retrieve User Sessions (`GET /{userID}/sessions`)
#### Response:
```json
{
    "count": 1,
    "sessions": [
        {
            "id": "4caffeeb-91b3-4112-a73e-f7f495383109",
            "userID": "",
            "refreshToken": "",
            "expiresAt": "2026-08-24T21:50:09.496Z",
            "createdAt": "2026-07-25T19:40:59.967Z"
        }
    ]
}
```

### Retrieve Session for a Specific User (`GET /{userID}/session/{sessionID}`)
#### Response:
```json
{
   "id": "4caffeeb-91b3-4112-a73e-f7f495383109",
   "userID": "",
   "refreshToken": "",
   "expiresAt": "2026-08-24T21:50:09.496Z",
   "createdAt": "2026-07-25T19:40:59.967Z"
}
```

### Delete Session for a Specific User (`DELETE /{userID}/session/{sessionID}`)
#### Response:
```json
{
    "status": "ok"
}
```

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
    "expiresAt": "2026-08-12T15:45:02.918Z"
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
    "expiresAt": "2026-08-12T15:45:02.918Z"
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
    "refreshToken": "<refresh_token>",
    "expiresAt": "2026-08-12T15:45:02.918Z"
}
```
