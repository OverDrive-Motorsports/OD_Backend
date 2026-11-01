# user-data-service database reference

## Overview

This database stores user-scoped configuration data for `user-data-service`.
It currently focuses on two domains:

- external provider connections
- saved layout presets (each holding the client's layout document as JSON)

## Relationships

- none between tables; `userId` refers logically to `auth-service`'s `User.id`

## Entity reference

### `ProviderConnection`

| Field | Type | Required | Constraints | Description | Example |
| --- | --- | --- | --- | --- | --- |
| `id` | `String` | Yes | Primary key, UUID, default generated | Unique identifier of the provider connection | `1d44b8db-1d0a-4f55-bc2f-c9951ad0cb11` |
| `userId` | `String` | Yes | Indexed | Identifier of the connected user | `user_42` |
| `providerId` | `String` | Yes | Indexed | Identifier of the external provider | `f1tv` |
| `configJson` | `Json` | Yes | None | Provider-specific connection settings and secrets metadata | `{"region":"EU","accountEmail":"driver@example.com"}` |
| `createdAt` | `DateTime` | Yes | Default `now()` | Creation timestamp of the provider link | `2026-04-14T10:00:00Z` |
| `updatedAt` | `DateTime` | Yes | Auto-updated | Last update timestamp of the provider link | `2026-04-14T10:15:00Z` |

### `UserPreset`

| Field | Type | Required | Constraints | Description | Example |
| --- | --- | --- | --- | --- | --- |
| `id` | `String` | Yes | Primary key, UUID, default generated | Unique identifier of the saved preset | `4fa6e548-ccf8-4b60-a8b8-6dc1a14ef761` |
| `userId` | `String` | Yes | Indexed | Owner of the preset | `user_42` |
| `name` | `String` | Yes | `varchar(50)` | Human-readable name of the preset | `Race Weekend Layout` |
| `isDefault` | `Boolean` | Yes | Default `false` | Default layout of the user; at most one per user, enforced transactionally by the service | `true` |
| `data` | `Json` | Yes | jsonb, JSON object ≤ 256 KiB (checked by the service) | Layout document produced by the client (widgets, positions, sizes, settings), stored as-is | `{"widgets":[{"type":"video","position":[0,1.2,-2]}]}` |
| `createdAt` | `DateTime` | Yes | Default `now()` | Creation timestamp of the preset | `2026-04-14T10:20:00Z` |
| `updatedAt` | `DateTime` | Yes | Auto-updated | Last update timestamp of the preset | `2026-04-14T10:40:00Z` |

## Constraints and indexes

| Item | Type | Description |
| --- | --- | --- |
| `ProviderConnection(userId, providerId)` | Unique constraint | Prevents duplicate provider links for the same user |
| `ProviderConnection.userId` | Index | Speeds up lookup of a user's provider connections |
| `ProviderConnection.providerId` | Index | Speeds up lookup by provider |
| `UserPreset.userId` | Index | Speeds up preset listing per user |
