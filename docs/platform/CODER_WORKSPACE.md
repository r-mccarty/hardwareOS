# Coder Workspace: optic-rs1-dev1

This document describes the Coder workspace instance used for RS-1 HIL (Hardware-in-the-Loop)
testing and development.

## Instance Overview

| Property | Value |
|----------|-------|
| **Hostname** | `optic-rs1-dev1` |
| **URL** | `coder.hardwareos.com` |
| **Template** | `opticworks-dev` |
| **Host** | Intel N100 Mini PC |
| **Location** | On-premises (connected to Luckfox dev hardware) |

## Hardware Specifications

| Component | Details |
|-----------|---------|
| **CPU** | Intel N100 (4 cores, x86_64) |
| **RAM** | 16 GB |
| **Storage** | 468 GB (overlay filesystem) |
| **Network** | WiFi (192.168.0.148) + Ethernet (enp1s0) |
| **USB** | Luckfox Pico Max connected via USB-C RNDIS |

## Operating System

| Property | Value |
|----------|-------|
| **OS** | Ubuntu 24.04.3 LTS (Noble Numbat) |
| **Kernel** | 6.14.0-35-generic |
| **Architecture** | x86_64 |

## Installed Development Tools

### Languages & Runtimes

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.23.4 | Backend development |
| Node.js | 22.21.0 | UI development |
| npm | 10.9.4 | Package management |

### Hardware Tools

| Tool | Version | Purpose |
|------|---------|---------|
| adb | 1.0.41 | Android Debug Bridge for Luckfox |
| rkdeveloptool | 1.0.0 | Rockchip flashing utility |
| picocom | 3.1 | Serial console |
| minicom | - | Serial console (alternative) |

### Pre-configured Access

| Service | Status | Details |
|---------|--------|---------|
| **GitHub CLI** | Authenticated | Account: `r-mccarty` |
| **Infisical Secrets** | Pre-loaded | `~/.env.secrets` at startup |
| **Docker** | Available | Installed, daemon running |

## Network Interfaces

| Interface | Purpose | IP Address |
|-----------|---------|------------|
| `wlo1` | WiFi (primary) | 192.168.0.148/24 |
| `enp1s0` | Ethernet (to Luckfox) | Configure: 192.168.1.1/24 |
| `enx5aef472011ac` | USB RNDIS (to Luckfox) | Configure: 172.32.0.1/24 |
| `docker0` | Docker bridge | 172.17.0.1/16 |

## Connected Hardware

### Luckfox Pico Max

| Property | Value |
|----------|-------|
| **SoC** | Rockchip RV1106G3 |
| **Connection** | USB-C (RNDIS + ADB) |
| **USB Interface** | `enx5aef472011ac` |
| **RNDIS IP** | 172.32.0.93 |
| **Ethernet IP** | 192.168.1.100 (when configured) |

#### Access Methods

```bash
# ADB (default, USB device mode)
sudo adb shell

# SSH over RNDIS
sudo ip addr add 172.32.0.1/24 dev enx5aef472011ac
ssh root@172.32.0.93  # password: luckfox

# SSH over Ethernet (when in USB host mode)
sudo ip addr add 192.168.1.1/24 dev enp1s0
ssh root@192.168.1.100
```

## Workspace Configuration

### Container Settings

| Setting | Value |
|---------|-------|
| **Network Mode** | `host` |
| **Privileged** | `true` |
| **Devices** | Full `/dev` access |

### Mounted Paths

| Host Path | Container Path | Purpose |
|-----------|----------------|---------|
| `/var/run/docker.sock` | `/var/run/docker.sock` | Docker access |
| `/dev` | `/dev` | Hardware access |

## Repository Locations

| Repository | Path | Branch |
|------------|------|--------|
| hardwareOS | `/home/coder/workspace/hardwareOS` | `dev` |
| presence-detection-engine | `/home/coder/workspace/presence-detection-engine` | `main` |

## Common Workflows

### Start Development Session

```bash
# 1. Verify Luckfox connection
sudo adb devices

# 2. Set up RNDIS networking (if needed)
sudo ip addr add 172.32.0.1/24 dev enx5aef472011ac

# 3. Access Luckfox
sudo adb shell
# or
ssh root@172.32.0.93
```

### Build and Deploy (hardwareOS)

```bash
cd /home/coder/workspace/hardwareOS

# Build UI
cd ui && npm ci && npm run build:device && cd ..

# Note: Full ARM builds require Docker buildkit
# See docs/rs1/CAMERA_HIL_INTEGRATION.md for limitations
```

### USB Camera HIL Testing

See [CAMERA_HIL_INTEGRATION.md](../rs1/CAMERA_HIL_INTEGRATION.md) for complete setup including:
- USB host mode configuration
- Power via GPIO header
- Ethernet fallback connection

## Troubleshooting

### ADB Permission Denied

```bash
adb kill-server
sudo adb start-server
sudo adb devices
```

### RNDIS Interface Not Found

```bash
# Check USB connection
lsusb | grep Rockchip

# Reload driver
sudo modprobe -r rndis_host
sudo modprobe rndis_host

# Find interface name
ip link show | grep enx
```

### Docker Build Failures

Docker builds with BuildKit may fail due to nested overlay filesystem issues.
Workarounds:
1. Use GitHub Actions for CI builds
2. Run builds on host (outside container)
3. Deploy pre-built binaries via SCP

## Maintenance

### Restarting Docker Daemon

```bash
# Docker runs manually in this container
sudo dockerd &
```

### Updating Git Identity

```bash
git config --global user.email "r-mccarty@users.noreply.github.com"
git config --global user.name "r-mccarty"
```

### Checking Secrets

```bash
# View loaded Infisical secrets
cat ~/.env.secrets

# Secrets are loaded at workspace startup via Machine Identity token
```

## Related Documentation

- [CAMERA_HIL_INTEGRATION.md](../rs1/CAMERA_HIL_INTEGRATION.md) - HIL testing setup
- [DEVELOPMENT.md](DEVELOPMENT.md) - General development guide
- [SECRETS.md](SECRETS.md) - Secrets management
- [Luckfox Setup](../../presence-detection-engine/docs/luckfox-pico-max-setup.md) - Detailed Luckfox guide
