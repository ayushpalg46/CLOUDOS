// CloudOS — Local-First Personal Cloud OS
//
// A privacy-first, decentralized, cross-device computing ecosystem.
// All data is created, stored, and processed locally by default.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/ayushgpal/cloudos/internal/ai"
	"github.com/ayushgpal/cloudos/internal/api"
	"github.com/ayushgpal/cloudos/internal/core"
	"github.com/ayushgpal/cloudos/internal/crypto"
	"github.com/ayushgpal/cloudos/internal/gui"
	"github.com/ayushgpal/cloudos/internal/integrity"
	"github.com/ayushgpal/cloudos/internal/network"
	"github.com/ayushgpal/cloudos/internal/plugins"
	"github.com/ayushgpal/cloudos/internal/storage"
	csync "github.com/ayushgpal/cloudos/internal/sync"
	"github.com/ayushgpal/cloudos/internal/usb"
	"github.com/ayushgpal/cloudos/internal/watcher"
)

const banner = `
   _____ _                 _  ____   _____ 
  / ____| |               | |/ __ \ / ____|
 | |    | | ___  _   _  __| | |  | | (___  
 | |    | |/ _ \| | | |/ _` + "`" + ` | |  | |\___ \ 
 | |____| | (_) | |_| | (_| | |__| |____) |
  \_____|_|\___/ \__,_|\__,_|\____/|_____/ 
                                  v%s
  Local-First Personal Cloud OS
  ─────────────────────────────────────────
`

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "init":
		cmdInit()
	case "status":
		cmdStatus()
	case "add":
		cmdAdd()
	case "snapshot":
		cmdSnapshot()
	case "history":
		cmdHistory()
	case "rollback":
		cmdRollback()
	case "encrypt":
		cmdEncrypt()
	case "decrypt":
		cmdDecrypt()
	case "search":
		cmdSearch()
	case "config":
		cmdConfig()
	case "serve":
		cmdServe()
	case "dashboard", "gui":
		cmdGUI()
	case "watch":
		cmdWatch()
	case "sync":
		cmdSync()
	case "peers":
		cmdPeers()
	case "conflicts":
		cmdConflicts()
	case "verify":
		cmdVerify()
	case "plugins":
		cmdPlugins()
	case "ai-search":
		cmdAISearch()
	case "ai-index":
		cmdAIIndex()
	case "ai-analyze":
		cmdAIAnalyze()
	case "usb-export":
		cmdUSBExport()
	case "usb-import":
		cmdUSBImport()
	case "usb-scan":
		cmdUSBScan()
	case "info":
		cmdInfo()
	case "help", "--help", "-h":
		printUsage()
	case "version", "--version", "-v":
		fmt.Printf("CloudOS v%s\n", core.Version)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf(banner, core.Version)
	fmt.Print(`
USAGE:
  cloudos <command> [arguments]

COMMANDS:
  init                  Initialize a new CloudOS workspace
  status                Show tracked files and their status
  add <path>            Track a file or directory
  snapshot [name]       Create a version snapshot
  history [file]        Show version history
  rollback <snap-id>    Rollback to a previous snapshot
  encrypt <path>        Encrypt a file (requires passphrase)
  decrypt <path>        Decrypt a file (requires passphrase)
  search <query>        Search files by name or metadata
  config                Show current configuration
  serve                 Start the local REST API server
  info                  Show system information
  version               Show version
  help                  Show this help message

  ── Phase 2: Multi-Device Sync ──
  watch                 Watch workspace for changes in real-time
  sync                  Start sync daemon (discovery + P2P + watcher)
  peers                 Discover and list LAN peers
  conflicts             View and resolve sync conflicts

  ── Phase 3: GUI + Security + Plugins ──
  gui                   Open the native desktop application
  dashboard             Alias for gui
  verify                Verify integrity of all tracked files
  plugins               List registered plugins and their status

  ── Phase 4: AI Intelligence ──
  ai-search <query>     Semantic search across files (AI-powered)
  ai-index              Build AI index (embeddings) for all files
  ai-analyze            AI workspace analysis with insights

  ── USB Sync ──
  usb-export <path>     Export sync bundle to USB drive / folder
  usb-import <path>     Import sync bundle from USB drive / folder
  usb-scan <path>       Scan USB drive for available sync bundles

EXAMPLES:
  cloudos init
  cloudos add ./documents
  cloudos snapshot "Weekly backup"
  cloudos search "report"
  cloudos serve
  cloudos watch
  cloudos sync
  cloudos peers
`)
}

func getWorkspaceDir() string {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot determine working directory: %v\n", err)
		os.Exit(1)
	}
	return dir
}

func requireInit() string {
	dir := getWorkspaceDir()
	if !core.IsInitialized(dir) {
		fmt.Fprintf(os.Stderr, "Error: not a CloudOS workspace. Run 'cloudos init' first.\n")
		os.Exit(1)
	}
	return dir
}

func initEngine(dir string) (*core.Engine, *storage.Store) {
	engine, err := core.NewEngine(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to initialize engine: %v\n", err)
		os.Exit(1)
	}
	engine.Start()

	store, err := storage.NewStore(engine.Config, engine.EventBus, engine.Logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to initialize storage: %v\n", err)
		os.Exit(1)
	}

	return engine, store
}

// ── Commands ─────────────────────────────────────────────────────

