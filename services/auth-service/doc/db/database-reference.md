# auth-service database reference

## Overview

This database supports account identity and refresh-session storage for `auth-service`.
The schema is intentionally compact and centered on two entities:

- `User` for account identity data
- `AuthSession` for refresh-token session lifecycle

## Relationships

- `User` `1 -> N` `AuthSession`
- deleting a `User` cascades deletion to linked `AuthSession` rows

## Entity reference

### `User`

| Field | Type | Required | Constraints | Description | Example |
| --- | --- | --- | --- | --- | --- |
| `id` | `String` | Yes | Primary key, UUID, default generated | Unique identifier of the user account | `550e8400-e29b-41d4-a716-446655440000` |
| `email` | `String` | Yes | Unique, `varchar(255)` | Login email used to identify the user | `driver@example.com` |
| `displayName` | `String` | Yes | `varchar(50)` | Public display name shown in the platform | `Max Verstappen` |
| `passwordHash` | `String` | Yes | None | Hashed password value, never the raw password | `$argon2id$v=19$m=65536,t=3,p=4$...` |
| `role` | `UserRole` (`user`, `admin`) | Yes | Default `user` | Access level issued in the `role` claim of the access token; changed by hand only (`docs/security.md`) | `user` |
| `createdAt` | `DateTime` | Yes | Default `now()` | Account creation timestamp | `YYYY-MM-DDThh:mm:ssZ` |
| `updatedAt` | `DateTime` | Yes | Auto-updated | Last modification timestamp for the user record | `YYYY-MM-DDThh:mm:ssZ` |

### `AuthSession`

| Field | Type | Required | Constraints | Description | Example |
| --- | --- | --- | --- | --- | --- |
| `id` | `String` | Yes | Primary key, UUID, default generated | Unique identifier of the authentication session | `b7a2f6c4-4d22-4f7b-94a7-2d10e6dbd900` |
| `userId` | `String` | Yes | Foreign key to `User.id`, indexed | Owner of the refresh session | `550e8400-e29b-41d4-a716-446655440000` |
| `refreshTokenHash` | `String` | Yes | None | Hashed refresh token stored for verification and rotation | `$argon2id$v=19$m=65536,t=3,p=4$refresh...` |
| `expiresAt` | `DateTime` | Yes | None | Expiration timestamp of the session | `YYYY-MM-DDThh:mm:ssZ` |
| `createdAt` | `DateTime` | Yes | Default `now()` | Session creation timestamp | `YYYY-MM-DDThh:mm:ssZ` |

## Constraints and indexes

| Item | Type | Description |
| --- | --- | --- |
| `User.email` | Unique constraint | Prevents duplicate accounts with the same email |
| `AuthSession.userId` | Index | Speeds up user-to-session lookups |
| `AuthSession.userId -> User.id` | Foreign key | Enforces the link between a session and its user |

## Notes

- `User` stores identity data
- `AuthSession` stores renewable authentication state
- expired sessions can be cleaned independently using `expiresAt`
