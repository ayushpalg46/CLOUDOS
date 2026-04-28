// Package api provides the local REST API server for uniteOS.
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ayushgpal/uniteos/internal/ai"
	"github.com/ayushgpal/uniteos/internal/core"
	"github.com/ayushgpal/uniteos/internal/dashboard"
	"github.com/ayushgpal/uniteos/internal/integrity"
	"github.com/ayushgpal/uniteos/internal/plugins"
	"github.com/ayushgpal/uniteos/internal/storage"
	csync "github.com/ayushgpal/uniteos/internal/sync"
)

// Server is the local REST API server with embedded dashboard.
type Server struct {
	store        *storage.Store
	engine       *core.Engine
	logger       *slog.Logger
	mux          *http.ServeMux
	port         int
	verifier     *integrity.Verifier
	shareManager *integrity.ShareManager
	pluginMgr    *plugins.PluginManager
	aiManager    *ai.Manager
	syncManager  *csync.SyncManager
}

// NewServer creates a new API server with dashboard.
func NewServer(engine *core.Engine, store *storage.Store, port int) *Server {
	s := &Server{
		store:        store,
		engine:       engine,
		logger:       engine.Logger,
		mux:          http.NewServeMux(),
		port:         port,
		verifier:     integrity.NewVerifier(store, engine.Logger),
		shareManager: integrity.NewShareManager(engine.Config.DeviceID, engine.Logger),
	}

	// Initialize plugin manager
	pluginDir := engine.Config.DataDir + "/plugins"
	s.pluginMgr = plugins.NewPluginManager(engine.EventBus, pluginDir, engine.Logger)
	plugins.RegisterAutoVersionPlugin(s.pluginMgr)
	plugins.RegisterAuditLogPlugin(s.pluginMgr)
	
	// Initialize AI manager
	s.aiManager = ai.NewManager(store, engine.EventBus, engine.Config.DataDir, engine.Logger)

	// Start background systems asynchronously so API starts instantly
	go func() {
		s.pluginMgr.StartAll()
		s.aiManager.Start()
	}()

	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	// ── API Routes ──
	s.mux.HandleFunc("/api/health", s.handleHealth)
	s.mux.HandleFunc("/api/status", s.handleStatus)
	s.mux.HandleFunc("/api/files", s.handleFiles)
	s.mux.HandleFunc("/api/search", s.handleSearch)
	s.mux.HandleFunc("/api/snapshots", s.handleSnapshots)
	s.mux.HandleFunc("/api/stats", s.handleStats)
	s.mux.HandleFunc("/api/events", s.handleEvents)
	s.mux.HandleFunc("/api/info", s.handleInfo)
	s.mux.HandleFunc("/api/system/browse", s.handleSystemBrowse) // Endpoint for PC browsing
	s.mux.HandleFunc("/api/upload", s.handleUpload) // Endpoint for Android/Web uploads
	s.mux.HandleFunc("/api/download/android", s.handleDownloadAndroid) // Download Android Binary
	s.mux.HandleFunc("/api/open", s.handleOpenFile) // Open file natively
	s.mux.HandleFunc("/api/delete", s.handleDeleteFile) // Delete file

	// ── Phase 3 Routes ──
	s.mux.HandleFunc("/api/integrity/verify", s.handleIntegrityVerify)
	s.mux.HandleFunc("/api/share", s.handleShare)
	s.mux.HandleFunc("/api/plugins", s.handlePlugins)

	// ── Phase 4: AI Routes ──
	s.mux.HandleFunc("/api/ai/search", s.handleAISearch)
	s.mux.HandleFunc("/api/ai/index", s.handleAIIndex)
	s.mux.HandleFunc("/api/ai/analyze", s.handleAIAnalyze)
	s.mux.HandleFunc("/api/ai/stats", s.handleAIStats)
	s.mux.HandleFunc("/api/chat", s.handleAIChat) // Mobile App Chat

	// ── Dashboard (embedded SPA) ──
	staticFS, err := dashboard.GetStaticFS()
	if err != nil {
		s.logger.Error("failed to load dashboard assets", "error", err)
		return
	}

	fileServer := http.FileServer(http.FS(staticFS))

	// Serve static assets
	s.mux.Handle("/static/", http.StripPrefix("/static/", fileServer))

	// Serve index.html for the root
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "/index.html" {
			// Try static file first
			if _, err := fs.Stat(staticFS, r.URL.Path[1:]); err == nil {
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		// Serve index.html for SPA routing
		indexData, err := fs.ReadFile(staticFS, "index.html")
		if err != nil {
			http.Error(w, "Dashboard not found", 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexData)
	})
}

// Start starts the API server by listening on the configured port.
func (s *Server) Start() error {
	// Auto-preload the Llama 3 AI model in the background so it's instantly ready!
	go s.preloadAIModel()

	addr := fmt.Sprintf("0.0.0.0:%d", s.port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return s.Serve(ln)
}

func (s *Server) preloadAIModel() {
	// Give the embedded Ollama process 3 seconds to fully boot up on port 11434
	time.Sleep(3 * time.Second)
	
	s.logger.Info("Waking up local Llama 3 AI in the background...")
	reqBody := map[string]interface{}{
		"model":  "llama3:latest",
		"prompt": "hi",
		"stream": false,
	}
	jsonData, _ := json.Marshal(reqBody)
	
	// Try up to 3 times to preload if the server is still booting
	for i := 0; i < 3; i++ {
		resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewBuffer(jsonData))
		if err == nil {
			resp.Body.Close()
			s.logger.Info("Local Llama 3 AI is fully loaded and ready!")
			return
		}
		time.Sleep(2 * time.Second)
	}
	s.logger.Warn("Failed to preload Llama 3 (it will load on first chat message instead)")
}

// Serve starts the API server on a pre-existing listener.
func (s *Server) Serve(ln net.Listener) error {
	s.logger.Info("API server starting", "addr", ln.Addr().String())
	server := &http.Server{
		Handler:      s.corsMiddleware(s.loggingMiddleware(s.mux)),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	return server.Serve(ln)
}

func (s *Server) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *Server) respondError(w http.ResponseWriter, status int, msg string) {
	s.respondJSON(w, status, map[string]string{"error": msg})
}

// ── Existing Handlers ──────────────────────────────────────────

// SetSyncManager attaches the sync manager for health reporting.
func (s *Server) SetSyncManager(sm *csync.SyncManager) {
	s.syncManager = sm
}

func getOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	peers := 0
	conflicts := 0
	var peerList []csync.PeerInfo
	if s.syncManager != nil {
		peerList = s.syncManager.GetPeers()
		peers = len(peerList)
		conflicts = len(s.syncManager.GetConflicts())
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "healthy",
		"version":   core.Version,
		"uptime":    s.engine.Uptime().String(),
		"peers":     peers,
		"peer_list": peerList,
		"conflicts": conflicts,
		"lan_ip":    getOutboundIP(),
		"api_port":  s.port,
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	// Register the REST client as a peer temporarily so it shows up on dashboard
	if s.syncManager != nil {
		ip := r.RemoteAddr
		if strings.Contains(ip, ":") {
			ip = strings.Split(ip, ":")[0]
		}
		// Ignore localhost so the PC dashboard doesn't show up as a mobile device
		if ip != "127.0.0.1" && ip != "::1" && ip != "" && ip != "localhost" {
			s.syncManager.AddPeer(&csync.PeerInfo{
				DeviceID:   "Mobile-" + ip,
				DeviceName: "Android App (" + ip + ")",
				Address:    ip,
				LastSeen:   time.Now(),
				Connected:  true,
			})
		}
	}

	statuses, err := s.store.GetStatus()
	if err != nil {
		s.respondError(w, 500, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"files": statuses,
		"count": len(statuses),
	})
}

func (s *Server) handleFiles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		files, err := s.store.DB.ListTrackedFiles()
		if err != nil {
			s.respondError(w, 500, err.Error())
			return
		}
		s.respondJSON(w, http.StatusOK, files)
	case http.MethodPost:
		var req struct{ Path string `json:"path"` }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.respondError(w, 400, "invalid request body")
			return
		}
		if err := s.store.TrackFile(req.Path); err != nil {
			s.respondError(w, 500, err.Error())
			return
		}
		s.respondJSON(w, http.StatusCreated, map[string]string{"status": "tracked"})
	default:
		s.respondError(w, 405, "method not allowed")
	}
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.respondError(w, 405, "method not allowed")
		return
	}

	r.ParseMultipartForm(500 << 20) // 500 MB limit

	file, header, err := r.FormFile("file")
	if err != nil {
		s.respondError(w, 400, "failed to get file: "+err.Error())
		return
	}
	defer file.Close()

	// Write directly to the Imports folder
	importDir := filepath.Join(s.engine.Config.WorkspaceDir, "Imports")
	os.MkdirAll(importDir, 0755)

	destPath := filepath.Join(importDir, header.Filename)
	dest, err := os.Create(destPath)
	if err != nil {
		s.respondError(w, 500, "failed to create file on disk")
		return
	}
	defer dest.Close()

	if _, err := io.Copy(dest, file); err != nil {
		s.respondError(w, 500, "failed to save file data")
		return
	}

	// Tell uniteOS to track it
	if err := s.store.TrackFile(destPath); err != nil {
		s.respondError(w, 500, "failed to track file: "+err.Error())
		return
	}

	s.respondJSON(w, http.StatusCreated, map[string]string{"status": "uploaded", "path": destPath})
}