func cmdInit() {
	dir := getWorkspaceDir()

	if core.IsInitialized(dir) {
		fmt.Println("⚠  CloudOS workspace already initialized here.")
		return
	}

	engine, store := initEngine(dir)
	defer store.Close()

	fmt.Printf(banner, core.Version)
	fmt.Printf("✅ Workspace initialized at: %s\n", dir)
	fmt.Printf("📁 Data directory: %s\n", engine.Config.DataDir)
	fmt.Printf("🔑 Device ID: %s\n", engine.Config.DeviceID)
	fmt.Printf("💻 Device Name: %s\n", engine.Config.DeviceName)
	fmt.Println("\nRun 'cloudos add .' to start tracking files.")
}

func cmdStatus() {
	dir := requireInit()
	_, store := initEngine(dir)
	defer store.Close()

	statuses, err := store.GetStatus()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(statuses) == 0 {
		fmt.Println("No tracked files. Run 'cloudos add <path>' to track files.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "STATUS\tFILE\tSIZE\tHASH")
	fmt.Fprintln(w, "──────\t────\t────\t────")

	modified, unchanged, deleted := 0, 0, 0
	for _, s := range statuses {
		icon := "✓"
		switch s.Status {
		case "modified":
			icon = "✎"
			modified++
		case "deleted":
			icon = "✗"
			deleted++
		default:
			unchanged++
		}
		hash := s.OldHash
		if len(hash) > 12 {
			hash = hash[:12]
		}
		fmt.Fprintf(w, "%s %s\t%s\t%s\t%s\n", icon, s.Status, s.Path, formatSize(s.Size), hash)
	}
	w.Flush()

	fmt.Printf("\n📊 Total: %d files | ✓ %d unchanged | ✎ %d modified | ✗ %d deleted\n",
		len(statuses), unchanged, modified, deleted)
}

func cmdAdd() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: cloudos add <path>")
		os.Exit(1)
	}

	dir := requireInit()
	_, store := initEngine(dir)
	defer store.Close()

	target := os.Args[2]
	absTarget, err := filepath.Abs(target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid path: %v\n", err)
		os.Exit(1)
	}

	info, err := os.Stat(absTarget)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if info.IsDir() {
		fmt.Printf("📁 Tracking directory: %s\n", target)
		count, err := store.TrackDirectory(absTarget)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Tracked %d items\n", count)
	} else {
		if err := store.TrackFile(absTarget); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Tracked: %s\n", target)
	}
}

func cmdSnapshot() {
	dir := requireInit()
	_, store := initEngine(dir)
	defer store.Close()

	name := fmt.Sprintf("Snapshot %s", time.Now().Format("2006-01-02 15:04:05"))
	desc := ""
	if len(os.Args) > 2 {
		name = os.Args[2]
	}
	if len(os.Args) > 3 {
		desc = strings.Join(os.Args[3:], " ")
	}

	// Update all tracked files first
	files, _ := store.DB.ListTrackedFiles()
	updatedCount := 0
	for _, f := range files {
		if f.IsDir {
			continue
		}
		changed, err := store.UpdateFile(f.Path)
		if err == nil && changed {
			updatedCount++
		}
	}

	snap, err := store.CreateSnapshot(name, desc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("📸 Snapshot created!\n")
	fmt.Printf("   ID:    %s\n", snap.SnapshotID)
	fmt.Printf("   Name:  %s\n", snap.Name)
	fmt.Printf("   Files: %d\n", snap.FileCount)
	fmt.Printf("   Size:  %s\n", formatSize(snap.TotalSize))
	if updatedCount > 0 {
		fmt.Printf("   Updated: %d files had changes\n", updatedCount)
	}
}

func cmdHistory() {
	dir := requireInit()
	_, store := initEngine(dir)
	defer store.Close()

	if len(os.Args) > 2 {
		// Show version history for a specific file
		target := os.Args[2]
		absTarget, _ := filepath.Abs(target)
		file, err := store.DB.GetTrackedFile(absTarget)
		if err != nil || file == nil {
			fmt.Fprintf(os.Stderr, "Error: file not tracked: %s\n", target)
			os.Exit(1)
		}

		versions, err := store.DB.GetFileVersions(file.ID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("📜 Version history for: %s\n\n", file.RelativePath)
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "VERSION\tCHANGE\tHASH\tSIZE\tDATE")
		for _, v := range versions {
			hash := v.Hash
			if len(hash) > 12 {
				hash = hash[:12]
			}
			fmt.Fprintf(w, "v%d\t%s\t%s\t%s\t%s\n",
				v.VersionNum, v.ChangeType, hash,
				formatSize(v.Size), v.CreatedAt.Format("2006-01-02 15:04"))
		}
		w.Flush()
		return
	}

	// Show snapshot history
	snapshots, err := store.DB.ListSnapshots()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(snapshots) == 0 {
		fmt.Println("No snapshots yet. Run 'cloudos snapshot' to create one.")
		return
	}

	fmt.Println("📜 Snapshot History")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tFILES\tSIZE\tDATE")
	for _, s := range snapshots {
		id := s.SnapshotID
		if len(id) > 20 {
			id = id[:20]
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n",
			id, s.Name, s.FileCount, formatSize(s.TotalSize),
			s.CreatedAt.Format("2006-01-02 15:04"))
	}
	w.Flush()
}

func cmdRollback() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: cloudos rollback <snapshot-id>")
		os.Exit(1)
	}

	dir := requireInit()
	_, store := initEngine(dir)
	defer store.Close()

	snapshotID := os.Args[2]

	// Try to find by partial match
	snapshots, _ := store.DB.ListSnapshots()
	var matchID string
	for _, s := range snapshots {
		if s.SnapshotID == snapshotID || strings.HasPrefix(s.SnapshotID, snapshotID) {
			matchID = s.SnapshotID
			break
		}
	}

	if matchID == "" {
		fmt.Fprintf(os.Stderr, "Error: snapshot not found: %s\n", snapshotID)
		os.Exit(1)
	}

	fmt.Printf("⏪ Rolling back to snapshot: %s\n", matchID)
	if err := store.RestoreSnapshot(matchID); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Rollback complete!")
}

