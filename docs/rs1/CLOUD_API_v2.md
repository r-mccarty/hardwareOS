# Cloud API Specification

This document specifies the cloud API that RS-1 devices connect to for registration, authentication, OTA updates, telemetry, and WebRTC signaling.

## Security Model

RS-1 devices use a hardware-bound identity model leveraging the RV1106's security features:

- **Device Identity**: Asymmetric keypair generated in TrustZone, private key never leaves secure world
- **Root of Trust**: Public key hash burned to eFuse at manufacturing, verified by BootROM
- **Authentication**: Challenge-response signatures, not bearer tokens
- **Firmware Integrity**: Secure boot chain + dm-verity verified rootfs
- **Anti-Cloning**: Device identity cannot be extracted or duplicated

```
┌─────────────────────────────────────────────────────────────────┐
│  Manufacturing (one-time)                                        │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │  1. Burn OpticWorks signing key hash to eFuse               ││
│  │  2. Device generates ECDSA P-256 keypair in TrustZone       ││
│  │  3. Device exports public key                               ││
│  │  4. Provisioning station registers serial ↔ pubkey          ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Runtime Authentication                                          │
│                                                                  │
│  RS-1 Device                         api.optic.works            │
│       │                                     │                    │
│       │  POST /auth/challenge               │                    │
│       │ ───────────────────────────────────►│                    │
│       │  { serial_number }                  │                    │
│       │  ◄─────────────────────────────────│                    │
│       │  { challenge, expires_at }          │                    │
│       │                                     │                    │
│       │         [TrustZone signs challenge] │                    │
│       │                                     │                    │
│       │  POST /auth/verify                  │                    │
│       │ ───────────────────────────────────►│                    │
│       │  { serial_number, signature }       │                    │
│       │  ◄─────────────────────────────────│                    │
│       │  { session_token, device_id,        │                    │
│       │    expires_at }                     │                    │
│       │                                     │                    │
│       │  [All subsequent requests]          │                    │
│       │  Authorization: Bearer {session}    │                    │
│       │                                     │                    │
└─────────────────────────────────────────────────────────────────┘
```

### Security Properties

| Threat | Mitigation |
|--------|------------|
| Token extraction from flash | No persistent token; session derived from challenge-response |
| Device cloning | Private key in TrustZone cannot be extracted |
| Credential brute force | No credentials; cryptographic identity only |
| Man-in-the-middle | TLS + challenge is device-specific and time-bound |
| Malicious firmware | Secure boot rejects unsigned code |
| Rootfs tampering | dm-verity fails, device panics |
| Firmware downgrade | Anti-rollback counter in eFuse |

## Architecture

```
RS-1 Device                          api.optic.works
     │                                     │
     │  POST /auth/challenge               │
     │ ─────────────────────────────────►  │  Go Device Mgmt Service
     │  { serial_number }                  │
     │  ◄───────────────────────────────── │
     │  { challenge, expires_at }          │
     │                                     │
     │  POST /auth/verify                  │
     │ ─────────────────────────────────►  │
     │  { serial_number, signature }       │
     │  ◄───────────────────────────────── │
     │  { session_token, device_id }       │
     │                                     │
     │  ... Customer pairs in app ...      │
     │                                     │
     │  WS /devices/{id}/signaling         │
     │ ════════════════════════════════════ │
     │  Authorization: Bearer {session}    │
     │                                     │
```

## Endpoints

### POST /auth/challenge

Request a challenge for device authentication. Challenge is bound to the device serial and has a short TTL.

**Request:**
```json
{
  "serial_number": "RS1-ABC123"
}
```

**Response (200):**
```json
{
  "challenge": "a]~E/9[Qx!...", 
  "challenge_id": "ch_789",
  "expires_at": "2024-01-15T10:31:00Z",
  "algorithm": "ES256"
}
```

**Response (404):**
```json
{
  "error": "device_not_found",
  "message": "Device serial number not registered. Contact support."
}
```

### POST /auth/verify

Verify the device's signature over the challenge. Returns a short-lived session token.

**Request:**
```json
{
  "serial_number": "RS1-ABC123",
  "challenge_id": "ch_789",
  "signature": "MEUCIQD2...",
  "firmware_version": "1.0.0"
}
```

**Response (200):**
```json
{
  "session_token": "sess_abc123...",
  "device_id": "dev_456",
  "customer_id": "cus_789",
  "expires_at": "2024-01-15T11:30:00Z",
  "status": "paired"
}
```