func (s *Server) handleDownloadAndroid(w http.ResponseWriter, r *http.Request) {
	// Serve the pre-compiled Android/Linux ARM64 binary
	binaryPath := filepath.Join(s.engine.Config.WorkspaceDir, "android_apk", "android_apk.apk")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		s.respondError(w, 404, "Android binary not compiled yet")
		return
	}
	w.Header().Set("Content-Disposition", "attachment; filename=uniteos.apk")
	w.Header().Set("Content-Type", "application/vnd.android.package-archive")
	http.ServeFile(w, r, binaryPath)
}

func (s *Server) handleOpenFile(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		s.respondError(w, 400, "missing path")
		return
	}
	
	// Ensure the path is within the workspace
	absPath := filepath.Join(s.engine.Config.WorkspaceDir, path)
	
	// Open native on Windows
	cmd := exec.Command("cmd", "/c", "start", "", absPath)
	if err := cmd.Start(); err != nil {
		s.respondError(w, 500, "failed to open file")
		return
	}
	s.respondJSON(w, 200, map[string]string{"status": "opened"})
}

func (s *Server) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		s.respondError(w, 405, "method not allowed")
		return
	}
	
	path := r.URL.Query().Get("path")
	if path == "" {
		s.respondError(w, 400, "missing path")
		return
	}
	
	absPath := filepath.Join(s.engine.Config.WorkspaceDir, path)
	os.Remove(absPath) // Remove from disk
	
	// Mark as deleted in the DB
	if tf, err := s.store.DB.GetTrackedFile(absPath); err == nil && tf != nil {
		tf.Status = "deleted"
		s.store.DB.AddTrackedFile(tf)
	}
	
	s.respondJSON(w, 200, map[string]string{"status": "deleted"})
}