func cmdEncrypt() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: cloudos encrypt <path>")
		os.Exit(1)
	}

	dir := requireInit()
	engine, _ := initEngine(dir)

	target := os.Args[2]
	absTarget, _ := filepath.Abs(target)

	// Initialize key manager
	km, err := crypto.NewKeyManager(engine.Config.DataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if !km.IsInitialized() {
		fmt.Print("🔑 Set encryption passphrase: ")
		var passphrase string
		fmt.Scanln(&passphrase)
		if err := km.Initialize(engine.Config.DeviceName, "desktop", passphrase); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	} else if !km.IsUnlocked() {
		fmt.Print("🔐 Enter passphrase: ")
		var passphrase string
		fmt.Scanln(&passphrase)
		if err := km.Unlock(passphrase); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	key, _ := km.GetMasterKey()
	destPath := absTarget + ".enc"
	if err := crypto.EncryptFile(absTarget, destPath, key); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("🔒 Encrypted: %s → %s\n", target, target+".enc")
}

func cmdDecrypt() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: cloudos decrypt <path>")
		os.Exit(1)
	}

	dir := requireInit()
	engine, _ := initEngine(dir)

	target := os.Args[2]
	absTarget, _ := filepath.Abs(target)

	km, err := crypto.NewKeyManager(engine.Config.DataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if !km.IsInitialized() {
		fmt.Fprintln(os.Stderr, "Error: encryption not set up. Run 'cloudos encrypt' first.")
		os.Exit(1)
	}

	fmt.Print("🔐 Enter passphrase: ")
	var passphrase string
	fmt.Scanln(&passphrase)
	if err := km.Unlock(passphrase); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	key, _ := km.GetMasterKey()
	destPath := strings.TrimSuffix(absTarget, ".enc")
	if destPath == absTarget {
		destPath = absTarget + ".dec"
	}

	if err := crypto.DecryptFile(absTarget, destPath, key); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	relDest, _ := filepath.Rel(dir, destPath)
	fmt.Printf("🔓 Decrypted: %s → %s\n", target, relDest)
}

func cmdSearch() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: cloudos search <query>")
		os.Exit(1)
	}

	dir := requireInit()
	_, store := initEngine(dir)
	defer store.Close()

	query := strings.Join(os.Args[2:], " ")
	files, err := store.DB.SearchFiles(query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(files) == 0 {
		fmt.Printf("No files matching: %s\n", query)
		return
	}

fmt.Printf("🔍 Search results for \"%s\":\n\n", query)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "FILE\tSIZE\tMODIFIED")
	for _, f := range files {
		fmt.Fprintf(w, "%s\t%s\t%s\n",
			f.RelativePath, formatSize(f.Size),
			f.ModTime.Format("2006-01-02 15:04"))
	}
	w.Flush()
	fmt.Printf("\n%d file(s) found\n", len(files))
}

func cmdConfig() {
	dir := requireInit()
	engine, _ := initEngine(dir)

	data, _ := json.MarshalIndent(engine.Config, "", "  ")
	fmt.Println("⚙  CloudOS Configuration:")
	fmt.Println(string(data))
}

func startBackgroundSync(engine *core.Engine, store *storage.Store, dir string) *csync.SyncManager {
	p2pPort := engine.Config.APIPort + 1

	syncMgr := csync.NewSyncManager(
		engine.Config.DeviceID, dir,
		store, engine.EventBus,
		csync.StrategyLastWriterWins,
		engine.Logger,
	)
	syncMgr.Start()

	w, err := watcher.NewWatcher(store, engine.EventBus, dir, engine.Logger)
	if err == nil {
		w.Start()
	}

	p2pServer := network.NewP2PServer(
		engine.Config.DeviceID, engine.Config.DeviceName,
		p2pPort, dir, syncMgr, engine.EventBus, engine.Logger,
	)
	p2pServer.Start()

	discovery := network.NewDiscovery(
		engine.Config.DeviceID, engine.Config.DeviceName,
		p2pPort, engine.EventBus, engine.Logger,
	)

	discovery.OnPeerFound(func(peer network.DiscoveredPeer) {
		client := network.NewP2PClient(engine.Config.DeviceID, engine.Logger)
		conn, err := client.Connect(peer.Address, peer.Port)
		if err != nil { return }
		defer conn.Close()

		_, err = client.Handshake(conn, engine.Config.DeviceName, engine.Config.APIPort+1)
		if err != nil { return }

		remoteStates, err := client.RequestSync(conn, true, nil)
		if err != nil { return }

		toDownload, _, _ := syncMgr.SyncWithPeer(peer.DeviceID, remoteStates)

		for _, state := range toDownload {
			absPath := filepath.Join(dir, state.Path)
			
			// Handle deletions
			if deleted, ok := state.Deleted.Value.(bool); ok && deleted {
				os.Remove(absPath)
				if tf, err := store.DB.GetTrackedFile(absPath); err == nil && tf != nil {
					tf.Status = "deleted"
					store.DB.AddTrackedFile(tf)
				}
				continue
			}

			// Download the actual file data
			data, err := client.RequestFile(conn, state.Path, state.Hash.Value.(string))
			if err != nil { continue }

			// Save to local workspace
			os.MkdirAll(filepath.Dir(absPath), 0755)
			os.WriteFile(absPath, data, 0644)

			// Tell local store to track the newly downloaded file
			store.TrackFile(absPath)
		}

		syncMgr.AddPeer(&csync.PeerInfo{
			DeviceID:   peer.DeviceID,
			DeviceName: peer.DeviceName,
			Address:    fmt.Sprintf("%s:%d", peer.Address, peer.Port),
			LastSeen:   time.Now(),
			Connected:  true,
		})
	})

	discovery.OnPeerLost(func(deviceID string) {
		syncMgr.RemovePeer(deviceID)
	})

	// Continuous background sync loop
	go func() {
		for {
			time.Sleep(5 * time.Second)
			peers := syncMgr.GetPeers()
			for _, peer := range peers {
				if !peer.Connected {
					continue
				}
				host, portStr, err := net.SplitHostPort(peer.Address)
				if err != nil {
					continue
				}
				port, _ := strconv.Atoi(portStr)

				client := network.NewP2PClient(engine.Config.DeviceID, engine.Logger)
				conn, err := client.Connect(host, port)
				if err != nil {
					continue
				}

				_, err = client.Handshake(conn, engine.Config.DeviceName, engine.Config.APIPort+1)
				if err != nil {
					conn.Close()
					continue
				}

				remoteStates, err := client.RequestSync(conn, true, nil)
				if err != nil {
					conn.Close()
					continue
				}

				toDownload, _, _ := syncMgr.SyncWithPeer(peer.DeviceID, remoteStates)

				for _, state := range toDownload {
					absPath := filepath.Join(dir, state.Path)

					if deleted, ok := state.Deleted.Value.(bool); ok && deleted {
						os.Remove(absPath)
						if tf, err := store.DB.GetTrackedFile(absPath); err == nil && tf != nil {
							tf.Status = "deleted"
							store.DB.AddTrackedFile(tf)
						}
						continue
					}

					data, err := client.RequestFile(conn, state.Path, state.Hash.Value.(string))
					if err != nil {
						continue
					}

					os.MkdirAll(filepath.Dir(absPath), 0755)
					os.WriteFile(absPath, data, 0644)
					store.TrackFile(absPath)
				}
				conn.Close()
			}
		}
	}()

	go discovery.Start(context.Background())
	return syncMgr
}

