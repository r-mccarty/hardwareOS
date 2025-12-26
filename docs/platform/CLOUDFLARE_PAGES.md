# Cloudflare Pages Deployment

This document describes the Cloudflare Pages deployment for the HardwareOS UI demo.

## Overview

The RS-1 visualization UI is deployed to Cloudflare Pages for public demo access. This allows viewing the Tesla FSD-style 3D visualization without requiring a physical device.

## Deployed Instance

| Property | Value |
|----------|-------|
| **Project Name** | `hardwareos-demo` |
| **Production URL** | https://hardwareos-demo.pages.dev |
| **Demo Route** | https://hardwareos-demo.pages.dev/demo |
| **Cloudflare Account** | OpticWorks (39f8fd4a5b0c7558aed585facd57ec3b) |

## Demo Route

The `/demo` route renders the 3D visualization with simulated occupant data:

- **3 simulated occupants** with different tracking sources (fused, radar-only, vision-only)
- **Sample CapturedRoom** geometry (walls, doors, windows, furniture)
- **30Hz animation** loop with random movement patterns
- **Full visualization features** including motion trails, prediction cones, radar sweep

**Note:** The root route (`/`) will show an error because it attempts to connect to the device API. Always use `/demo` for the standalone demonstration.

## Deployment Process

### Prerequisites

1. Wrangler CLI authenticated with Cloudflare
2. Node.js 22.x
3. Build dependencies installed (`npm install` in `ui/` directory)

### Manual Deployment

From the `ui/` directory:

```bash
# Build the production bundle
npm run build:prod

# Deploy to Cloudflare Pages
npx wrangler pages deploy dist --project-name=hardwareos-demo --commit-dirty=true
```

### First-Time Setup

If the project doesn't exist:

```bash
# Create the Pages project
npx wrangler pages project create hardwareos-demo --production-branch main

# Deploy
npx wrangler pages deploy dist --project-name=hardwareos-demo
```

## Build Modes

| Mode | Command | Purpose |
|------|---------|---------|
| `build:device` | `npm run build:device` | Embedded device deployment |
| `build:staging` | `npm run build:staging` | Cloud staging environment |
| `build:prod` | `npm run build:prod` | Cloud production / Pages demo |

The `build:prod` mode configures:
- API endpoints for cloud backend
- Production optimizations (minification, tree-shaking)
- Source maps disabled

## Environment Configuration

The Pages deployment uses `cloud-production` mode which sets:

```typescript
// vite.config.ts mode configuration
{
  mode: 'cloud-production',
  define: {
    'import.meta.env.VITE_API_BASE': '"https://api.opticworks.com"',
    'import.meta.env.VITE_ENVIRONMENT': '"production"'
  }
}
```

## Deployment History

Deployments are tracked in the Cloudflare dashboard:
- https://dash.cloudflare.com → Pages → hardwareos-demo

Each deployment gets a unique URL (e.g., `6663d645.hardwareos-demo.pages.dev`) in addition to the production URL.

## Troubleshooting

### "Failed to fetch" Error

**Cause:** The app is trying to connect to the device/cloud API which isn't available.

**Solution:** Use the `/demo` route which bypasses API calls and uses simulated data.

### CORS Errors

**Cause:** The production build may reference `api.jetkvm.com` (legacy) or other external APIs.

**Solution:** The `/demo` route avoids external API calls. For full cloud functionality, ensure the cloud API has proper CORS headers.

### Build Failures

**Symlink issues:** The `public/sse.html` symlink must point to a valid file:
```bash
# Fix broken symlink
rm ui/public/sse.html
ln -s ../../platform/logging/sse.html ui/public/sse.html
```

## Related Services

| Service | URL | Purpose |
|---------|-----|---------|
| Cloud API | https://api.opticworks.com | Device registration, authentication |
| Cloudflare Workers | `opticworks-cloud-api` | Backend API (Cloudflare Workers) |
| Pages Demo | https://hardwareos-demo.pages.dev/demo | UI visualization demo |

## CI/CD Integration

Future: GitHub Actions workflow for automatic deployment on merge to `main`:

```yaml
# .github/workflows/deploy-pages.yml (planned)
name: Deploy to Cloudflare Pages
on:
  push:
    branches: [main]
    paths: ['ui/**']
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: '22'
      - run: cd ui && npm ci && npm run build:prod
      - uses: cloudflare/pages-action@v1
        with:
          apiToken: ${{ secrets.CLOUDFLARE_API_TOKEN }}
          accountId: ${{ secrets.CLOUDFLARE_ACCOUNT_ID }}
          projectName: hardwareos-demo
          directory: ui/dist
```