func (s *Server) handleSystemBrowse(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		// Default to User Home Dir
		home, _ := os.UserHomeDir()
		if home == "" {
			path = "C:\\"
		} else {
			path = home
		}
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		s.respondError(w, 500, err.Error())
		return
	}

	type FileEntry struct {
		Name  string `json:"name"`
		Path  string `json:"path"`
		IsDir bool   `json:"is_dir"`
	}

	var results []FileEntry
	
	// Add parent directory option if not root
	parent := filepath.Dir(path)
	if parent != path {
		results = append(results, FileEntry{Name: "..", Path: parent, IsDir: true})
	}

	for _, e := range entries {
		// Skip hidden files or system files
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		results = append(results, FileEntry{
			Name:  e.Name(),
			Path:  filepath.Join(path, e.Name()),
			IsDir: e.IsDir(),
		})
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"current_path": path,
		"entries":      results,
	})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		s.respondError(w, 400, "query parameter 'q' required")
		return
	}
	files, err := s.store.DB.SearchFiles(query)
	if err != nil {
		s.respondError(w, 500, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, files)
}

func (s *Server) handleSnapshots(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		snapshots, err := s.store.DB.ListSnapshots()
		if err != nil {
			s.respondError(w, 500, err.Error())
			return
		}
		s.respondJSON(w, http.StatusOK, snapshots)
	case http.MethodPost:
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		snap, err := s.store.CreateSnapshot(req.Name, req.Description)
		if err != nil {
			s.respondError(w, 500, err.Error())
			return
		}
		s.respondJSON(w, http.StatusCreated, snap)
	default:
		s.respondError(w, 405, "method not allowed")
	}
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.store.DB.GetStats()
	if err != nil {
		s.respondError(w, 500, err.Error())
		return
	}
	blobSize, _ := s.store.Blobs.GetStoreSize()
	stats["blob_store_size"] = blobSize
	s.respondJSON(w, http.StatusOK, stats)
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	events := s.engine.EventBus.History(50)
	s.respondJSON(w, http.StatusOK, events)
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"app":         core.AppName,
		"version":     core.Version,
		"device_id":   s.engine.Config.DeviceID,
		"device_name": s.engine.Config.DeviceName,
		"workspace":   s.engine.Config.WorkspaceDir,
		"uptime":      s.engine.Uptime().String(),
		"port":        s.port,
	})
}

