# auth-service

`auth-service` is the authentication and authorization service of the platform.

## Run

```bash
go run ./services/auth-service
```

Default port: `3001`

Database ownership: `auth-service` owns `AUTH_DATABASE_URL`, which should target its dedicated `overdrive_auth` database locally.

## Current behavior

- exposes `GET /health` and returns the service status and service name
- exposes `POST /login` and `POST /register` to login or register a user
- exposes `GET/POST /{userID}/sessions` for retrieving or creating user sessions
- exposes `GET/DELETE /{userID}/session/{sessionID}` for retrieving or deleting user session

## Planned scope

- login and identity flows
- access and refresh tokens
- authorization rules
- account security policies
