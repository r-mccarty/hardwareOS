# Secrets and Credentials

This document catalogs all secrets, credentials, and sensitive configuration for JetKVM. Use this for setting up your secrets vault (e.g., Infisical).

## Vault Structure Recommendation

```
jetkvm/
├── device/
│   ├── CLOUD_TOKEN              # Cloud API bearer token
│   ├── GOOGLE_IDENTITY          # Verified Google OIDC identity
│   ├── LOCAL_AUTH_TOKEN         # Session authentication token
│   └── HASHED_PASSWORD          # bcrypt-hashed device password
├── tls/
│   ├── PRIVATE_KEY              # TLS private key (PEM)
│   └── CERTIFICATE              # TLS certificate (PEM)
├── ci-cd/
│   ├── WIREGUARD_PRIVATE_KEY    # CI WireGuard private key
│   ├── WIREGUARD_PUBLIC_KEY     # CI WireGuard public key
│   ├── WIREGUARD_ENDPOINT       # CI WireGuard endpoint
│   └── SSH_PRIVATE_KEY          # CI SSH private key
└── cloud-storage/
    └── RCLONE_CONFIG            # R2/S3 credentials for releases
```

---

## Device Secrets

These are stored on-device in `/userdata/kvm_config.json`:

| Secret | Field | Type | Purpose |
|--------|-------|------|---------|
| Cloud Token | `cloudToken` | String | Bearer token for cloud API authentication |
| Google Identity | `googleIdentity` | String | Verified OIDC identity (audience:subject) |
| Local Auth Token | `localAuthToken` | UUID | Session token for local web authentication |
| Hashed Password | `hashedPassword` | String | bcrypt hash of device password |

### Cloud Token
- **Used in**: `cloud.go` (Authorization header)
- **Format**: Bearer token string
- **Lifetime**: Managed by cloud API
- **Storage**: Plain text in config JSON

### Local Auth Token
- **Used in**: `web.go` (cookie-based auth)
- **Format**: UUID v4
- **Lifetime**: 7 days (cookie expiry)
- **Storage**: Config JSON + HTTP cookie (`authToken`)

### Hashed Password
- **Used in**: `web.go` (login validation)
- **Format**: bcrypt hash (DefaultCost)
- **Storage**: Config JSON only

---

## TLS Certificates

Stored in `/userdata/jetkvm/tls/`:

| File | Purpose | Format |
|------|---------|--------|
| `*.crt` | TLS certificate | PEM-encoded X.509 |
| `*.key` | TLS private key | PEM-encoded |

### Configuration
- **Config field**: `tlsMode`
- **Options**: `"self-signed"`, `"user-defined"`, `""` (disabled)
- **CA Name**: `jetkvm.local` (for self-signed)

---

## SSH Keys

| Path | Purpose |
|------|---------|
| `/userdata/dropbear/.ssh/authorized_keys` | Authorized public keys for SSH access |

### Supported Key Types
- `ssh-rsa`
- `ssh-ed25519`
- `ecdsa-sha2-nistp256/384/521`
- `sk-ssh-ed25519@openssh.com` (security keys)
- `sk-ecdsa-sha2-nistp256@openssh.com` (security keys)

---

## CI/CD Secrets (GitHub Actions)

Required in GitHub repository secrets for the smoketest workflow:

| Secret Name | Purpose | Used In |
|-------------|---------|---------|
| `JETKVM_CI_WG_PRIVATE` | WireGuard private key | VPN tunnel to CI device |
| `JETKVM_CI_WG_PUBLIC` | WireGuard public key | VPN tunnel to CI device |
| `JETKVM_CI_WG_ENDPOINT` | WireGuard server endpoint | VPN tunnel to CI device |
| `JETKVM_CI_SSH_PRIVATE` | SSH private key | Device access for tests |

### CI/CD Variables (Non-Secret)

| Variable Name | Purpose |
|---------------|---------|
| `JETKVM_CI_HOST` | CI device hostname/IP |
| `JETKVM_CI_USER` | SSH username |
| `JETKVM_CI_WG_IPS` | WireGuard IP addresses |
| `JETKVM_CI_GATEWAY` | WireGuard gateway |
| `JETKVM_CI_WG_ALLOWED_IPS` | WireGuard allowed IPs |

---

## Cloud Storage (R2/Cloudflare)

Used for release artifact uploads via `rclone`:

| Bucket | Path Pattern | Purpose |
|--------|--------------|---------|
| `jetkvm-update` | `/app/{VERSION}/` | Application binaries |
| `jetkvm-update` | `/system/{VERSION}/` | System images |
| `jetkvm-cloud-app` | `/` | Cloud UI static assets |

