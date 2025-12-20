# Cloud API Specification

This document specifies the cloud API that RS-1 devices connect to for registration, authentication, OTA updates, telemetry, and WebRTC signaling.

## Architecture

```
RS-1 Device                          api.optic.works
     │                                     │
     │  POST /devices/register             │
     │ ─────────────────────────────────►  │  Go Device Mgmt Service
     │  { serial_number }                  │
     │  ◄───────────────────────────────── │
     │  { temp_token, device_id }          │
     │                                     │
     │  ... Customer pairs in app ...      │
     │                                     │
     │  POST /devices/token                │
     │ ─────────────────────────────────►  │
     │  { temp_token }                     │
     │  ◄───────────────────────────────── │
     │  { cloud_token, device_id }         │
     │                                     │
     │  WS /devices/{id}/signaling         │
     │ ════════════════════════════════════ │
     │  Authorization: Bearer {token}      │
     │                                     │
```

## Endpoints

### POST /devices/register

Device registration at first boot. Returns a temporary token.

**Request:**
```json
{
  "serial_number": "RS1-ABC123",
  "firmware_version": "1.0.0",
  "product_type": "rs1"
}
```

**Response (200):**
```json
{
  "temp_token": "abc123...",
  "device_id": "dev_456",
  "status": "awaiting_pairing"
}
```

**Response (404):**
```json
{
  "error": "device_not_found",
  "message": "Device serial number not registered. Contact support."
}
```

**Response (409):**
```json
{
  "error": "already_registered",
  "message": "Device is already registered",
  "device_id": "dev_456"
}
```

### POST /devices/token

Exchange temporary token for permanent cloud token after customer pairing.

**Request:**
```json
{
  "temp_token": "abc123..."
}
```

**Response (200):**
```json
{
  "cloud_token": "eyJhbGc...",
  "device_id": "dev_456",
  "customer_id": "cus_789"
}
```

**Response (401):**
```json
{
  "error": "invalid_token",
  "message": "Token is invalid or expired"
}
```

**Response (412):**
```json
{
  "error": "not_paired",
  "message": "Device not yet paired to a customer"
}
```

### DELETE /devices/{deviceId}

Deregister device from cloud.

**Headers:**
```
Authorization: Bearer {cloud_token}
```

**Response (200):**
```json
{
  "success": true
}
```

### GET /devices/{deviceId}/ota

Check for firmware updates.

**Headers:**
```
Authorization: Bearer {cloud_token}
```

**Query Parameters:**
- `current_version` - Current firmware version (e.g., "1.0.0")

**Response (200) - Update Available:**
```json
{
  "update_available": true,
  "version": "1.1.0",
  "url": "https://r2.optic.works/firmware/rs1/1.1.0/opticworks-rs1_app",
  "sha256": "abc123...",
  "size_bytes": 15728640,
  "release_notes": "Bug fixes and performance improvements",
  "mandatory": false,
  "min_version": "0.9.0"
}
```

**Response (200) - No Update:**
```json
{
  "update_available": false,
  "current_version": "1.1.0"
}
```

### POST /devices/{deviceId}/telemetry

Submit device telemetry data.

**Headers:**
```
Authorization: Bearer {cloud_token}
Content-Type: application/json
```

**Request:**
```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "metrics": {
    "uptime_seconds": 86400,
    "memory_free_mb": 128,
    "memory_total_mb": 256,
    "cpu_temp_c": 45.2,
    "wifi_rssi": -65,
    "wifi_ssid": "HomeNetwork",
    "radar_detections_1h": 42,
    "video_fps": 30,
    "video_bitrate_kbps": 2500
  },
  "events": [
    {
      "type": "boot",
      "timestamp": "2024-01-14T10:30:00Z"
    },
    {
      "type": "ota_applied",
      "timestamp": "2024-01-14T10:35:00Z",
      "data": { "from": "1.0.0", "to": "1.1.0" }
    }
  ]
}
```

**Response (200):**
```json
{
  "received": true
}
```

### WS /devices/{deviceId}/signaling

WebSocket connection for WebRTC signaling.

**Connection Headers:**
```
Authorization: Bearer {cloud_token}
X-Device-ID: {serial_number}
X-App-Version: {firmware_version}
```

**Message Types:**

```json
// Ping (device → cloud)
{ "type": "ping" }

// Pong (cloud → device)
{ "type": "pong" }

// Session request (cloud → device)
{
  "type": "session_request",
  "request_id": "req_123",
  "offer": {
    "sdp": "v=0\r\n..."
  },
  "ice_servers": [
    { "urls": "stun:stun.l.google.com:19302" }
  ]
}

// Session answer (device → cloud)
{
  "type": "session_answer",
  "request_id": "req_123",
  "answer": {
    "sdp": "v=0\r\n..."
  }
}

// ICE candidate (bidirectional)
{
  "type": "ice_candidate",
  "request_id": "req_123",
  "candidate": {
    "candidate": "candidate:...",
    "sdpMid": "0",
    "sdpMLineIndex": 0
  }
}

// Session end (cloud → device)
{
  "type": "session_end",
  "request_id": "req_123"
}

// Error (bidirectional)
{
  "type": "error",
  "request_id": "req_123",
  "code": "session_failed",
  "message": "Failed to establish WebRTC connection"
}
```

