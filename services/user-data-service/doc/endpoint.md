# user-data-service endpoints

All responses are **camelCase** JSON. Lists are **bare JSON arrays**. Errors use the shared
envelope `{ "error": { "code", "status", "message" } }`.

## `GET /health`

Returns the runtime status of the service. Open (no authentication), used by probes.

## Presets

A preset is one saved AR/VR layout of the **authenticated user**. Its `data` is the layout
document produced by the client (widgets, positions, sizes, settings...), stored as-is: the
backend only checks it is a JSON object of at most 256 KiB and never interprets it.

```json
{
  "id": "4fa6e548-ccf8-4b60-a8b8-6dc1a14ef761",
  "name": "Race weekend",
  "isDefault": true,
  "data": { "widgets": [ { "type": "video", "position": [0, 1.2, -2] } ] },
  "createdAt": "2026-11-01T10:00:00Z",
  "updatedAt": "2026-11-01T10:05:00Z"
}
```

Postgres stores `data` as `jsonb`: the content round-trips unchanged, but key order and
whitespace may be normalised.

### Authentication and ownership

Every preset route requires `Authorization: Bearer <access token>`, the JWT returned by
`POST /auth/login` (not the gateway token). The user is **only** taken from the token's `sub`
claim: no route accepts a user ID in the path, query or body (an extra `userId` body field is
rejected as an unknown field). A preset owned by another user answers `404`, exactly like a
missing one.

| Situation | Status | `code` |
| --- | --- | --- |
| No / non-Bearer `Authorization` header | `401` | `UNAUTHORIZED` |
| Expired access token | `401` | `TOKEN_EXPIRED` |
| Any other invalid token | `401` | `INVALID_TOKEN` |

Through the gateway the public paths are `/presets` and `/presets/{presetId}`; the gateway
strips its `/presets` prefix, so the service itself serves them on `/` and `/{presetId}`.

### Rules

- `name`: required, non-blank, at most 50 characters
- `data`: required on creation, a JSON object, at most 256 KiB
- at most **20 presets per user**
- a user's **first** preset always becomes the default
- setting `isDefault: true` on a preset clears it on the user's other presets in the same
  transaction, a user never has two defaults
- deleting the default preset does not promote another one
- request bodies: one JSON object, unknown fields and trailing data rejected, at most 320 KiB

### `GET /presets`

The caller's presets, newest first. `[]` when none.

### `POST /presets`

```json
{ "name": "Race weekend", "isDefault": false, "data": { "widgets": [] } }
```

`isDefault` is optional (default `false`, forced to `true` for the first preset).

- `201`, the created preset
- `400` `VALIDATION_ERROR`, invalid body or rule violation (message names the field)
- `409` `PRESET_CONFLICT`, the user already owns 20 presets

### `GET /presets/{presetId}`

- `200`, the preset
- `404` `PRESET_NOT_FOUND`, missing or owned by another user

### `PATCH /presets/{presetId}`

Partial update; at least one field. An absent field (or `"data": null`) is left unchanged.

```json
{ "name": "Qualifying", "isDefault": true, "data": { "widgets": [] } }
```

- `200`, the updated preset
- `400` `VALIDATION_ERROR`, empty update, invalid body or rule violation
- `404` `PRESET_NOT_FOUND`

`isDefault: false` on the default preset leaves the user without a default.

### `DELETE /presets/{presetId}`

- `204`, deleted (empty body)
- `404` `PRESET_NOT_FOUND`

### Example (through the gateway)

```bash
TOKEN=$(curl -s -X POST http://localhost:3000/auth/login \
  -H "Authorization: Bearer $GATEWAY_AUTH_TOKEN" -H "Content-Type: application/json" \
  -d '{"email":"me@example.com","password":"..."}' | jq -r .token)

curl -s -X POST http://localhost:3000/presets -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" -d '{"name":"Race weekend","data":{"widgets":[]}}'
curl -s http://localhost:3000/presets -H "Authorization: Bearer $TOKEN"
```
