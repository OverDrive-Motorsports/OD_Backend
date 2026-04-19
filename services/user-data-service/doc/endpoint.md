# user-data-service endpoints

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
  "service": "user-data-service"
}
```

## Error Messages

No endpoint-specific error response is currently implemented for this handler.