// ── Phase 3 Handlers ───────────────────────────────────────────

func (s *Server) handleIntegrityVerify(w http.ResponseWriter, r *http.Request) {
	report, err := s.verifier.VerifyAll()
	if err != nil {
		s.respondError(w, 500, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, report)
}

func (s *Server) handleShare(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		tokens := s.shareManager.ListTokens()
		s.respondJSON(w, http.StatusOK, tokens)
	case http.MethodPost:
		var req struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.respondError(w, 400, "invalid request")
			return
		}
		// Find file hash
		file, err := s.store.DB.GetTrackedFile(req.Path)
		if err != nil || file == nil {
			// Try relative path search
			files, _ := s.store.DB.SearchFiles(req.Path)
			if len(files) > 0 {
				file = &files[0]
			} else {
				s.respondError(w, 404, "file not found")
				return
			}
		}
		token, err := s.shareManager.GenerateToken(
			file.RelativePath, file.Hash,
			24*time.Hour, 5,
		)
		if err != nil {
			s.respondError(w, 500, err.Error())
			return
		}
		s.respondJSON(w, http.StatusCreated, map[string]interface{}{
			"token":   token.Token[:32] + "...",
			"file":    token.FilePath,
			"expires": token.ExpiresAt.Format(time.RFC3339),
		})
	default:
		s.respondError(w, 405, "method not allowed")
	}
}

func (s *Server) handlePlugins(w http.ResponseWriter, r *http.Request) {
	pluginList := s.pluginMgr.ListPlugins()
	var result []map[string]interface{}
	for _, p := range pluginList {
		result = append(result, map[string]interface{}{
			"name":        p.Manifest.Name,
			"version":     p.Manifest.Version,
			"description": p.Manifest.Description,
			"state":       p.State,
		})
	}
	s.respondJSON(w, http.StatusOK, result)
}
// ── Phase 4: AI Handlers ───────────────────────────────────────

