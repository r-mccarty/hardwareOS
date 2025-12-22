# Cloud API (v1 - current)

This document describes the current Cloudflare Workers API used by RS-1 devices and the cloud UI.

## Status

- Current implementation: `services/cloud-api/` (Cloudflare Workers + D1)
- WebRTC signaling: not implemented in v1
- Ambitious v2 design (shelved): `docs/legacy/CLOUD_API_v2.md`

## Architecture

```
RS-1 Device / Cloud UI          Cloudflare Workers + D1
     |                                   |
     |  POST /devices/register           |
     |  POST /devices/token              |
     |  POST /devices/:id/telemetry      |
     |  GET  /devices/:id/ota            |
     |  POST /devices/:id/ota/status     |
     |                                   |
     |  POST /auth/login (UI)            |
     |  POST /auth/signup (UI)           |
     |  GET  /me (UI)                    |
     |  GET  /devices (UI)               |
```

## Base URL

- Local dev: `http://localhost:3000` (Wrangler)
- Production: `https://api.optic.works` (or configured Cloudflare domain)

## Authentication

### Device Authentication

Devices authenticate with a bearer token obtained during pairing:

1. Device registers with serial number -> receives `temp_token`
2. UI pairs device to a user -> device becomes paired
3. Device exchanges `temp_token` for `cloud_token`
4. Device calls protected endpoints with `Authorization: Bearer {cloud_token}`

Token format:
- `temp_token`: 32 bytes (64 hex chars)
- `cloud_token`: 64 bytes (128 hex chars)

### User Authentication

The cloud UI uses a session cookie (`opticworks_session`) created by `/auth/login` or `/auth/signup`.
Sessions are stored hashed in D1.

## Endpoints

### Health

- `GET /` -> service status and environment
- `GET /health` -> `{ "status": "ok" }`

### Device Registration

#### POST /devices/register

Registers a device and returns a temporary token.

Request:
```json
{
  "serial_number": "RS1-ABC123",
  "firmware_version": "1.0.0",
  "product_type": "rs1"
}
```

Response:
```json
{
  "temp_token": "abc123...",
  "device_id": "dev_456",
  "status": "awaiting_pairing"
}
```

Errors:
- `400 invalid_request`
- `409 already_registered`

#### POST /devices/:deviceId/pair

Pairs a device to a user (cloud UI flow).

Request:
```json
{
  "user_id": "usr_demo"
}
```

Response:
```json
{
  "success": true,
  "device_id": "dev_456",
  "customer_id": "usr_demo"
}
```

#### POST /devices/token

Exchange temporary token for permanent cloud token after pairing.

Request:
```json
{
  "temp_token": "abc123..."
}
```

Response:
```json
{
  "cloud_token": "def456...",
  "device_id": "dev_456",
  "customer_id": "usr_demo"
}
```

Errors:
- `401 invalid_token`
- `412 not_paired`

#### DELETE /devices/:deviceId

Deregister device from cloud.

Headers:
```
Authorization: Bearer {cloud_token}
```

Response:
```json
{ "success": true }
```

#### GET /devices/:deviceId

Fetch device info (device-authenticated).

Headers:
```
Authorization: Bearer {cloud_token}
```

Response:
```json
{
  "id": "dev_456",
  "serial_number": "RS1-ABC123",
  "product_type": "rs1",
  "firmware_version": "1.0.0",
  "customer_id": "usr_demo",
  "last_seen_at": "2024-01-15T10:30:00Z",
  "registered_at": "2024-01-15T10:00:00Z",
  "paired_at": "2024-01-15T10:05:00Z"
}
```

#### GET /devices

List devices for the authenticated user (cloud UI).

Response:
```json
{
  "devices": [
    {
      "id": "dev_456",
      "name": "RS1-ABC123",
      "online": true,
      "lastSeen": "2024-01-15T10:30:00Z",
      "version": "1.0.0"
    }
  ]
}
```

### OTA

#### GET /devices/:deviceId/ota

Check for firmware updates.

Headers:
```
Authorization: Bearer {cloud_token}
```

Query:
- `current_version` (optional)

Response (update available):
```json
{
  "update_available": true,
  "current_version": "1.0.0",
  "version": "1.1.0",
  "url": "https://.../opticworks-rs1_app",
  "sha256": "abc123...",
  "size_bytes": 15728640,
  "release_notes": "Bug fixes and performance improvements",
  "mandatory": false,
  "min_version": "0.9.0"
}
```

Response (no update):
```json
{
  "update_available": false,
  "current_version": "1.1.0"
}
```

#### POST /devices/:deviceId/ota/status

Report OTA progress.

Request:
```json
{
  "version": "1.1.0",
  "status": "downloading",
  "from_version": "1.0.0"
}
```

Response:
```json
{ "received": true }
```

### Telemetry

#### POST /devices/:deviceId/telemetry

Submit telemetry.

Headers:
```
Authorization: Bearer {cloud_token}
Content-Type: application/json
```

Request:
```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "metrics": {
    "uptime_seconds": 86400,
    "memory_free_mb": 128,
    "cpu_temp_c": 45.2
  },
  "events": [
    { "type": "boot", "timestamp": "2024-01-14T10:30:00Z" }
  ]
}
```

Response:
```json
{ "received": true }
```

#### GET /devices/:deviceId/telemetry

Fetch recent telemetry (admin/debug).

Query:
- `limit` (default 100, max 1000)

Response:
```json
{
  "telemetry": [
    {
      "id": 1,
      "timestamp": "2024-01-15T10:30:00Z",
      "metrics": { "uptime_seconds": 86400 },
      "events": null,
      "created_at": "2024-01-15T10:31:00Z"
    }
  ]
}
```

### Cloud UI Auth

#### POST /auth/login

Request:
```json
{ "email": "demo@optic.works" }
```

Response:
```json
{ "success": true, "user": { "id": "usr_demo", "email": "demo@optic.works" } }
```

#### POST /auth/signup

Request:
```json
{ "email": "demo@optic.works", "name": "Demo User" }
```

Response:
```json
{ "success": true, "user": { "id": "usr_demo", "email": "demo@optic.works" } }
```

#### POST /auth/logout

Response:
```json
{ "success": true }
```

#### GET /me

Response:
```json
{
  "id": "usr_demo",
  "email": "demo@optic.works",
  "name": "Demo User",
  "devices": [
    { "id": "dev_456", "serial_number": "RS1-ABC123" }
  ]
}
```

### Frontend Compatibility

- `GET /device/status` returns `{ "isSetup": true, "isOnDevice": false }`
- `GET /device` returns `404 not_on_device`

## Data Storage (D1)

Key tables:

- `users`, `user_sessions` (cloud UI auth)
- `devices` (registration + pairing)
- `device_telemetry` (metrics)
- `device_ota_updates` (OTA status)
- `firmware_releases` (available firmware)

See `services/cloud-api/src/db/schema.sql` for full schema.

## Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `invalid_request` | 400 | Missing or invalid fields |
| `invalid_token` | 401 | Token is invalid or expired |
| `not_paired` | 412 | Device not paired |
| `already_registered` | 409 | Device already registered |
| `already_paired` | 409 | Device already paired |
| `user_exists` | 409 | User already exists |
| `version_too_old` | 400 | Firmware below minimum required version |
| `unauthorized` | 401 | Missing/invalid auth |
| `device_not_found` | 404 | Device not found |
| `not_on_device` | 404 | Cloud endpoint used for on-device route |
| `not_found` | 404 | Route does not exist |
| `internal_error` | 500 | Unhandled server error |