**Response (200) - Not Yet Paired:**
```json
{
  "session_token": "sess_abc123...",
  "device_id": "dev_456",
  "customer_id": null,
  "expires_at": "2024-01-15T11:30:00Z",
  "status": "awaiting_pairing"
}
```

**Response (401):**
```json
{
  "error": "invalid_signature",
  "message": "Signature verification failed"
}
```

**Response (401):**
```json
{
  "error": "challenge_expired",
  "message": "Challenge has expired, request a new one"
}
```

### POST /auth/refresh

Refresh an active session without full re-authentication. Device must sign the current session token.

**Headers:**
```
Authorization: Bearer {session_token}
```

**Request:**
```json
{
  "signature": "MEUCIQD2..."
}
```

**Response (200):**
```json
{
  "session_token": "sess_def456...",
  "expires_at": "2024-01-15T12:30:00Z"
}
```

### DELETE /devices/{deviceId}

Deregister device from cloud. Device remains functional but loses cloud connectivity until re-paired.

**Headers:**
```
Authorization: Bearer {session_token}
```

**Response (200):**
```json
{
  "success": true,
  "message": "Device deregistered, cryptographic identity retained"
}
```

### GET /devices/{deviceId}/ota

Check for firmware updates. Returns signed update manifest.

**Headers:**
```
Authorization: Bearer {session_token}
```

**Query Parameters:**
- `current_version` - Current firmware version (e.g., "1.0.0")
- `slot` - Current boot slot ("a" or "b")

**Response (200) - Update Available:**
```json
{
  "update_available": true,
  "version": "1.1.0",
  "images": {
    "rootfs": {
      "url": "https://r2.optic.works/firmware/rs1/1.1.0/rootfs.squashfs",
      "sha256": "abc123...",
      "size_bytes": 15728640
    },
    "kernel": {
      "url": "https://r2.optic.works/firmware/rs1/1.1.0/boot.fit",
      "sha256": "def456...",
      "size_bytes": 8388608
    }
  },
  "verity": {
    "root_hash": "b96a69664f9279857931dbf64f942caf909076e40fd5bd5ed8d30b53ff922941",
    "salt": "2a4c7638f03b92bdb92d7284a742e0c4407c9ef65fdf2a7ea78ed02fde4a518b",
    "hash_algorithm": "sha256",
    "data_block_size": 4096,
    "hash_block_size": 4096
  },
  "manifest_signature": "MEUCIQDx...",
  "release_notes": "Bug fixes and performance improvements",
  "mandatory": false,
  "min_version": "0.9.0",
  "anti_rollback_index": 3
}
```

**Response (200) - No Update:**
```json
{
  "update_available": false,
  "current_version": "1.1.0"
}
```

### POST /devices/{deviceId}/ota/status

Report OTA update progress and completion.

**Headers:**
```
Authorization: Bearer {session_token}
```

**Request:**
```json
{
  "version": "1.1.0",
  "status": "completed",
  "from_version": "1.0.0",
  "slot": "b",
  "verity_verified": true,
  "boot_successful": true
}
```

**Status Values:**
- `downloading` - Download in progress
- `verifying` - Verifying signatures and hashes
- `applying` - Writing to inactive slot
- `pending_reboot` - Written successfully, awaiting reboot
- `completed` - Booted successfully into new version
- `failed` - Update failed (include `error_code` and `error_message`)
- `rolled_back` - Rolled back to previous slot after boot failure

**Response (200):**
```json
{
  "received": true
}
```

### POST /devices/{deviceId}/telemetry

Submit device telemetry data.

