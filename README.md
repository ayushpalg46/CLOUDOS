# uniteOS — Local-First Personal Cloud OS

```
             _ _        ____   _____ 
  _   _ _ __(_) |_ ___ / __ \ / ____|
 | | | | '_ \ | __/ _ \ |  | | (___  
 | |_| | | | | ||  __/ |__| |\___ \ 
  \__,_|_| |_|_|\__\___|\____/|_____/ 
                                  v0.1.0
```

A privacy-first, decentralized, cross-device personal cloud operating system.
Zero cloud dependency. Offline-first. AI-powered. Fully local.

---

## Quick Start

### Prerequisites

- **Go 1.22+** — [Download Go](https://go.dev/dl/)
- **Windows / Linux / macOS** (any platform Go supports)

### 1. Build

```bash
# Clone or navigate to the project
cd personal_cloud_os

# Build the binary
go build -o uniteos.exe ./cmd/uniteos       # Windows
go build -o uniteos ./cmd/uniteos            # Linux/macOS
```

> **Windows Note:** If your organization has an Application Control policy that blocks
> new executables, build to your user profile directory instead:
> ```powershell
> go build -o "$env:USERPROFILE\uniteos.exe" .\cmd\uniteos
> ```

### 2. Initialize a Workspace

```bash
# Navigate to the folder you want uniteOS to manage
cd ~/Documents/my-workspace

# Initialize uniteOS
uniteos init
```

This creates a `.uniteos` directory with:
- SQLite database for metadata
- Content-addressable blob store
- Unique device identity (auto-generated)

### 3. Start Tracking Files

```bash
# Track everything in the current directory
uniteos add .

# Or track specific files/folders
uniteos add ./documents
uniteos add ./projects/my-app
```

### 4. Check Status

```bash
uniteos status
```

### 5. Open the Dashboard

```bash
uniteos serve
# Open http://localhost:7890 in your browser
```

---

## All Commands

### Core (Phase 1)

| Command | Description |
|---------|-------------|
| `uniteos init` | Initialize a new workspace |
| `uniteos add <path>` | Track a file or directory |
| `uniteos status` | Show file status (unchanged/modified/deleted) |
| `uniteos snapshot [name]` | Create a point-in-time backup |
| `uniteos history` | Show snapshot history |
| `uniteos rollback <snap-id>` | Restore a previous snapshot |
| `uniteos search <query>` | Search files by name |
| `uniteos encrypt <path>` | Encrypt a file (AES-256-GCM) |
| `uniteos decrypt <path>` | Decrypt a file |
| `uniteos config` | Show configuration |
| `uniteos info` | System information & stats |

### Multi-Device Sync (Phase 2)

| Command | Description |
|---------|-------------|
| `uniteos watch` | Watch for real-time file changes |
| `uniteos sync` | Start full sync daemon (watcher + discovery + P2P) |
| `uniteos peers` | Discover other uniteOS devices on your LAN |
| `uniteos conflicts` | View and resolve sync conflicts |

### Dashboard & Security (Phase 3)

| Command | Description |
|---------|-------------|
| `uniteos serve` | Start API server + web dashboard |
| `uniteos dashboard` | Alias for serve |
| `uniteos verify` | Verify integrity of all tracked files |
| `uniteos plugins` | List registered plugins |

### AI Intelligence (Phase 4)

| Command | Description |
|---------|-------------|
| `uniteos ai-index` | Build AI search index (TF-IDF embeddings) |
| `uniteos ai-search <query>` | Semantic search across files |
| `uniteos ai-analyze` | AI workspace analysis with insights |

### USB Sync

| Command | Description |
|---------|-------------|
| `uniteos usb-export <path>` | Export sync bundle to USB drive / folder |
| `uniteos usb-import <path>` | Import sync bundle from USB drive / folder |
| `uniteos usb-scan <path>` | Scan USB drive for available sync bundles |

---

## Connecting Multiple Devices

uniteOS supports automatic multi-device sync over your local network (LAN).
All devices must be on the **same WiFi/network**.

### Step 1: Set Up Each Device

On **every device** you want to sync:

```bash
# Build uniteOS (or copy the binary)
go build -o uniteos ./cmd/uniteos

# Navigate to the folder you want to sync
cd ~/shared-workspace

# Initialize
uniteos init
uniteos add .
```

### Step 2: Start Sync on Each Device

```bash
uniteos sync
```

This starts:
- **File Watcher** — detects changes in real-time
- **LAN Discovery** — broadcasts UDP beacons to find peers
- **P2P Server** — accepts TCP connections from other devices
- **CRDT Sync** — conflict-free state replication

### Step 3: Verify Peer Discovery

```bash
uniteos peers
```

You should see other devices listed with their Device ID, name, and IP.

---

## Connecting via USB (No Internet/Network)

If you don't have a network connection, you can sync devices using a USB drive (Sneakernet) or USB Tethering.

### Mode 1: USB Drive (Sneakernet)

This is the most secure way to sync, as it never touches a network.

1.  **Export from Device A:**
    ```bash
    uniteos usb-export D:\  # Exports your workspace to USB drive D:
    ```
2.  **Plug USB into Device B.**
3.  **Scan for Bundles:**
    ```bash
    uniteos usb-scan E:\    # Finds bundles on the second device's USB port
    ```
4.  **Import to Device B:**
    ```bash
    uniteos usb-import E:\uniteos-sync-a74c0f6b
    ```

### Mode 2: USB Tethering (Network over USB)

If you connect your phone to your PC via USB and enable "USB Tethering", your phone acts as a network adapter.
uniteOS will detect the new network interface automatically. Just run:

```bash
uniteos sync
```

It will work exactly like WiFi but over the USB cable.

### Network Requirements

| Protocol | Port | Purpose |
|----------|------|---------|
| UDP | 41234 | Peer discovery (broadcast) |
| TCP | 7891 | P2P file transfer |
| TCP | 7890 | Dashboard / REST API |

> **Firewall:** You may need to allow these ports through your firewall.
> On Windows, you'll get a prompt when you first run `uniteos sync`.

### Supported Platforms

| Platform | Status |
|----------|--------|
| Windows 10/11 | ✅ Fully supported |
| Linux (Ubuntu, Fedora, etc.) | ✅ Fully supported |
| macOS (Intel & Apple Silicon) | ✅ Fully supported |
| Android (via Termux) | ⚡ Experimental |
| Raspberry Pi | ✅ Fully supported |

### Cross-Compile for Other Devices

```bash
# Build for Linux (from Windows)
set GOOS=linux
set GOARCH=amd64
go build -o uniteos-linux ./cmd/uniteos

# Build for macOS
set GOOS=darwin
set GOARCH=arm64
go build -o uniteos-macos ./cmd/uniteos

# Build for Raspberry Pi
set GOOS=linux
set GOARCH=arm
go build -o uniteos-rpi ./cmd/uniteos

# Build for Android (Termux)
set GOOS=android
set GOARCH=arm64
go build -o uniteos-android ./cmd/uniteos
```

---

## REST API

All endpoints are available at `http://localhost:7890/api/`

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/health` | GET | Health check |
| `/api/status` | GET | File status |
| `/api/files` | GET/POST | List/track files |
| `/api/search?q=` | GET | Search files |
| `/api/snapshots` | GET/POST | List/create snapshots |
| `/api/stats` | GET | Storage statistics |
| `/api/events` | GET | Event history |
| `/api/info` | GET | System information |
| `/api/integrity/verify` | GET | Verify file integrity |
| `/api/share` | GET/POST | Secure file sharing |
| `/api/plugins` | GET | Plugin list |
| `/api/ai/search?q=` | GET | Semantic AI search |
| `/api/ai/index` | POST | Build AI index |
| `/api/ai/analyze` | GET | Workspace analysis |
| `/api/ai/stats` | GET | AI statistics |

---

## Architecture

```
uniteos.exe  (15.7 MB self-contained binary)
├── Core Engine (config, events, lifecycle)
├── Storage (SQLite + content-addressable blobs)
├── Crypto (AES-256-GCM + Argon2id key derivation)
├── Sync (CRDTs, delta diff, conflict resolution)
├── Network (UDP discovery + TCP P2P)
├── Watcher (fsnotify real-time monitoring)
├── Dashboard (embedded glassmorphic SPA)
├── Integrity (hash verification + secure sharing)
├── Plugins (extensible event hooks)
└── AI (TF-IDF embeddings, vector search, analyzer)
```

## License

Private — All rights reserved.
