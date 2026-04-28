// Package network provides LAN discovery and P2P communication for UNITEos.
package network

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/ayushgpal/uniteos/internal/core"
)

const (
	// UNITEos discovery beacon port (UDP broadcast)
	discoveryPort = 7891
	// Beacon interval
	beaconInterval = 10 * time.Second
	// Peer timeout
	peerTimeout = 30 * time.Second
)

// DiscoveredPeer represents a device found on the local network.
type DiscoveredPeer struct {
	DeviceID   string    `json:"device_id"`
	DeviceName string    `json:"device_name"`
	Address    string    `json:"address"`
	Port       int       `json:"port"`
	Version    string    `json:"version"`
	LastSeen   time.Time `json:"last_seen"`
}

// Beacon is the UDP broadcast message for device discovery.
type Beacon struct {
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
	Port       int    `json:"port"` // P2P service port
	Version    string `json:"version"`
}

// Discovery handles LAN-based peer discovery using UDP broadcast.
type Discovery struct {
	deviceID   string
	deviceName string
	servicePort int // Port where P2P server listens
	eventBus   *core.EventBus
	logger     *slog.Logger

	peers   map[string]*DiscoveredPeer
	peersMu sync.RWMutex

	onPeerFound func(DiscoveredPeer)
	onPeerLost  func(string)

	conn   *net.UDPConn
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewDiscovery creates a new LAN discovery service.
func NewDiscovery(deviceID, deviceName string, servicePort int, eventBus *core.EventBus, logger *slog.Logger) *Discovery {
	return &Discovery{
		deviceID:    deviceID,
		deviceName:  deviceName,
		servicePort: servicePort,
		eventBus:    eventBus,
		logger:      logger,
		peers:       make(map[string]*DiscoveredPeer),
		stopCh:      make(chan struct{}),
	}
}

// OnPeerFound sets the callback for when a new peer is discovered.
func (d *Discovery) OnPeerFound(fn func(DiscoveredPeer)) {
	d.onPeerFound = fn
}

// OnPeerLost sets the callback for when a peer disappears.
func (d *Discovery) OnPeerLost(fn func(string)) {
	d.onPeerLost = fn
}

// Start begins broadcasting and listening for peers on the LAN.
func (d *Discovery) Start(ctx context.Context) error {
	addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf(":%d", discoveryPort))
	if err != nil {
		return fmt.Errorf("resolve address: %w", err)
	}

	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return fmt.Errorf("listen UDP: %w", err)
	}
	d.conn = conn

	d.wg.Add(3)
	go d.broadcastLoop(ctx)
	go d.listenLoop(ctx)
	go d.cleanupLoop(ctx)

	d.logger.Info("LAN discovery started",
		"device_id", d.deviceID[:12],
		"port", discoveryPort,
		"service_port", d.servicePort,
	)
	return nil
}

// Stop shuts down the discovery service.
func (d *Discovery) Stop() {
	close(d.stopCh)
	if d.conn != nil {
		d.conn.Close()
	}
	d.wg.Wait()
	d.logger.Info("LAN discovery stopped")
}

// broadcastLoop sends periodic beacon messages to the LAN.
func (d *Discovery) broadcastLoop(ctx context.Context) {
	defer d.wg.Done()
	ticker := time.NewTicker(beaconInterval)
	defer ticker.Stop()

	// Send initial beacon immediately
	d.sendBeacon()

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stopCh:
			return
		case <-ticker.C:
			d.sendBeacon()
		}
	}
}

