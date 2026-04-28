package network

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/ayushgpal/cloudos/internal/core"
	csync "github.com/ayushgpal/cloudos/internal/sync"
)

// MessageType identifies the type of P2P protocol message.
type MessageType string

const (
	MsgHandshake    MessageType = "handshake"
	MsgSyncRequest  MessageType = "sync_request"
	MsgSyncResponse MessageType = "sync_response"
	MsgFileRequest  MessageType = "file_request"
	MsgFileData     MessageType = "file_data"
	MsgDeltaRequest MessageType = "delta_request"
	MsgDeltaData    MessageType = "delta_data"
	MsgPing         MessageType = "ping"
	MsgPong         MessageType = "pong"
)

// Message is the wire protocol message for P2P communication.
type Message struct {
	Type      MessageType     `json:"type"`
	DeviceID  string          `json:"device_id"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

// HandshakePayload is sent when two peers first connect.
type HandshakePayload struct {
	DeviceName string `json:"device_name"`
	Version    string `json:"version"`
	FileCount  int    `json:"file_count"`
	P2PPort    int    `json:"p2p_port,omitempty"`
}

// SyncRequestPayload asks a peer for their file states.
type SyncRequestPayload struct {
	VectorClock csync.VectorClock `json:"vector_clock,omitempty"`
	FullSync    bool              `json:"full_sync"`
}

// SyncResponsePayload contains file states from a peer.
type SyncResponsePayload struct {
	FileStates map[string]*csync.FileState `json:"file_states"`
}

// FileRequestPayload requests a specific file from a peer.
type FileRequestPayload struct {
	FilePath string `json:"file_path"`
	Hash     string `json:"hash"`
}

// FileDataPayload contains a file's content.
type FileDataPayload struct {
	FilePath string `json:"file_path"`
	Hash     string `json:"hash"`
	Size     int64  `json:"size"`
	Data     []byte `json:"data"`
}

// P2PServer handles incoming peer connections.
type P2PServer struct {
	deviceID     string
	deviceName   string
	port         int
	workspaceDir string
	syncManager  *csync.SyncManager
	eventBus     *core.EventBus
	logger       *slog.Logger
	listener     net.Listener
	stopCh       chan struct{}
}

// NewP2PServer creates a new P2P server.
func NewP2PServer(
	deviceID, deviceName string,
	port int,
	workspaceDir string,
	syncManager *csync.SyncManager,
	eventBus *core.EventBus,
	logger *slog.Logger,
) *P2PServer {
	return &P2PServer{
		deviceID:     deviceID,
		deviceName:   deviceName,
		port:         port,
		workspaceDir: workspaceDir,
		syncManager:  syncManager,
		eventBus:     eventBus,
		logger:       logger,
		stopCh:       make(chan struct{}),
	}
}

// Start begins listening for peer connections.
func (s *P2PServer) Start() error {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	s.listener = ln

	go s.acceptLoop()
	s.logger.Info("P2P server started", "port", s.port)
	return nil
}

// Stop shuts down the P2P server.
func (s *P2PServer) Stop() {
	close(s.stopCh)
	if s.listener != nil {
		s.listener.Close()
	}
}

func (s *P2PServer) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.stopCh:
				return
			default:
				s.logger.Error("accept error", "error", err)
				continue
			}
		}
		go s.handleConnection(conn)
	}
}

func (s *P2PServer) handleConnection(conn net.Conn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(60 * time.Second))

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	remoteAddr := conn.RemoteAddr().(*net.TCPAddr).IP.String()

	for {
		var msg Message
		if err := decoder.Decode(&msg); err != nil {
			if err != io.EOF {
				s.logger.Debug("decode error", "error", err)
			}
			return
		}

		// Auto-register peer
		if msg.DeviceID != "" {
			var address string
			var deviceName string = "Peer"
			
			if existingPeer, exists := s.syncManager.GetPeer(msg.DeviceID); exists {
				address = existingPeer.Address
				deviceName = existingPeer.DeviceName
			} else {
				address = remoteAddr
			}

			if msg.Type == MsgHandshake {
				var payload HandshakePayload
				if err := json.Unmarshal(msg.Payload, &payload); err == nil {
					if payload.P2PPort > 0 {
						address = fmt.Sprintf("%s:%d", remoteAddr, payload.P2PPort)
					}
					if payload.DeviceName != "" {
						deviceName = payload.DeviceName
					}
				}
			}
			
			s.syncManager.AddPeer(&csync.PeerInfo{
				DeviceID:   msg.DeviceID,
				DeviceName: deviceName,
				Address:    address,
				LastSeen:   time.Now(),
				Connected:  true,
			})
		}

		response, err := s.handleMessage(&msg)
		if err != nil {
			s.logger.Error("handle message", "type", msg.Type, "error", err)
			return
		}

		if response != nil {
			if err := encoder.Encode(response); err != nil {
				s.logger.Error("encode response", "error", err)
				return
			}
		}
	}
}

func (s *P2PServer) handleMessage(msg *Message) (*Message, error) {
	switch msg.Type {
	case MsgHandshake:
		var payload HandshakePayload
		if err := json.Unmarshal(msg.Payload, &payload); err == nil {
			if p, ok := s.syncManager.GetPeer(msg.DeviceID); ok {
				p.DeviceName = payload.DeviceName
				s.syncManager.AddPeer(p)
			}
		}
		return s.handleHandshake(msg)
	case MsgSyncRequest:
		return s.handleSyncRequest(msg)
	case MsgFileRequest:
		return s.handleFileRequest(msg)
	case MsgPing:
		return s.createMessage(MsgPong, nil)
	default:
		return nil, fmt.Errorf("unknown message type: %s", msg.Type)
	}
}

func (s *P2PServer) handleHandshake(_ *Message) (*Message, error) {
	states := s.syncManager.GetStatesForSync()
	payload := HandshakePayload{
		DeviceName: s.deviceName,
		Version:    core.Version,
		FileCount:  len(states),
	}
	return s.createMessage(MsgHandshake, payload)
}

func (s *P2PServer) handleSyncRequest(msg *Message) (*Message, error) {
	var req SyncRequestPayload
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return nil, err
	}

	var states map[string]*csync.FileState
	if req.FullSync {
		states = s.syncManager.GetStatesForSync()
	} else {
		states = s.syncManager.GetChangedSince(req.VectorClock)
	}

	resp := SyncResponsePayload{FileStates: states}
	return s.createMessage(MsgSyncResponse, resp)
}

func (s *P2PServer) handleFileRequest(msg *Message) (*Message, error) {
	var req FileRequestPayload
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return nil, err
	}

	filePath := filepath.Join(s.workspaceDir, req.FilePath)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	resp := FileDataPayload{
		FilePath: req.FilePath,
		Hash:     req.Hash,
		Size:     int64(len(data)),
		Data:     data,
	}
	return s.createMessage(MsgFileData, resp)
}

func (s *P2PServer) createMessage(msgType MessageType, payload interface{}) (*Message, error) {
	var raw json.RawMessage
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		raw = data
	}
	return &Message{
		Type:      msgType,
		DeviceID:  s.deviceID,
		Timestamp: time.Now(),
		Payload:   raw,
	}, nil
}

// ─── P2P Client ────────────────────────────────────────────────

// P2PClient connects to a peer and performs sync operations.
type P2PClient struct {
	deviceID string
	logger   *slog.Logger
}

// NewP2PClient creates a new P2P client.
func NewP2PClient(deviceID string, logger *slog.Logger) *P2PClient {
	return &P2PClient{deviceID: deviceID, logger: logger}
}

// Connect establishes a TCP connection to a peer.
func (c *P2PClient) Connect(address string, port int) (net.Conn, error) {
	addr := net.JoinHostPort(address, fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", addr, err)
	}
	conn.SetDeadline(time.Now().Add(60 * time.Second))
	return conn, nil
}

// Handshake performs the initial handshake with a peer.
func (c *P2PClient) Handshake(conn net.Conn, deviceName string, myP2PPort int) (*HandshakePayload, error) {
	msg, err := c.createMessage(MsgHandshake, HandshakePayload{
		DeviceName: deviceName,
		Version:    core.Version,
		P2PPort:    myP2PPort,
	})
	if err != nil {
		return nil, err
	}

	resp, err := c.sendAndReceive(conn, msg)
	if err != nil {
		return nil, err
	}

	var payload HandshakePayload
	if err := json.Unmarshal(resp.Payload, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

// RequestSync requests file states from a peer.
func (c *P2PClient) RequestSync(conn net.Conn, fullSync bool, clock csync.VectorClock) (map[string]*csync.FileState, error) {
	msg, err := c.createMessage(MsgSyncRequest, SyncRequestPayload{
		FullSync:    fullSync,
		VectorClock: clock,
	})
	if err != nil {
		return nil, err
	}

	resp, err := c.sendAndReceive(conn, msg)
	if err != nil {
		return nil, err
	}

	var payload SyncResponsePayload
	if err := json.Unmarshal(resp.Payload, &payload); err != nil {
		return nil, err
	}
	return payload.FileStates, nil
}

// RequestFile downloads a file from a peer.
func (c *P2PClient) RequestFile(conn net.Conn, filePath, hash string) ([]byte, error) {
	msg, err := c.createMessage(MsgFileRequest, FileRequestPayload{
		FilePath: filePath,
		Hash:     hash,
	})
	if err != nil {
		return nil, err
	}

	resp, err := c.sendAndReceive(conn, msg)
	if err != nil {
		return nil, err
	}

	var payload FileDataPayload
	if err := json.Unmarshal(resp.Payload, &payload); err != nil {
		return nil, err
	}
	return payload.Data, nil
}

func (c *P2PClient) sendAndReceive(conn net.Conn, msg *Message) (*Message, error) {
	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(msg); err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}

	decoder := json.NewDecoder(conn)
	var resp Message
	if err := decoder.Decode(&resp); err != nil {
		return nil, fmt.Errorf("receive: %w", err)
	}
	return &resp, nil
}

func (c *P2PClient) createMessage(msgType MessageType, payload interface{}) (*Message, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &Message{
		Type:      msgType,
		DeviceID:  c.deviceID,
		Timestamp: time.Now(),
		Payload:   json.RawMessage(data),
	}, nil
}