## Authentication

### Device Authentication

Devices authenticate using a bearer token obtained through the registration/pairing flow:

1. Device registers with serial number → receives `temp_token`
2. Customer pairs device in app → device becomes paired
3. Device exchanges `temp_token` for `cloud_token`
4. All subsequent requests use `Authorization: Bearer {cloud_token}`

### Token Format

Tokens are cryptographically random hex strings:
- `temp_token`: 32 bytes (64 hex chars)
- `cloud_token`: 64 bytes (128 hex chars)

### Token Validation

The Go service validates tokens by querying the `device` table:

```sql
SELECT id, serial_number, customer_id, firmware_version
FROM device
WHERE cloud_token = $1
  AND customer_id IS NOT NULL
```

## Database Schema

The Go service reads from and writes to the same PostgreSQL database as Medusa:

### Read Tables (Medusa-managed)

```sql
-- device table (created by Medusa migrations)
CREATE TABLE device (
  id VARCHAR PRIMARY KEY,
  serial_number VARCHAR UNIQUE NOT NULL,
  product_type VARCHAR DEFAULT 'rs1',
  firmware_version VARCHAR,
  customer_id VARCHAR,
  order_id VARCHAR,
  cloud_token VARCHAR,
  temp_token VARCHAR,
  last_seen_at TIMESTAMP,
  registered_at TIMESTAMP,
  paired_at TIMESTAMP,
  metadata JSONB,
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW()
);
```

### Write Tables (Go-managed)

```sql
-- device_telemetry (created by Go migrations)
CREATE TABLE device_telemetry (
  id SERIAL PRIMARY KEY,
  device_id VARCHAR NOT NULL REFERENCES device(id),
  timestamp TIMESTAMP NOT NULL,
  metrics JSONB NOT NULL,
  events JSONB,
  created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_device_telemetry_device_id ON device_telemetry(device_id);
CREATE INDEX idx_device_telemetry_timestamp ON device_telemetry(timestamp);

-- device_ota_updates (created by Go migrations)
CREATE TABLE device_ota_updates (
  id SERIAL PRIMARY KEY,
  device_id VARCHAR NOT NULL REFERENCES device(id),
  from_version VARCHAR NOT NULL,
  to_version VARCHAR NOT NULL,
  started_at TIMESTAMP NOT NULL,
  completed_at TIMESTAMP,
  status VARCHAR DEFAULT 'pending', -- pending, downloading, applying, completed, failed
  error_message VARCHAR,
  created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_device_ota_device_id ON device_ota_updates(device_id);
```

## Service Structure

```
cmd/
  devicemgmt/
    main.go              # Entry point

internal/
  api/
    handlers.go          # HTTP handlers
    middleware.go        # Auth middleware
    websocket.go         # WebSocket signaling hub

  db/
    device.go            # Device queries
    telemetry.go         # Telemetry writes
    ota.go               # OTA update tracking
    migrations/          # Go-specific migrations

  ota/
    releases.go          # Release management
    r2.go                # R2 storage client

  auth/
    token.go             # Token validation

  signaling/
    hub.go               # WebSocket connection hub
    session.go           # WebRTC session management

config/
  config.go              # Environment configuration
```

## Configuration

```bash
# Required
DATABASE_URL=postgres://user:pass@host:5432/medusa
JWT_SECRET=...

# Optional
PORT=8080
R2_ENDPOINT=https://xxx.r2.cloudflarestorage.com
R2_ACCESS_KEY_ID=...
R2_SECRET_ACCESS_KEY=...
R2_BUCKET=opticworks-firmware
SENTRY_DSN=...
```

## Deployment

The Go service runs alongside Medusa on the same Hetzner node:

```nginx
# Nginx configuration
location /devices/ {
    proxy_pass http://127.0.0.1:8080;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
}
```

PM2 process configuration:

```json
{
  "name": "opticworks-devicemgmt",
  "script": "./devicemgmt",
  "cwd": "/opt/opticworks/device-mgmt",
  "env": {
    "DATABASE_URL": "...",
    "PORT": "8080"
  }
}
```

## Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `device_not_found` | 404 | Serial number not registered |
| `already_registered` | 409 | Device already registered |
| `invalid_token` | 401 | Token is invalid or expired |
| `not_paired` | 412 | Device not paired to customer |
| `unauthorized` | 401 | Missing or invalid authorization |
| `session_failed` | 500 | WebRTC session failed |

## Rate Limits

| Endpoint | Limit |
|----------|-------|
| POST /devices/register | 5/min per IP |
| POST /devices/token | 10/min per device |
| POST /devices/telemetry | 60/min per device |
| WebSocket connection | 1 per device |