func cmdGUI() {
	// Global recovery to prevent silent crashes in windowsgui mode
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "CRITICAL ERROR: %v\n", r)
		}
	}()

	dir := getWorkspaceDir()
	
	// Automatic initialization for a seamless "first-run" experience
	if !core.IsInitialized(dir) {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		defaultDir := filepath.Join(home, "CloudOS")
		
		if !core.IsInitialized(defaultDir) {
			os.MkdirAll(defaultDir, 0755)
			engine, store := initEngine(defaultDir)
			if store != nil { store.Close() }
			if engine != nil { engine.Stop() }
		}
		dir = defaultDir
	}

	engine, store := initEngine(dir)
	
	// Start the Wi-Fi auto-sync daemon in the background later when we know the API port
	ln, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		// Fallback to searching if :0 fails (rare)
		port := 7890
		for i := 0; i < 50; i++ {
			ln, err = net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port+i))
			if err == nil {
				break
			}
		}
	}

	if ln == nil {
		os.Exit(1)
	}

	// Get the port the OS actually assigned
	port := ln.Addr().(*net.TCPAddr).Port
	
	// Set the correct API port before starting background sync so P2P uses it
	engine.Config.APIPort = port
	
	// Start the Wi-Fi auto-sync daemon in the background
	syncMgr := startBackgroundSync(engine, store, dir)

	server := api.NewServer(engine, store, port)
	server.SetSyncManager(syncMgr)

	// Start server using the locked listener
	go func() {
		if err := server.Serve(ln); err != nil {
			os.Exit(1)
		}
	}()

	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	
	// Fast heartbeat check in Go to ensure server is serving before opening Edge
	for i := 0; i < 20; i++ {
		resp, err := http.Get(url + "/api/health")
		if err == nil {
			resp.Body.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	
	// Print connection information for the user
	lanIP := "127.0.0.1"
	if conn, err := net.Dial("udp", "8.8.8.8:80"); err == nil {
		lanIP = conn.LocalAddr().(*net.UDPAddr).IP.String()
		conn.Close()
	}

	fmt.Printf(banner, core.Version)
	fmt.Printf("🌐 Local Dashboard:   http://127.0.0.1:%d\n", port)
	fmt.Printf("📱 Android Fallback: %s:%d\n", lanIP, port)
	fmt.Printf("📡 Device ID:        %s\n", engine.Config.DeviceID)
	fmt.Println("---------------------------------------------------------")

	// Start native window directly
	gui.StartWindow(gui.Config{
		Title:  "CloudOS — Personal Cloud",
		URL:    url,
		Width:  1200,
		Height: 800,
	})

	// Keep the main process alive so the background server doesn't die!
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	<-sigCh
}

func cmdServe() {
	dir := requireInit()
	engine, store := initEngine(dir)
	defer store.Close()

	server := api.NewServer(engine, store, engine.Config.APIPort)

	lanIP := "127.0.0.1"
	if conn, err := net.Dial("udp", "8.8.8.8:80"); err == nil {
		lanIP = conn.LocalAddr().(*net.UDPAddr).IP.String()
		conn.Close()
	}

	fmt.Printf(banner, core.Version)
	fmt.Printf("🌐 Local Dashboard:   http://127.0.0.1:%d\n", engine.Config.APIPort)
	fmt.Printf("📱 Android Fallback: %s:%d\n", lanIP, engine.Config.APIPort)
	fmt.Printf("📡 Device ID:        %s\n", engine.Config.DeviceID)
	fmt.Println("---------------------------------------------------------")
	fmt.Println("   Press Ctrl+C to stop")
	fmt.Println("   API Endpoints:")
	fmt.Println("   GET  /api/health            — Health check")
	fmt.Println("   GET  /api/status            — File status")
	fmt.Println("   GET  /api/files             — List tracked files")
	fmt.Println("   GET  /api/search?q=         — Search files")
	fmt.Println("   GET  /api/snapshots         — List snapshots")
	fmt.Println("   GET  /api/stats             — Storage stats")
	fmt.Println("   GET  /api/events            — Event history")
	fmt.Println("   GET  /api/info              — System info")
	fmt.Println("   GET  /api/integrity/verify   — Verify file integrity")
	fmt.Println("   POST /api/share             — Generate share token")
	fmt.Println("   GET  /api/plugins           — List plugins")
	fmt.Println("   GET  /api/ai/search?q=      — Semantic AI search")
	fmt.Println("   POST /api/ai/index          — Build AI index")
	fmt.Println("   GET  /api/ai/analyze        — Workspace analysis")
	fmt.Println("   GET  /api/ai/stats          — AI system stats")

	if err := server.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func cmdVerify() {
	dir := requireInit()
	engine, store := initEngine(dir)
	defer store.Close()

	verifier := integrity.NewVerifier(store, engine.Logger)

	fmt.Println("🔍 Verifying integrity of all tracked files...")

	report, err := verifier.VerifyAll()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, r := range report.Results {
		icon := "✅"
		if !r.Match {
			icon = "❌"
		}
		if r.Error != "" {
			icon = "⚠ "
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", icon, r.Path, formatSize(r.Size))
	}
	w.Flush()

	fmt.Printf("\n📊 Results: %d passed | %d failed | %d errors | %s\n",
		report.Passed, report.Failed, report.Errors, report.Duration)

	if report.Failed > 0 {
		fmt.Println("\n⚠  Some files have been modified outside CloudOS tracking!")
	}
}

func cmdPlugins() {
	dir := requireInit()
	engine, store := initEngine(dir)
	defer store.Close()

	pluginDir := engine.Config.DataDir + "/plugins"
	pm := plugins.NewPluginManager(engine.EventBus, pluginDir, engine.Logger)
	plugins.RegisterAutoVersionPlugin(pm)
	plugins.RegisterAuditLogPlugin(pm)

	list := pm.ListPlugins()

	fmt.Println("🔌 Registered Plugins:")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tVERSION\tSTATUS\tDESCRIPTION")
	fmt.Fprintln(w, "────\t───────\t──────\t───────────")
	for _, p := range list {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			p.Manifest.Name, p.Manifest.Version,
			p.State, p.Manifest.Description)
	}
	w.Flush()
	fmt.Printf("\n%d plugin(s)\n", len(list))
}

// ── Phase 4: AI Commands ────────────────────────────────────────

func cmdAISearch() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: cloudos ai-search <query>")
		os.Exit(1)
	}

	dir := requireInit()
	engine, store := initEngine(dir)
	defer store.Close()

	query := strings.Join(os.Args[2:], " ")

	aiMgr := ai.NewManager(store, engine.EventBus, engine.Config.DataDir, engine.Logger)
	aiMgr.Start()
	defer aiMgr.Stop()

	// Always rebuild index to ensure TF-IDF vocabulary is loaded
	// (vectors persist but vocabulary must be recomputed — fast: ~5ms)
	if _, err := aiMgr.IndexAll(); err != nil {
		fmt.Fprintf(os.Stderr, "Error indexing: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("🔍 Semantic search: \"%s\"\n\n", query)
	results := aiMgr.SemanticSearch(query, 10)

	if len(results) == 0 {
		fmt.Println("No matching files found.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SCORE\tFILE\tCATEGORY\tTAGS")
	fmt.Fprintln(w, "─────\t────\t────────\t────")
	for _, r := range results {
		tags := strings.Join(r.Entry.Tags, ", ")
		if len(tags) > 40 {
			tags = tags[:40] + "..."
		}
		cat := r.Entry.Metadata["category"]
		fmt.Fprintf(w, "%.2f\t%s\t%s\t%s\n", r.Score, r.Entry.ID, cat, tags)
	}
	w.Flush()
	fmt.Printf("\n%d result(s)\n", len(results))
}

func cmdAIIndex() {
	dir := requireInit()
	engine, store := initEngine(dir)
	defer store.Close()

	aiMgr := ai.NewManager(store, engine.EventBus, engine.Config.DataDir, engine.Logger)
	aiMgr.Start()
	defer aiMgr.Stop()

	fmt.Println("🧠 Building AI index...")
	fmt.Println("   Extracting content, computing TF-IDF embeddings...")

	report, err := aiMgr.IndexAll()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ AI index built!")
	fmt.Printf("   📄 Indexed:    %d files\n", report.Indexed)
	fmt.Printf("   📚 Vocabulary: %d terms\n", report.VocabularySize)
	fmt.Printf("   🔢 Vectors:    %d embeddings\n", report.TotalVectors)
	fmt.Printf("   ⏱  Duration:   %s\n", report.Duration)
	if report.Errors > 0 {
		fmt.Printf("   ⚠  Errors:     %d\n", report.Errors)
	}
}

func cmdAIAnalyze() {
	dir := requireInit()
	engine, store := initEngine(dir)
	defer store.Close()

	aiMgr := ai.NewManager(store, engine.EventBus, engine.Config.DataDir, engine.Logger)
	aiMgr.Start()
	defer aiMgr.Stop()

	// Build index if needed
	stats := aiMgr.GetStats()
	if stats["indexed_files"].(int) == 0 {
		fmt.Println("🧠 Building AI index first...")
		aiMgr.IndexAll()
		fmt.Println()
	}

	fmt.Println("🔬 Analyzing workspace...")

	analysis, err := aiMgr.AnalyzeWorkspace()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Categories
	fmt.Println("📊 File Categories:")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for cat, count := range analysis.Categories {
		pct := float64(count) / float64(analysis.TotalFiles) * 100
		bar := strings.Repeat("█", int(pct/5)) + strings.Repeat("░", 20-int(pct/5))
		fmt.Fprintf(w, "   %s\t%d\t%s\t%.0f%%\n", cat, count, bar, pct)
	}
	w.Flush()

	// Languages
	if len(analysis.Languages) > 0 {
		fmt.Println("\n💻 Programming Languages:")
		for lang, count := range analysis.Languages {
			fmt.Printf("   %s: %d files\n", lang, count)
		}
	}

	// Insights
	if len(analysis.Insights) > 0 {
		fmt.Println("\n💡 AI Insights:")
		for i, insight := range analysis.Insights {
			fmt.Printf("   %d. %s\n", i+1, insight)
		}
	}

	// Organization suggestions
	if analysis.Organization != nil && len(analysis.Organization.Suggestions) > 0 {
		fmt.Printf("\n📁 Organization Suggestions (%d):\n", len(analysis.Organization.Suggestions))
		for i, s := range analysis.Organization.Suggestions {
			if i >= 10 {
				fmt.Printf("   ... and %d more\n", len(analysis.Organization.Suggestions)-10)
				break
			}
			fmt.Printf("   → Move %s → %s\n", s.FilePath, s.SuggestedDir)
		}
	}

	// Duplicates
	if len(analysis.Duplicates) > 0 {
		fmt.Printf("\n🔄 Similar File Groups (%d):\n", len(analysis.Duplicates))
		for _, group := range analysis.Duplicates {
			fmt.Printf("   [%s]\n", strings.Join(group, ", "))
		}
	}

	fmt.Printf("\n📈 Total: %d files analyzed\n", analysis.TotalFiles)
}

// ── USB Sync Commands ────────────────────────────────────────────

func cmdUSBExport() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: cloudos usb-export <target-path>")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  cloudos usb-export D:\\              # Export to USB drive D:")
		fmt.Fprintln(os.Stderr, "  cloudos usb-export E:\\CloudSync     # Export to specific folder")
		fmt.Fprintln(os.Stderr, "  cloudos usb-export ./sync-bundle    # Export to local folder")
		os.Exit(1)
	}

	targetPath := os.Args[2]
	dir := requireInit()
	engine, store := initEngine(dir)
	defer store.Close()

	syncMgr := csync.NewSyncManager(engine.Config.DeviceID, engine.Config.WorkspaceDir, store, engine.EventBus, csync.StrategyLastWriterWins, engine.Logger)

	usbSync := usb.NewUSBSync(
		store, syncMgr,
		engine.Config.WorkspaceDir,
		engine.Config.DeviceID,
		engine.Config.DeviceName,
		engine.Logger,
	)

	fmt.Printf("📦 Exporting workspace to: %s\n", targetPath)

	report, err := usbSync.Export(targetPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n✅ USB export complete!")
	fmt.Printf("   📁 Bundle:    %s\n", report.BundlePath)
	fmt.Printf("   📄 Files:     %d\n", report.FilesCopied)
	fmt.Printf("   💾 Size:      %s\n", formatSize(report.BytesCopied))
	fmt.Printf("   ⏱  Duration:  %s\n", report.Duration)
	if report.Errors > 0 {
		fmt.Printf("   ⚠  Errors:    %d\n", report.Errors)
	}
	fmt.Println("\n📋 Next: Plug USB into another device and run:")
	fmt.Printf("   cloudos usb-import \"%s\"\n", report.BundlePath)
}

