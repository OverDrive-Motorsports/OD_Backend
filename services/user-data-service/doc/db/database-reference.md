# user-data-service database reference

## Overview

This database stores user-scoped configuration data for `user-data-service`.
It currently focuses on two domains:

- external provider connections
- saved layout presets and widgets

## Enum reference

### `WidgetType`

| Value | Description | Example usage |
| --- | --- | --- |
| `video` | Widget used to display a video or stream surface in a saved layout | A preset containing an onboard video panel |

## Relationships

- `UserPreset` `1 -> N` `PresetWidget`
- deleting a `UserPreset` cascades deletion to linked `PresetWidget` rows

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
| `isDefault` | `Boolean` | Yes | None | Indicates whether the preset is the default layout for the user | `true` |
| `createdAt` | `DateTime` | Yes | Default `now()` | Creation timestamp of the preset | `2026-04-14T10:20:00Z` |
| `updatedAt` | `DateTime` | Yes | Auto-updated | Last update timestamp of the preset | `2026-04-14T10:40:00Z` |

### `PresetWidget`

| Field | Type | Required | Constraints | Description | Example |
| --- | --- | --- | --- | --- | --- |
| `id` | `String` | Yes | Primary key, UUID, default generated | Unique identifier of the widget | `a9d16c85-bca5-4d25-b0d8-d3a0d56bd8bb` |
| `presetId` | `String` | Yes | Foreign key to `UserPreset.id`, indexed | Identifier of the parent preset | `4fa6e548-ccf8-4b60-a8b8-6dc1a14ef761` |
| `type` | `WidgetType` | Yes | Enum | Widget category used by the frontend renderer | `video` |
| `settings` | `Json` | Yes | None | Widget-specific configuration payload | `{"stream":"onboard","driverNumber":1}` |
| `posX` | `Float` | Yes | None | Widget X position in the layout space | `1.25` |
| `posY` | `Float` | Yes | None | Widget Y position in the layout space | `0.40` |
| `posZ` | `Float` | Yes | None | Widget Z position in the layout space | `-2.00` |
| `rotX` | `Float` | Yes | None | Widget rotation on the X axis | `0.00` |
| `rotY` | `Float` | Yes | None | Widget rotation on the Y axis | `15.00` |
| `rotZ` | `Float` | Yes | None | Widget rotation on the Z axis | `0.00` |
| `width` | `Float` | Yes | None | Widget width in layout units | `1.80` |
| `height` | `Float` | Yes | None | Widget height in layout units | `1.00` |
| `orderIndex` | `Int` | Yes | Default `0` | Display order inside the preset | `2` |

## Constraints and indexes

| Item | Type | Description |
| --- | --- | --- |
| `ProviderConnection(userId, providerId)` | Unique constraint | Prevents duplicate provider links for the same user |
| `ProviderConnection.userId` | Index | Speeds up lookup of a user's provider connections |
| `ProviderConnection.providerId` | Index | Speeds up lookup by provider |
| `UserPreset.userId` | Index | Speeds up preset listing per user |
| `PresetWidget.presetId` | Index | Speeds up full layout loading |
| `PresetWidget.presetId -> UserPreset.id` | Foreign key | Enforces the link between a widget and its preset |