**Headers:**
```
Authorization: Bearer {session_token}
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
    "video_bitrate_kbps": 2500,
    "boot_slot": "a",
    "verity_errors": 0
  },
  "events": [
    {
      "type": "boot",
      "timestamp": "2024-01-14T10:30:00Z",
      "data": { "slot": "a", "reason": "power_on" }
    },
    {
      "type": "ota_completed",
      "timestamp": "2024-01-14T10:35:00Z",
      "data": { "from": "1.0.0", "to": "1.1.0", "slot": "b" }
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
Authorization: Bearer {session_token}
X-Device-ID: {serial_number}
X-Firmware-Version: {firmware_version}
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

### Device Identity Model

Devices authenticate using ECDSA P-256 signatures. The private key is generated and stored in the RV1106's TrustZone secure world and never leaves the device.

**Key Generation (Manufacturing):**
1. Device boots into provisioning mode
2. TrustZone generates ECDSA P-256 keypair
3. Private key stored in secure storage (OTP-derived encryption)
4. Public key exported to provisioning station
5. Provisioning station registers `serial_number ↔ public_key` mapping

**Runtime Authentication:**
1. Device requests challenge from cloud
2. Cloud generates 32-byte random challenge, stores with TTL
3. Device passes challenge to TrustZone for signing
4. TrustZone signs: `signature = ECDSA_Sign(SHA256(challenge || serial_number || timestamp), private_key)`
5. Device sends signature to cloud
6. Cloud verifies against registered public key
7. Cloud issues short-lived session token (1 hour default)

### Session Tokens

Session tokens are JWTs signed by the cloud service:

```json
{
  "sub": "dev_456",
  "serial": "RS1-ABC123",
  "customer_id": "cus_789",
  "iat": 1705312200,
  "exp": 1705315800,
  "scope": ["telemetry", "ota", "signaling"]
}
```

Session tokens:
- Are short-lived (1 hour default, configurable)
- Can be refreshed with a signed request
- Are validated on every API call
- Do not grant any privilege if extracted (attacker cannot sign refresh requests)

### Signature Format

All device signatures use ECDSA with P-256 curve and SHA-256 hash:

```
Algorithm: ES256 (ECDSA using P-256 and SHA-256)
Signature encoding: Base64URL-encoded DER format
```

**Challenge Signature Construction:**
```
message = SHA256(challenge_bytes || serial_number || timestamp_iso8601)
signature = ECDSA_Sign(message, device_private_key)
```

### Token Validation

The Go service validates session tokens and optionally verifies device signatures:

```sql
-- Validate session token (JWT verification + DB check)
SELECT id, serial_number, customer_id, public_key, firmware_version
FROM device
WHERE id = $1
  AND revoked_at IS NULL