func cmdUSBImport() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: cloudos usb-import <bundle-path>")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  cloudos usb-import D:\\cloudos-sync-a74c0f6b")
		fmt.Fprintln(os.Stderr, "  cloudos usb-import E:\\CloudSync\\cloudos-sync-b82d3f9a")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Tip: Use 'cloudos usb-scan D:\\' to find bundles on a drive")
		os.Exit(1)
	}

	bundlePath := os.Args[2]
	dir := requireInit()
	engine, store := initEngine(dir)
	defer store.Close()

	syncMgr := csync.NewSyncManager(engine.Config.DeviceID, engine.Config.WorkspaceDir, store, engine.EventBus, csync.StrategyLastWriterWins, engine.Logger)

	usbSync := usb.NewUSBSync(
		store, syncMgr,
		engine.Config.WorkspaceDir,
		engine.Config.DeviceID,
		engine.Config.DeviceName,
		engine.Logger,
	)

	fmt.Printf("📥 Importing from: %s\n", bundlePath)

	report, err := usbSync.Import(bundlePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n✅ USB import complete! (from %s)\n", report.SourceDevice)
	fmt.Printf("   🆕 New files:   %d\n", report.NewFiles)
	fmt.Printf("   🔄 Updated:     %d\n", report.Updated)
	fmt.Printf("   ⏭  Skipped:     %d (identical)\n", report.Skipped)
	fmt.Printf("   🔗 States:      %d CRDT states merged\n", report.StatesMerged)
	fmt.Printf("   💾 Data:        %s copied\n", formatSize(report.BytesCopied))
	fmt.Printf("   ⏱  Duration:    %s\n", report.Duration)
	if report.Errors > 0 {
		fmt.Printf("   ⚠  Errors:      %d\n", report.Errors)
	}
}