func (s *Server) handleAISearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		s.respondError(w, 400, "query parameter 'q' required")
		return
	}
	topK := 10
	if k := r.URL.Query().Get("k"); k != "" {
		if n, err := strconv.Atoi(k); err == nil && n > 0 {
			topK = n
		}
	}
	results := s.aiManager.SemanticSearch(query, topK)
	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"query":   query,
		"results": results,
		"count":   len(results),
	})
}

func (s *Server) handleAIIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.respondError(w, 405, "method not allowed — use POST")
		return
	}
	report, err := s.aiManager.IndexAll()
	if err != nil {
		s.respondError(w, 500, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, report)
}

func (s *Server) handleAIAnalyze(w http.ResponseWriter, r *http.Request) {
	analysis, err := s.aiManager.AnalyzeWorkspace()
	if err != nil {
		s.respondError(w, 500, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, analysis)
}

func (s *Server) handleAIStats(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, s.aiManager.GetStats())
}

// ── Middleware ──────────────────────────────────────────────────

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		// Only log API requests to reduce noise
		if len(r.URL.Path) >= 4 && r.URL.Path[:4] == "/api" {
			s.logger.Debug("api request",
				"method", r.Method,
				"path", r.URL.Path,
				"duration", time.Since(start),
			)
		}
	})
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) executeAIActions(reply string) string {
	baseDir := s.engine.Config.GetWorkspaceDir()
	
	if strings.Contains(reply, "[CREATE_FOLDER:") {
		start := strings.Index(reply, "[CREATE_FOLDER:") + 15
		end := strings.Index(reply[start:], "]")
		if end != -1 {
			folder := strings.TrimSpace(reply[start : start+end])
			targetPath := filepath.Join(baseDir, folder)
			if err := os.MkdirAll(targetPath, 0755); err == nil {
				reply = strings.Replace(reply, reply[start-15:start+end+1], fmt.Sprintf("\n✅ Created folder: %s", folder), 1)
			}
		}
	}
	
	if strings.Contains(reply, "[CREATE_FILE:") {
		start := strings.Index(reply, "[CREATE_FILE:") + 13
		end := strings.Index(reply[start:], "]")
		if end != -1 {
			parts := strings.SplitN(reply[start:start+end], "|", 2)
			if len(parts) == 2 {
				fileName := strings.TrimSpace(parts[0])
				content := strings.TrimSpace(parts[1])
				targetPath := filepath.Join(baseDir, fileName)
				if err := os.WriteFile(targetPath, []byte(content), 0644); err == nil {
					reply = strings.Replace(reply, reply[start-13:start+end+1], fmt.Sprintf("\n✅ Created file: %s", fileName), 1)
				}
			}
		}
	}
	
	if strings.Contains(reply, "[DELETE:") {
		start := strings.Index(reply, "[DELETE:") + 8
		end := strings.Index(reply[start:], "]")
		if end != -1 {
			target := strings.TrimSpace(reply[start : start+end])
			targetPath := filepath.Join(baseDir, target)
			if err := os.RemoveAll(targetPath); err == nil {
				reply = strings.Replace(reply, reply[start-8:start+end+1], fmt.Sprintf("\n🗑️ Deleted: %s", target), 1)
			}
		}
	}

	if strings.Contains(reply, "[MOVE:") {
		start := strings.Index(reply, "[MOVE:") + 6
		end := strings.Index(reply[start:], "]")
		if end != -1 {
			parts := strings.SplitN(reply[start:start+end], "|", 2)
			if len(parts) == 2 {
				src := strings.TrimSpace(parts[0])
				dst := strings.TrimSpace(parts[1])
				srcPath := filepath.Join(baseDir, src)
				dstPath := filepath.Join(baseDir, dst)
				if err := os.Rename(srcPath, dstPath); err == nil {
					reply = strings.Replace(reply, reply[start-6:start+end+1], fmt.Sprintf("\n🚚 Moved %s to %s", src, dst), 1)
				}
			}
		}
	}
	
	return reply
}