-- Verify device signature
-- Done in application code using stored public_key
```

## Manufacturing Provisioning

### Provisioning API

These endpoints are only accessible from the manufacturing network.

#### POST /manufacturing/provision

Register a new device from the manufacturing line.

**Headers:**
```
Authorization: Bearer {manufacturing_api_key}
X-Station-ID: {provisioning_station_id}
```

**Request:**
```json
{
  "serial_number": "RS1-ABC123",
  "public_key": "MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE...",
  "product_type": "rs1",
  "hardware_revision": "1.2",
  "manufacturing_lot": "2024-01-15-A",
  "factory_firmware_version": "1.0.0"
}
```

**Response (200):**
```json
{
  "device_id": "dev_456",
  "serial_number": "RS1-ABC123",
  "provisioned_at": "2024-01-15T10:30:00Z"
}
```

**Response (409):**
```json
{
  "error": "already_provisioned",
  "message": "Device serial number already registered",
  "device_id": "dev_456"
}
```

#### GET /manufacturing/devices/{serialNumber}

Look up device provisioning status.

**Headers:**
```
Authorization: Bearer {manufacturing_api_key}
```

**Response (200):**
```json
{
  "device_id": "dev_456",
  "serial_number": "RS1-ABC123",
  "public_key": "MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE...",
  "product_type": "rs1",
  "provisioned_at": "2024-01-15T10:30:00Z",
  "first_seen_at": "2024-01-16T14:20:00Z",
  "customer_id": "cus_789",
  "paired_at": "2024-01-16T14:25:00Z"
}
```

## Database Schema

The Go service reads from and writes to the same PostgreSQL database as Medusa:

### Device Table (Medusa-managed with Go extensions)

```sql
CREATE TABLE device (
  id VARCHAR PRIMARY KEY,
  serial_number VARCHAR UNIQUE NOT NULL,
  
  -- Cryptographic identity (set at manufacturing, immutable)
  public_key TEXT NOT NULL,
  public_key_algorithm VARCHAR DEFAULT 'ES256',
  
  -- Device metadata
  product_type VARCHAR DEFAULT 'rs1',
  hardware_revision VARCHAR,
  manufacturing_lot VARCHAR,
  
  -- Firmware tracking
  firmware_version VARCHAR,
  boot_slot VARCHAR(1),
  anti_rollback_index INTEGER DEFAULT 0,
  
  -- Customer association (mutable)
  customer_id VARCHAR,
  order_id VARCHAR,
  
  -- Lifecycle timestamps
  provisioned_at TIMESTAMP NOT NULL,
  first_seen_at TIMESTAMP,
  paired_at TIMESTAMP,
  last_seen_at TIMESTAMP,
  revoked_at TIMESTAMP,
  
  -- Metadata
  metadata JSONB,
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_device_serial ON device(serial_number);
CREATE INDEX idx_device_customer ON device(customer_id);
```

### Challenge Table (Go-managed)

```sql
CREATE TABLE auth_challenge (
  id VARCHAR PRIMARY KEY,
  device_id VARCHAR NOT NULL REFERENCES device(id),
  challenge BYTEA NOT NULL,
  expires_at TIMESTAMP NOT NULL,
  used_at TIMESTAMP,
  created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_challenge_device ON auth_challenge(device_id);
CREATE INDEX idx_challenge_expires ON auth_challenge(expires_at);

-- Cleanup expired challenges
CREATE INDEX idx_challenge_cleanup ON auth_challenge(expires_at) WHERE used_at IS NULL;
```

### Session Table (Go-managed)

```sql
CREATE TABLE device_session (
  id VARCHAR PRIMARY KEY,
  device_id VARCHAR NOT NULL REFERENCES device(id),
  token_hash VARCHAR NOT NULL,
  expires_at TIMESTAMP NOT NULL,
  created_at TIMESTAMP DEFAULT NOW(),
  revoked_at TIMESTAMP
);

CREATE INDEX idx_session_device ON device_session(device_id);
CREATE INDEX idx_session_token ON device_session(token_hash);
CREATE INDEX idx_session_expires ON device_session(expires_at);
```

### Telemetry Table (Go-managed)

```sql
CREATE TABLE device_telemetry (
  id SERIAL PRIMARY KEY,
  device_id VARCHAR NOT NULL REFERENCES device(id),
  timestamp TIMESTAMP NOT NULL,
  metrics JSONB NOT NULL,
  events JSONB,
  created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_telemetry_device ON device_telemetry(device_id);
CREATE INDEX idx_telemetry_timestamp ON device_telemetry(timestamp);

-- Partition by month for efficient cleanup
-- (implementation depends on PostgreSQL version)
```

### OTA Updates Table (Go-managed)

```sql
CREATE TABLE device_ota_updates (
  id SERIAL PRIMARY KEY,
  device_id VARCHAR NOT NULL REFERENCES device(id),
  from_version VARCHAR NOT NULL,
  to_version VARCHAR NOT NULL,
  target_slot VARCHAR(1) NOT NULL,
  
  -- Progress tracking
  started_at TIMESTAMP NOT NULL,
  completed_at TIMESTAMP,
  status VARCHAR DEFAULT 'pending',
  
  -- Verification
  verity_verified BOOLEAN,
  boot_successful BOOLEAN,
  rolled_back BOOLEAN DEFAULT FALSE,
  
  -- Error tracking
  error_code VARCHAR,
  error_message VARCHAR,
  
  created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_ota_device ON device_ota_updates(device_id);
CREATE INDEX idx_ota_status ON device_ota_updates(status);
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

  auth/
    challenge.go         # Challenge generation/validation
    signature.go         # ECDSA signature verification
    session.go           # Session token management
    
  db/
    device.go            # Device queries
    challenge.go         # Challenge storage
    session.go           # Session storage
    telemetry.go         # Telemetry writes
    ota.go               # OTA update tracking
    migrations/          # Go-specific migrations

  ota/
    releases.go          # Release management
    manifest.go          # Manifest signing
    r2.go                # R2 storage client

  signaling/
    hub.go               # WebSocket connection hub
    session.go           # WebRTC session management

  manufacturing/
    provision.go         # Device provisioning
    handlers.go          # Manufacturing API handlers

config/
  config.go              # Environment configuration
```

## Configuration

```bash
# Required
DATABASE_URL=postgres://user:pass@host:5432/medusa
SESSION_SIGNING_KEY=...          # For JWT session tokens
MANIFEST_SIGNING_KEY=...         # For OTA manifest signatures (private key)

# Manufacturing API
MANUFACTURING_API_KEY=...        # API key for provisioning stations
MANUFACTURING_NETWORK_CIDR=10.0.0.0/8  # Restrict manufacturing endpoints

# Optional
PORT=8080
SESSION_TTL=3600                 # Session token TTL in seconds (default 1 hour)
CHALLENGE_TTL=60                 # Challenge TTL in seconds (default 1 minute)

# R2 Storage
R2_ENDPOINT=https://xxx.r2.cloudflarestorage.com
R2_ACCESS_KEY_ID=...
R2_SECRET_ACCESS_KEY=...
R2_BUCKET=opticworks-firmware

# Monitoring
SENTRY_DSN=...
```

## Deployment

The Go service runs alongside Medusa on the same Hetzner node:

```nginx
# Nginx configuration
location /auth/ {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
}

location /devices/ {
    proxy_pass http://127.0.0.1:8080;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
}

# Manufacturing API - restricted to internal network
location /manufacturing/ {
    allow 10.0.0.0/8;
    deny all;
    proxy_pass http://127.0.0.1:8080;
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
    "PORT": "8080",
    "SESSION_SIGNING_KEY": "...",
    "MANIFEST_SIGNING_KEY": "..."
  }
}
```

## OTA Update Security

### Firmware Signing

All firmware images are signed using ECDSA P-256:

```bash
# Build process generates signed manifest
{
  "version": "1.1.0",
  "images": { ... },
  "verity": { ... },
  "anti_rollback_index": 3,
  "signed_at": "2024-01-15T10:00:00Z"
}

# Manifest is signed
manifest_signature = ECDSA_Sign(SHA256(manifest_json), release_signing_key)
```

### Device Verification Flow

```
1. Device fetches OTA manifest from cloud
2. Device verifies manifest_signature against embedded OpticWorks public key
   (This key's hash is in eFuse, verified during secure boot)
3. Device downloads images to inactive slot
4. Device verifies SHA256 of downloaded images matches manifest
5. Device writes dm-verity hash tree
6. Device updates U-Boot to boot from new slot with verity parameters
7. Device reboots
8. Secure boot verifies SPL → U-Boot → kernel chain
9. Kernel verifies rootfs via dm-verity using root_hash from FIT image
10. If boot fails, watchdog triggers rollback to previous slot
11. Device reports OTA status to cloud
```

### Anti-Rollback Protection

```
- Each firmware version has an anti_rollback_index
- Device stores current index in eFuse (monotonic counter)
- Device rejects updates with index < current
- Index is only incremented after successful boot verification
```

## Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `device_not_found` | 404 | Serial number not registered |
| `invalid_signature` | 401 | Device signature verification failed |
| `challenge_expired` | 401 | Challenge TTL exceeded |
| `session_expired` | 401 | Session token expired |
| `session_revoked` | 401 | Session has been revoked |
| `not_paired` | 412 | Device not paired to customer |
| `unauthorized` | 401 | Missing or invalid authorization |
| `anti_rollback` | 400 | Firmware version older than current anti-rollback index |
| `signature_required` | 400 | Request requires device signature |
| `session_failed` | 500 | WebRTC session failed |

## Rate Limits

| Endpoint | Limit | Notes |
|----------|-------|-------|
| POST /auth/challenge | 10/min per serial | Prevents challenge flooding |
| POST /auth/verify | 5/min per serial | Prevents signature brute force |
| POST /auth/refresh | 60/min per session | Normal refresh rate |
| POST /devices/telemetry | 60/min per device | |
| GET /devices/{id}/ota | 10/min per device | |
| WebSocket connection | 1 per device | Enforced by session |
| POST /manufacturing/* | 100/min per station | Manufacturing line rate |

## Appendix: Cryptographic Details

### ECDSA P-256 Key Format

**Public Key (stored in database):**
```
Format: Base64-encoded SubjectPublicKeyInfo (SPKI) DER
Example: MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE...
```

**Signature Format:**
```
Format: Base64URL-encoded DER (ECDSA-Sig-Value)
ASN.1: SEQUENCE { r INTEGER, s INTEGER }
```

### Session Token (JWT)

```
Algorithm: ES256 (ECDSA P-256 + SHA-256)
Header: { "alg": "ES256", "typ": "JWT" }
Payload: { "sub": "dev_456", "serial": "RS1-ABC123", ... }
```

### OTA Manifest Signature

```
Algorithm: ES256
Message: SHA256(canonical_json(manifest))
Signature: Base64-encoded DER
```

Public key for manifest verification is embedded in device firmware and its hash is verified during secure boot via eFuse root of trust.

---

## Implementation Discussion

> **Status**: Planning / RFC
> **Last Updated**: 2024-12-20

### Current State Analysis

The existing codebase (`hardwareos`) uses CLOUD_API v1 (see `docs/CLOUD_API.md`), which has the following characteristics:

| Feature | v1 (Current) | v2 (This Spec) |
|---------|--------------|----------------|
| Authentication | Bearer token (plaintext in config) | Challenge-response with ECDSA P-256 |
| Key Storage | N/A | TrustZone secure world |
| Token Type | Permanent `cloud_token` | Short-lived JWT sessions |
| OTA Verification | SHA256 hash only | Signature + dm-verity + anti-rollback |
| Manufacturing | None | Provisioning API + public key registration |
| Device Identity | Serial from `/proc/cpuinfo` | Keypair bound to eFuse |

**Key files in current implementation:**
- `cloud.go:321-323` - Bearer token auth: `header.Set("Authorization", "Bearer "+config.CloudToken)`
- `config.go:28` - Token stored in plaintext: `CloudToken string`
- `platform/ota/utils.go:132-193` - SHA256-only verification, no signature checks

### Open Questions

#### 1. RV1106 TrustZone / OP-TEE Availability

Is there a Rockchip-provided OP-TEE implementation or secure world SDK for the RV1106? Key generation in TrustZone is the foundation of the hardware-bound identity model.

**Options if OP-TEE unavailable:**
- Use software keys with secure storage (less secure but functional)
- Investigate alternative TEE implementations
- Use hardware security module (HSM) at manufacturing only

#### 2. Manufacturing & Provisioning

Who handles device manufacturing? The provisioning flow requires:
- Factory firmware with provisioning mode
- Provisioning station software
- Secure network for `POST /manufacturing/provision`
- Serial ↔ public key database population

**Sub-questions:**
- In-house or contract manufactured?
- What's the production volume expectation?
- Is there existing factory tooling to integrate with?

#### 3. Legacy Device Migration

If devices are already deployed on v1:
- How do we migrate them to v2?
- Can software keys be generated post-deployment?
- Do we need to maintain v1 compatibility indefinitely?

#### 4. Cross-Repository Coordination

This spec touches multiple repositories:

| Repository | Components Affected |
|------------|---------------------|
| `hardwareos` | Device-side auth, OTA client, TrustZone integration |
| `opticworks-store` | Medusa device module, customer pairing API |
| Go Device Mgmt Service | Challenge/session endpoints, WebSocket signaling |
| Build system | dm-verity rootfs, signed manifests, anti-rollback |

**Questions:**
- Where does the Go Device Mgmt Service live? (New repo? Monorepo with store?)
- How do we coordinate schema changes across Medusa and Go service?
- Who owns the release signing keys?

#### 5. Timeline & Phasing

What's the timeline pressure? A phased approach may be more practical:

**Phase 1 - v1.5 (Immediate security improvements)**
- Challenge-response auth with software-generated keys
- Signed OTA manifests (signature verification on device)
- Short-lived session tokens (no persistent cloud_token)
- No TrustZone dependency

**Phase 2 - v2 (Full hardware security)**
- TrustZone key generation at manufacturing
- dm-verity rootfs verification
- eFuse anti-rollback counter
- Manufacturing provisioning integration

### Feasibility Assessment

| Component | Feasibility | Notes |
|-----------|-------------|-------|
| Challenge-response auth | **High** | ECDSA P-256 already used in TLS code |
| Session tokens (JWT) | **High** | Standard library support |
| Signed OTA manifests | **High** | Add signature field to existing metadata |
| TrustZone key storage | **Medium** | Requires RV1106 OP-TEE investigation |
| dm-verity integration | **Medium** | Build system changes for squashfs + hash tree |
| eFuse anti-rollback | **Medium** | Needs kernel driver or userspace tool |
| Manufacturing provisioning | **Medium** | Depends on factory setup |

### Recommended Next Steps

1. **Investigate RV1106 OP-TEE** - Contact Rockchip or check SDK for TrustZone support
2. **Create CLOUD_API_v1.5 spec** - Intermediate milestone without hardware dependencies
3. **Prototype challenge-response** - Implement in Go service, test with software keys
4. **Design cross-repo coordination** - Define which components live where
5. **Document manufacturing requirements** - Work with production team on provisioning flow

### Related Documents

- `docs/CLOUD_API.md` - Current v1 specification
- `opticworks-store/docs/reference/DEVICE_MANAGEMENT.md` - Medusa device module
- `docs/rs1/RS1_ARCHITECTURE.md` - Overall RS-1 system architecture

---

*Discussion added 2024-12-20. Please add comments/responses below or in GitHub issues.*
