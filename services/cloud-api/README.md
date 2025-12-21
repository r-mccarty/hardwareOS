# OpticWorks Cloud API

A Cloudflare Workers-based cloud backend for OpticWorks/HardwareOS devices. Provides device registration, authentication, OTA updates, and telemetry collection.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     Cloud API (This Service)                     │
│                    Cloudflare Workers + D1                       │
├─────────────────────────────────────────────────────────────────┤
│  POST /devices/register      │  Device registration             │
│  POST /devices/token         │  Token exchange after pairing    │
│  DELETE /devices/:id         │  Deregister device               │
│  GET /devices/:id/ota        │  Check for OTA updates           │
│  POST /devices/:id/telemetry │  Submit device telemetry         │
├─────────────────────────────────────────────────────────────────┤
│  GET /me                     │  Get current user + devices      │
│  POST /auth/login            │  User login                      │
│  POST /auth/logout           │  User logout                     │
│  POST /auth/signup           │  Create new user                 │
└─────────────────────────────────────────────────────────────────┘
```

## Quick Start

### Prerequisites

- Node.js 18+
- Cloudflare account
- Wrangler CLI (`npm install -g wrangler`)

### Development Setup

```bash
cd services/cloud-api

# Install dependencies
npm install

# Login to Cloudflare (first time only)
npx wrangler login

# Create the D1 database (first time only)
npm run db:create

# Initialize local database with schema
npm run db:init

# Start development server
npm run dev
```

The API will be available at `http://localhost:3000`.

### Database Setup

After creating the D1 database with `npm run db:create`, update `wrangler.toml` with the database ID:

```toml
[[d1_databases]]
binding = "DB"
database_name = "opticworks-devices"
database_id = "YOUR_DATABASE_ID_HERE"
```

## API Endpoints

### Health Check

```bash
# Check API health
curl http://localhost:3000/health
# {"status":"ok"}
```

### Device Registration Flow

```bash
# 1. Device registers at first boot
curl -X POST http://localhost:3000/devices/register \
  -H "Content-Type: application/json" \
  -d '{"serial_number": "RS1-TEST001", "firmware_version": "1.0.0"}'
# {"temp_token":"abc123...","device_id":"dev_xxx","status":"awaiting_pairing"}

# 2. User pairs device (via mobile app or web UI)
curl -X POST http://localhost:3000/devices/dev_xxx/pair \
  -H "Content-Type: application/json" \
  -d '{"user_id": "usr_demo"}'

# 3. Device exchanges temp token for permanent token
curl -X POST http://localhost:3000/devices/token \
  -H "Content-Type: application/json" \
  -d '{"temp_token": "abc123..."}'
# {"cloud_token":"def456...","device_id":"dev_xxx","customer_id":"usr_demo"}
```

### User Authentication

```bash
# Login (creates session cookie)
curl -X POST http://localhost:3000/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "demo@optic.works"}' \
  -c cookies.txt

# Get current user info
curl http://localhost:3000/me -b cookies.txt
# {"id":"usr_demo","email":"demo@optic.works","devices":[...]}

# Logout
curl -X POST http://localhost:3000/auth/logout -b cookies.txt
```

### Device API (requires device token)

```bash
TOKEN="your_cloud_token_here"

# Check for OTA updates
curl "http://localhost:3000/devices/dev_xxx/ota?current_version=1.0.0" \
  -H "Authorization: Bearer $TOKEN"

# Submit telemetry
curl -X POST http://localhost:3000/devices/dev_xxx/telemetry \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "timestamp": "2024-01-15T10:30:00Z",
    "metrics": {
      "uptime_seconds": 86400,
      "memory_free_mb": 128,
      "cpu_temp_c": 45.2
    }
  }'
```

## Development

### Project Structure

```
services/cloud-api/
├── src/
│   ├── index.ts          # Main entry point, route mounting
│   ├── types.ts          # TypeScript type definitions
│   ├── utils.ts          # Utility functions
│   ├── routes/
│   │   ├── devices.ts    # Device registration/management
│   │   ├── auth.ts       # User authentication
│   │   ├── telemetry.ts  # Device telemetry
│   │   └── ota.ts        # OTA update endpoints
│   └── db/
│       └── schema.sql    # D1 database schema
├── wrangler.toml         # Cloudflare Worker config
├── tsconfig.json         # TypeScript config
└── package.json
```

### Available Scripts

```bash
npm run dev              # Start local dev server
npm run deploy           # Deploy to Cloudflare Workers
npm run deploy:staging   # Deploy to staging environment
npm run deploy:production # Deploy to production
npm run db:init          # Initialize local D1 database
npm run db:init:remote   # Initialize remote D1 database
npm run typecheck        # Run TypeScript type checking
npm run tail             # Stream live logs from deployed Worker
```

### Testing with Frontend

1. Start the cloud API:
   ```bash
   cd services/cloud-api
   npm run dev
   ```

2. Start the frontend in cloud mode:
   ```bash
   cd ui
   npm run dev:cloud
   ```

3. The frontend will connect to `http://localhost:3000` (configured in `.env.cloud-development`)

## Deployment

### First-time Setup

1. Create D1 database:
   ```bash
   npm run db:create
   ```

2. Copy the database ID from the output and update `wrangler.toml`

3. Initialize the remote database:
   ```bash
   npm run db:init:remote
   ```

4. Deploy:
   ```bash
   npm run deploy
   ```

### Environment Variables

Set secrets using Wrangler:

```bash
# Optional: JWT signing key (for future use)
npx wrangler secret put JWT_SECRET
```

## API Reference

See [docs/CLOUD_API.md](../../docs/CLOUD_API.md) for the complete API specification.

## Security Notes

- Session tokens are stored as SHA-256 hashes in the database
- Cookies use `HttpOnly` and `SameSite=Lax` flags
- CORS is configured to allow localhost and production domains
- Device tokens are 64-byte hex strings

## Demo Data

The schema includes demo data for development:
- Demo user: `demo@optic.works` (id: `usr_demo`)
- Demo device: `RS1-DEMO001` (id: `dev_demo`, temp_token: `demo_temp_token_12345`)