func cmdUSBScan() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: cloudos usb-scan <drive-path>")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  cloudos usb-scan D:\\")
		fmt.Fprintln(os.Stderr, "  cloudos usb-scan E:\\CloudSync")
		os.Exit(1)
	}

	drivePath := os.Args[2]
	fmt.Printf("🔍 Scanning %s for CloudOS sync bundles...\n\n", drivePath)

	bundles, err := usb.ListBundles(drivePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(bundles) == 0 {
		fmt.Println("No sync bundles found.")
		fmt.Println("\nTip: Run 'cloudos usb-export <path>' on another device first.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "DEVICE\tID\tFILES\tDATE\tPATH")
	fmt.Fprintln(w, "──────\t──\t─────\t────\t────")
	for _, b := range bundles {
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n",
			b.DeviceName,
			b.DeviceID[:8]+"...",
			b.FileCount,
			b.CreatedAt.Format("2006-01-02 15:04"),
			b.Path,
		)
	}
	w.Flush()

	fmt.Printf("\n%d bundle(s) found. Import with:\n", len(bundles))
	fmt.Printf("   cloudos usb-import \"%s\"\n", bundles[0].Path)
}

// ── Phase 2 Commands ─────────────────────────────────────────────

func cmdWatch() {
	dir := requireInit()
	engine, store := initEngine(dir)
	defer store.Close()

	w, err := watcher.NewWatcher(store, engine.EventBus, dir, engine.Logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := w.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf(banner, core.Version)
	fmt.Printf("👁  Watching workspace: %s\n", dir)
	fmt.Printf("   Watched paths: %d\n", len(w.GetWatchedPaths()))
	fmt.Println("   Press Ctrl+C to stop")

	// Print events in real-time
	engine.EventBus.SubscribeAll(func(event core.Event) {
		path, _ := event.Data["path"].(string)
		switch event.Type {
		case core.EventFileCreated:
			fmt.Printf("   ✚ created:  %s\n", path)
		case core.EventFileModified:
			fmt.Printf("   ✎ modified: %s\n", path)
		case core.EventFileDeleted:
			fmt.Printf("   ✗ deleted:  %s\n", path)
		case core.EventFileTracked:
			fmt.Printf("   ⊕ tracked:  %s\n", path)
		}
	})

	// Wait for interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	w.Stop()
	fmt.Println("\n👁  Watcher stopped.")
}

func cmdSync() {
	dir := requireInit()
	engine, store := initEngine(dir)
	defer store.Close()

	p2pPort := engine.Config.APIPort + 1 // 7891

	// Initialize sync manager
	syncMgr := csync.NewSyncManager(
		engine.Config.DeviceID, dir,
		store, engine.EventBus,
		csync.StrategyLastWriterWins,
		engine.Logger,
	)
	if err := syncMgr.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting sync manager: %v\n", err)
		os.Exit(1)
	}

	// Start file watcher
	w, err := watcher.NewWatcher(store, engine.EventBus, dir, engine.Logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting watcher: %v\n", err)
		os.Exit(1)
	}
	w.Start()

	// Start P2P server
	p2pServer := network.NewP2PServer(
		engine.Config.DeviceID, engine.Config.DeviceName,
		p2pPort, dir, syncMgr, engine.EventBus, engine.Logger,
	)
	if err := p2pServer.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting P2P server: %v\n", err)
		os.Exit(1)
	}

	// Start LAN discovery
	ctx, cancel := context.WithCancel(context.Background())
	discovery := network.NewDiscovery(
		engine.Config.DeviceID, engine.Config.DeviceName,
		p2pPort, engine.EventBus, engine.Logger,
	)

	// Auto-sync when peers are found
	discovery.OnPeerFound(func(peer network.DiscoveredPeer) {
		fmt.Printf("   🔗 Peer found: %s (%s) at %s:%d\n",
			peer.DeviceName, peer.DeviceID[:12], peer.Address, peer.Port)

		// Connect and sync
		client := network.NewP2PClient(engine.Config.DeviceID, engine.Logger)
		conn, err := client.Connect(peer.Address, peer.Port)
		if err != nil {
			fmt.Printf("   ⚠ Failed to connect to %s: %v\n", peer.DeviceName, err)
			return
		}
		defer conn.Close()

		// Handshake
		hsResp, err := client.Handshake(conn, engine.Config.DeviceName, engine.Config.APIPort+1)
		if err != nil {
			fmt.Printf("   ⚠ Handshake failed: %v\n", err)
			return
		}
		fmt.Printf("   🤝 Connected: %s (v%s, %d files)\n",
			hsResp.DeviceName, hsResp.Version, hsResp.FileCount)

		// Request full sync
		remoteStates, err := client.RequestSync(conn, true, nil)
		if err != nil {
			fmt.Printf("   ⚠ Sync request failed: %v\n", err)
			return
		}

		toDownload, conflicts, err := syncMgr.SyncWithPeer(peer.DeviceID, remoteStates)
		if err != nil {
			fmt.Printf("   ⚠ Sync failed: %v\n", err)
			return
		}

		var downloaded int
		for _, state := range toDownload {
			data, err := client.RequestFile(conn, state.Path, state.Hash.Value.(string))
			if err != nil { continue }

			absPath := filepath.Join(dir, state.Path)
			os.MkdirAll(filepath.Dir(absPath), 0755)
			os.WriteFile(absPath, data, 0644)
			
			store.TrackFile(absPath)
			downloaded++
		}

		fmt.Printf("   ✅ Synced metadata for %d files, downloaded %d files from %s", len(remoteStates), downloaded, peer.DeviceName)
		if len(conflicts) > 0 {
			fmt.Printf(" (%d conflicts)", len(conflicts))
		}
		fmt.Println()

		syncMgr.AddPeer(&csync.PeerInfo{
			DeviceID:   peer.DeviceID,
			DeviceName: peer.DeviceName,
			Address:    fmt.Sprintf("%s:%d", peer.Address, peer.Port),
			LastSeen:   time.Now(),
			Connected:  true,
		})
	})

	discovery.OnPeerLost(func(deviceID string) {
		fmt.Printf("   📡 Peer lost: %s\n", deviceID[:12])
		syncMgr.RemovePeer(deviceID)
	})

	if err := discovery.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: LAN discovery failed: %v\n", err)
		fmt.Println("   Continuing without LAN discovery...")
	}

	// Start API server in background
	apiServer := api.NewServer(engine, store, engine.Config.APIPort)
	go apiServer.Start()

	fmt.Printf(banner, core.Version)
	fmt.Println("🔄 Sync daemon running")
	fmt.Printf("   Device:     %s (%s)\n", engine.Config.DeviceName, engine.Config.DeviceID[:12])
	fmt.Printf("   Workspace:  %s\n", dir)
	fmt.Printf("   P2P Port:   %d\n", p2pPort)
	fmt.Printf("   API Port:   %d\n", engine.Config.APIPort)
	fmt.Println("   Watching for file changes...")
	fmt.Println("   Discovering LAN peers...")
	fmt.Println("   Press Ctrl+C to stop")

	// Wait for interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\n⏹  Shutting down...")
	cancel()
	discovery.Stop()
	p2pServer.Stop()
	w.Stop()
	syncMgr.Stop()
	engine.Stop()
	fmt.Println("✅ Sync daemon stopped.")
}