### Rclone Configuration

Credentials stored in `~/.config/rclone/rclone.conf`:

```ini
[r2]
type = s3
provider = Cloudflare
access_key_id = <R2_ACCESS_KEY_ID>
secret_access_key = <R2_SECRET_ACCESS_KEY>
endpoint = https://<ACCOUNT_ID>.r2.cloudflarestorage.com
```

| Secret | Purpose |
|--------|---------|
| `R2_ACCESS_KEY_ID` | Cloudflare R2 access key |
| `R2_SECRET_ACCESS_KEY` | Cloudflare R2 secret key |
| `R2_ACCOUNT_ID` | Cloudflare account ID |

---

## Frontend Environment Variables

Build-time variables (not secrets, but environment-specific):

| Variable | Environment | Value |
|----------|-------------|-------|
| `VITE_CLOUD_API` | Development | `http://localhost:3000` |
| `VITE_CLOUD_API` | Staging | `https://staging-api.jetkvm.com` |
| `VITE_CLOUD_API` | Production | `https://api.jetkvm.com` |

---

## OAuth/OIDC (Runtime Only)

These are not stored secrets but runtime-verified tokens:

| Item | Source | Storage |
|------|--------|---------|
| Google OIDC Token | User browser | Verified once, not stored |
| OIDC Client ID | Cloud API response | Runtime only |

**Provider**: `https://accounts.google.com` (hardcoded)

---

## API Endpoints (Hardcoded)

Not secrets, but important configuration:

| Endpoint | Purpose |
|----------|---------|
| `https://api.jetkvm.com` | Production cloud API |
| `https://staging-api.jetkvm.com` | Staging cloud API |
| `https://app.jetkvm.com` | Cloud web app |
| `https://accounts.google.com` | Google OIDC provider |
| `time.cloudflare.com` | NTP server |
| `time.aws.com` | NTP server (fallback) |
| `https://cp.cloudflare.com/` | HTTP time sync |

---

## Infisical Setup

### Environment Mapping

| Infisical Environment | Purpose |
|-----------------------|---------|
| `development` | Local development |
| `staging` | Staging/test deployments |
| `production` | Production devices |
| `ci-cd` | GitHub Actions workflows |

### Secret References

For device configuration, inject these at runtime:

```bash
# Device secrets
JETKVM_CLOUD_TOKEN=<from vault>
JETKVM_LOCAL_AUTH_TOKEN=<from vault>

# TLS (if using vault-managed certs)
JETKVM_TLS_CERT=<from vault>
JETKVM_TLS_KEY=<from vault>
```

For CI/CD, configure GitHub Actions secrets to pull from Infisical:

```yaml
# .github/workflows/smoketest.yml
env:
  JETKVM_CI_WG_PRIVATE: ${{ secrets.JETKVM_CI_WG_PRIVATE }}
  JETKVM_CI_SSH_PRIVATE: ${{ secrets.JETKVM_CI_SSH_PRIVATE }}
```

---

## Security Recommendations

### High Priority

1. **Encrypt config file at rest**
   - `/userdata/kvm_config.json` contains cloud token and auth tokens
   - Consider LUKS encryption for `/userdata` partition

2. **Protect TLS private keys**
   - Set restrictive permissions: `chmod 600 /userdata/jetkvm/tls/*.key`
   - Consider encrypting keys at rest

3. **Rotate tokens periodically**
   - Cloud token: Per cloud API lifecycle
   - Local auth token: On password change or manual rotation

### Medium Priority

4. **SSH key management**
   - Audit authorized_keys periodically
   - Use Ed25519 keys (recommended over RSA)

5. **CI/CD secrets**
   - Rotate WireGuard keys periodically
   - Use short-lived SSH keys if possible

### Audit Checklist

- [ ] Cloud token not exposed in logs
- [ ] TLS private keys have restricted permissions
- [ ] SSH authorized_keys contains only expected keys
- [ ] CI/CD secrets rotated within policy period
- [ ] R2 credentials have minimal required permissions

---

## Files Containing Secrets

| File Path | Secrets Present |
|-----------|-----------------|
| `/userdata/kvm_config.json` | cloudToken, googleIdentity, localAuthToken, hashedPassword |
| `/userdata/jetkvm/tls/*.key` | TLS private keys |
| `/userdata/dropbear/.ssh/authorized_keys` | SSH public keys |
| `~/.config/rclone/rclone.conf` | R2 access credentials |
