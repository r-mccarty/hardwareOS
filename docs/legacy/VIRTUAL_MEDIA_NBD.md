# Virtual Media via NBD (Block Device Bridge)

JetKVM presents remote ISO/IMG files to the target computer as a USB mass‑storage device. The `block_device*.go` files implement the **Network Block Device (NBD)** bridge that makes this possible.

## Purpose
- Expose a **remote image** (HTTP URL) or **local file** as a block device.
- Let the **target machine** boot/install from that image as if it were a physical USB drive.
- Avoid kernel module development by using NBD from userspace.

## How It Works
### Storage-backed (local file)
1. The image is stored on the device (e.g., `/userdata/jetkvm/images/foo.iso`).
2. The USB gadget mass‑storage LUN points directly to that file.
3. The target machine reads it as a block device.

### HTTP-backed (remote URL)
1. `rpcMountWithHTTP` creates an HTTP range reader for the URL.
2. An in‑process **NBD server** exposes that reader as a block device.
3. An **NBD client** connects `/dev/nbd0` to that server.
4. The USB gadget LUN is set to `/dev/nbd0`, so the target sees a disk.

```mermaid
flowchart LR
    URL[HTTP ISO/IMG] --> NBDServer[NBD Server (userspace)]
    NBDServer --> NBDClient[/dev/nbd0 (kernel)]
    NBDClient --> USB[USB Gadget LUN]
    USB --> Target[Target Computer]
```

## Key Paths and Files
- NBD device: `/dev/nbd0`
- NBD socket: `/var/run/nbd.socket`
- USB gadget LUN path: `mass_storage_lun0` (via `internal/usbgadget`)
- Images folder: `/userdata/jetkvm/images`

## Why Not “Read /dev/nbd0 Directly”?
`/dev/nbd0` is a kernel endpoint that **requires** an attached NBD backend. The code starts a server and client to bind real data to that device. Without that handshake, it is just an empty block interface.

## Limitations and Notes
- HTTP-backed media is **read‑only** (`remoteImageBackend.WriteAt` is not supported).
- Only one virtual media source is mounted at a time (`currentVirtualMediaState`).
- This flow is for **virtual media to the target machine**, not JetKVM firmware updates (those use OTA).

## Relevant Code
- NBD implementation: `block_device.go`, `block_device_linux.go`
- Virtual media RPCs: `usb_mass_storage.go`
- USB gadget config: `internal/usbgadget/`