func cmdPeers() {
	dir := requireInit()
	engine, _ := initEngine(dir)

	p2pPort := engine.Config.APIPort + 1
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	discovery := network.NewDiscovery(
		engine.Config.DeviceID, engine.Config.DeviceName,
		p2pPort, engine.EventBus, engine.Logger,
	)

	fmt.Println("📡 Scanning LAN for CloudOS peers (5 seconds)...")

	if err := discovery.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	<-ctx.Done()
	discovery.Stop()

	peers := discovery.GetPeers()
	if len(peers) == 0 {
		fmt.Println("No peers found on the local network.")
		fmt.Println("Make sure other CloudOS instances are running 'cloudos sync'.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "DEVICE\tID\tADDRESS\tPORT\tVERSION")
	fmt.Fprintln(w, "──────\t──\t───────\t────\t───────")
	for _, p := range peers {
		id := p.DeviceID
		if len(id) > 12 {
			id = id[:12]
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
			p.DeviceName, id, p.Address, p.Port, p.Version)
	}
	w.Flush()
	fmt.Printf("\n%d peer(s) found\n", len(peers))
}

func cmdConflicts() {
	dir := requireInit()
	engine, store := initEngine(dir)
	defer store.Close()

	syncMgr := csync.NewSyncManager(
		engine.Config.DeviceID, dir,
		store, engine.EventBus,
		csync.StrategyLastWriterWins,
		engine.Logger,
	)
	syncMgr.Start()
	defer syncMgr.Stop()

	conflicts := syncMgr.GetConflicts()

	if len(conflicts) == 0 {
		fmt.Println("✅ No unresolved conflicts.")
		return
	}

	fmt.Printf("⚠  %d unresolved conflict(s):\n\n", len(conflicts))
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tFILE\tSTRATEGY\tCREATED")
	for _, c := range conflicts {
		id := c.ID
		if len(id) > 20 {
			id = id[:20]
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			id, c.FilePath, c.Strategy,
			c.CreatedAt.Format("2006-01-02 15:04"))
	}
	w.Flush()

	if len(os.Args) > 2 && os.Args[2] == "resolve" && len(os.Args) > 4 {
		conflictID := os.Args[3]
		resolution := os.Args[4]
		if err := syncMgr.ResolveConflict(conflictID, resolution); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("\n✅ Conflict %s resolved as '%s'\n", conflictID, resolution)
	} else {
		fmt.Println("\nTo resolve: cloudos conflicts resolve <conflict-id> <local|remote|both>")
	}
}

func cmdInfo() {
	dir := requireInit()
	engine, store := initEngine(dir)
	defer store.Close()

	stats, _ := store.DB.GetStats()
	blobSize, _ := store.Blobs.GetStoreSize()

	fmt.Printf(banner, core.Version)
	fmt.Printf("  Device ID:     %s\n", engine.Config.DeviceID)
	fmt.Printf("  Device Name:   %s\n", engine.Config.DeviceName)
	fmt.Printf("  Workspace:     %s\n", engine.Config.WorkspaceDir)
	fmt.Printf("  Data Dir:      %s\n", engine.Config.DataDir)
	fmt.Printf("  API Port:      %d\n", engine.Config.APIPort)
	fmt.Println()
	fmt.Printf("  📊 Storage Statistics:\n")
	fmt.Printf("     Tracked Files:  %v\n", stats["tracked_files"])
	fmt.Printf("     Tracked Dirs:   %v\n", stats["tracked_dirs"])
	fmt.Printf("     Versions:       %v\n", stats["versions"])
	fmt.Printf("     Snapshots:      %v\n", stats["snapshots"])
	fmt.Printf("     Total Size:     %s\n", formatSize(stats["total_size_bytes"].(int64)))
	fmt.Printf("     Blob Store:     %s\n", formatSize(blobSize))
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