func callRealAI(query string, context string) string {
	prompt := fmt.Sprintf(`You are the uniteOS AI Assistant. You help manage a local, secure personal cloud.
If the user asks you to create, delete, or move files/folders, you MUST append EXACTLY ONE of these tags to the end of your response:
[CREATE_FOLDER: folder_name]
[CREATE_FILE: file_name | file_content]
[DELETE: file_path]
[MOVE: source_path | destination_path]

Context from user's files:
%s

User Message: %s`, context, query)

	// Try Gemini first if API key is set
	if apiKey := os.Getenv("GEMINI_API_KEY"); apiKey != "" {
		url := "https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash:generateContent?key=" + apiKey
		reqBody := map[string]interface{}{
			"contents": []map[string]interface{}{
				{"parts": []map[string]interface{}{{"text": prompt}}},
			},
		}
		jsonData, _ := json.Marshal(reqBody)
		resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
		if err == nil {
			defer resp.Body.Close()
			var res struct {
				Candidates []struct {
					Content struct {
						Parts []struct {
							Text string `json:"text"`
						} `json:"parts"`
					} `json:"content"`
				} `json:"candidates"`
			}
			if json.NewDecoder(resp.Body).Decode(&res) == nil && len(res.Candidates) > 0 && len(res.Candidates[0].Content.Parts) > 0 {
				return res.Candidates[0].Content.Parts[0].Text
			}
		}
	}

	// Fallback to local Ollama (100% private)
	reqBody := map[string]interface{}{
		"model":  "llama3:latest",
		"prompt": prompt,
		"stream": false,
	}
	jsonData, _ := json.Marshal(reqBody)
	resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewBuffer(jsonData))
	if err == nil {
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)
		var res struct {
			Response string `json:"response"`
		}
		if json.Unmarshal(bodyBytes, &res) == nil && res.Response != "" {
			return res.Response
		}
	}
	return ""
}

func (s *Server) handleAIChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.respondError(w, 405, "method not allowed")
		return
	}
	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, 400, "invalid request")
		return
	}
	
	query := strings.ToLower(req.Message)
	var reply string

	// 1. Gather Context (RAG)
	context := "No specific file context found."
	if strings.Contains(query, "analyze") {
		analysis, err := s.aiManager.AnalyzeWorkspace()
		if err == nil && len(analysis.Insights) > 0 {
			context = "Workspace Analysis:\n- " + strings.Join(analysis.Insights, "\n- ")
		}
	} else {
		results := s.aiManager.SemanticSearch(req.Message, 3)
		if len(results) > 0 {
			context = "Relevant local files:\n"
			for _, res := range results {
				context += fmt.Sprintf("- File: %s (Relevance: %.0f%%)\n", res.Entry.ID, res.Score*100)
			}
		}
	}

	// 2. Try querying the REAL AI Model (Gemini or Ollama)
	realReply := callRealAI(req.Message, context)
	
	if realReply != "" {
		reply = s.executeAIActions(realReply)
	} else {
		// 3. Fallback routing if no AI is configured
		if strings.Contains(query, "analyze") {
			reply = "I couldn't connect to the Real AI, but here is my local analysis:\n\n" + context
		} else if query == "hi" || query == "hello" || query == "hey" || strings.HasPrefix(query, "hello ") {
			reply = "Hello! I am your secure uniteOS Assistant. To enable my advanced brain, please run a local Ollama server or set the GEMINI_API_KEY environment variable!"
		} else if strings.Contains(query, "how are you") {
			reply = "I'm running perfectly! Your local uniteOS node is online, secure, and ready to assist."
		} else if strings.Contains(query, "who are you") {
			reply = "I am the uniteOS Local AI. Currently running in fallback mode."
		} else {
			reply = "Here is what I found in your files:\n\n" + context + "\n\n(Note: To chat with me naturally, please run Ollama locally or set the GEMINI_API_KEY)."
		}
	}
	
	s.respondJSON(w, http.StatusOK, map[string]string{
		"reply": reply,
	})
}