// sendBeacon broadcasts a discovery beacon to the LAN.
func (d *Discovery) sendBeacon() {
	beacon := Beacon{
		DeviceID:   d.deviceID,
		DeviceName: d.deviceName,
		Port:       d.servicePort,
		Version:    core.Version,
	}

	data, err := json.Marshal(beacon)
	if err != nil {
		d.logger.Error("marshal beacon", "error", err)
		return
	}

	// Broadcast to all interfaces
	interfaces, err := net.Interfaces()
	if err != nil {
		return
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagBroadcast == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP.To4() == nil {
				continue
			}

			// Calculate broadcast address
			broadcast := broadcastAddress(ipNet)
			destAddr := &net.UDPAddr{IP: broadcast, Port: discoveryPort}

			if d.conn != nil {
				d.conn.WriteToUDP(data, destAddr)
			}
		}
	}
}

// listenLoop listens for beacon messages from other devices.
func (d *Discovery) listenLoop(ctx context.Context) {
	defer d.wg.Done()
	buf := make([]byte, 4096)

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stopCh:
			return
		default:
		}

		if d.conn == nil {
			return
		}

		d.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, remoteAddr, err := d.conn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			select {
			case <-d.stopCh:
				return
			default:
				continue
			}
		}

		var beacon Beacon
		if err := json.Unmarshal(buf[:n], &beacon); err != nil {
			continue
		}

		// Ignore our own beacons
		if beacon.DeviceID == d.deviceID {
			continue
		}

		d.handleBeacon(beacon, remoteAddr)
	}
}

// handleBeacon processes a received beacon from a peer.
func (d *Discovery) handleBeacon(beacon Beacon, addr *net.UDPAddr) {
	d.peersMu.Lock()
	defer d.peersMu.Unlock()

	peer := &DiscoveredPeer{
		DeviceID:   beacon.DeviceID,
		DeviceName: beacon.DeviceName,
		Address:    addr.IP.String(),
		Port:       beacon.Port,
		Version:    beacon.Version,
		LastSeen:   time.Now(),
	}

	_, existed := d.peers[beacon.DeviceID]
	d.peers[beacon.DeviceID] = peer

	if !existed {
		d.logger.Info("new peer discovered",
			"device_id", beacon.DeviceID[:12],
			"name", beacon.DeviceName,
			"address", addr.IP.String(),
		)

		d.eventBus.Publish(core.NewEvent(core.EventDeviceDetected, "discovery", map[string]interface{}{
			"device_id":   beacon.DeviceID,
			"device_name": beacon.DeviceName,
			"address":     addr.IP.String(),
			"port":        beacon.Port,
		}))

		if d.onPeerFound != nil {
			go d.onPeerFound(*peer)
		}
	}
}

// cleanupLoop removes stale peers.
func (d *Discovery) cleanupLoop(ctx context.Context) {
	defer d.wg.Done()
	ticker := time.NewTicker(peerTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stopCh:
			return
		case <-ticker.C:
			d.cleanupPeers()
		}
	}
}

func (d *Discovery) cleanupPeers() {
	d.peersMu.Lock()
	defer d.peersMu.Unlock()

	now := time.Now()
	for id, peer := range d.peers {
		if now.Sub(peer.LastSeen) > peerTimeout {
			delete(d.peers, id)
			d.logger.Info("peer lost", "device_id", id[:12], "name", peer.DeviceName)
			d.eventBus.Publish(core.NewEvent(core.EventDeviceLost, "discovery", map[string]interface{}{
				"device_id": id,
			}))
			if d.onPeerLost != nil {
				go d.onPeerLost(id)
			}
		}
	}
}

// GetPeers returns all currently known peers.
func (d *Discovery) GetPeers() []DiscoveredPeer {
	d.peersMu.RLock()
	defer d.peersMu.RUnlock()
	peers := make([]DiscoveredPeer, 0, len(d.peers))
	for _, p := range d.peers {
		peers = append(peers, *p)
	}
	return peers
}

// broadcastAddress calculates the broadcast address for a network.
func broadcastAddress(ipNet *net.IPNet) net.IP {
	ip := ipNet.IP.To4()
	mask := ipNet.Mask
	broadcast := make(net.IP, len(ip))
	for i := range ip {
		broadcast[i] = ip[i] | ^mask[i]
	}
	return broadcast
}
