package network

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/freeeakn/AetherWave/pkg/blockchain"
)

// PeerInfo содержит информацию о пире
type PeerInfo struct {
	Address     string
	LastSeen    time.Time
	Active      bool
	FailedPings int
}

// NetworkMessage представляет сообщение, передаваемое по сети
type NetworkMessage struct {
	Type    string
	Payload json.RawMessage
}

// Node представляет узел в сети
type Node struct {
	Address        string
	Peers          map[string]net.Conn
	KnownPeers     map[string]PeerInfo
	Blockchain     *blockchain.Blockchain
	mutex          sync.RWMutex
	bootstrapPeers []string
	listener       net.Listener
	shutdown       chan struct{}
	isRunning      bool
	discovery      *MDNSDiscovery
	useDiscovery   bool
}

// Константы для настройки сетевого взаимодействия
const (
	DialTimeout           = 5 * time.Second
	WriteTimeout          = 2 * time.Second
	PeerBroadcastInterval = 10 * time.Second
	PeerProbeInterval     = 30 * time.Second
	PeerTimeout           = 15 * time.Second
	BroadcastRetryDelay   = 1 * time.Second
	MaxBroadcastRetries   = 3
	MaxFailedPings        = 3
)

// DialPeer - функция для установки соединения с пиром
// Экспортирована для возможности замены в тестах
var DialPeer = func(address string) (net.Conn, error) {
	return net.DialTimeout("tcp", address, DialTimeout)
}

// NewNode создает новый экземпляр узла
func NewNode(address string, bc *blockchain.Blockchain, bootstrapPeers []string) *Node {
	return &Node{
		Address:        address,
		Peers:          make(map[string]net.Conn),
		KnownPeers:     make(map[string]PeerInfo),
		Blockchain:     bc,
		bootstrapPeers: bootstrapPeers,
		shutdown:       make(chan struct{}),
		useDiscovery:   false,
	}
}

// Start запускает узел и начинает прослушивание входящих соединений
func (n *Node) Start() error {
	if n.isRunning {
		return fmt.Errorf("node is already running")
	}

	ln, err := net.Listen("tcp", n.Address)
	if err != nil {
		return fmt.Errorf("error starting node: %v", err)
	}
	n.listener = ln
	n.isRunning = true

	fmt.Printf("Node %s started, listening on %s\n", n.Address, n.Address)

	n.mutex.Lock()
	n.KnownPeers[n.Address] = PeerInfo{Address: n.Address, LastSeen: time.Now(), Active: true}
	n.mutex.Unlock()

	// Запускаем автоматическое обнаружение узлов, если оно включено
	if n.useDiscovery {
		n.discovery = NewMDNSDiscovery(n)
		if err := n.discovery.Start(); err != nil {
			fmt.Printf("Warning: Failed to start mDNS discovery: %v\n", err)
		}
	}

	go n.bootstrapDiscovery()
	go n.broadcastPeerList()
	go n.probePeers()
	go n.acceptConnections()

	return nil
}

// Stop останавливает узел и закрывает все соединения
func (n *Node) Stop() {
	if !n.isRunning {
		return
	}

	close(n.shutdown)
	if n.listener != nil {
		n.listener.Close()
	}

	// Останавливаем сервис обнаружения, если он запущен
	if n.discovery != nil {
		n.discovery.Stop()
	}

	n.mutex.Lock()
	defer n.mutex.Unlock()

	for addr, conn := range n.Peers {
		conn.Close()
		delete(n.Peers, addr)
	}
	n.isRunning = false
	fmt.Printf("Node %s stopped\n", n.Address)
}

// acceptConnections обрабатывает входящие соединения
func (n *Node) acceptConnections() {
	for {
		select {
		case <-n.shutdown:
			return
		default:
			conn, err := n.listener.Accept()
			if err != nil {
				select {
				case <-n.shutdown:
					return
				default:
					fmt.Println("Error accepting connection:", err)
					time.Sleep(100 * time.Millisecond)
					continue
				}
			}
			remoteAddr := conn.RemoteAddr().String()
			fmt.Printf("Node %s accepted connection from %s\n", n.Address, remoteAddr)
			go n.handleConnection(conn, remoteAddr)
		}
	}
}

// bootstrapDiscovery подключается к начальным пирам
func (n *Node) bootstrapDiscovery() {
	for _, peerAddr := range n.bootstrapPeers {
		if peerAddr != n.Address {
			n.ConnectToPeer(peerAddr)
		}
	}
}

// ConnectToPeer устанавливает соединение с пиром
func (n *Node) ConnectToPeer(peerAddress string) error {
	if peerAddress == n.Address {
		return nil
	}

	n.mutex.RLock()
	if info, exists := n.KnownPeers[peerAddress]; exists && info.Active {
		n.mutex.RUnlock()
		return nil
	}
	n.mutex.RUnlock()

	conn, err := DialPeer(peerAddress)
	if err != nil {
		fmt.Printf("Node %s failed to connect to %s: %v\n", n.Address, peerAddress, err)
		n.updatePeerStatus(peerAddress, false, true)
		return err
	}

	n.mutex.Lock()
	n.Peers[peerAddress] = conn
	n.KnownPeers[peerAddress] = PeerInfo{Address: peerAddress, LastSeen: time.Now(), Active: true, FailedPings: 0}
	n.mutex.Unlock()

	fmt.Printf("Node %s connected to peer %s\n", n.Address, peerAddress)
	n.requestChain(peerAddress)
	go n.handleConnection(conn, peerAddress)
	return nil
}

// handleConnection обрабатывает соединение с пиром
func (n *Node) handleConnection(conn net.Conn, initialRemoteAddr string) {
	defer conn.Close()
	decoder := json.NewDecoder(conn)

	// Начинаем с заполнителя; уточним его с адресом прослушивания
	peerAddr := ""

	for {
		var msg NetworkMessage
		if err := decoder.Decode(&msg); err != nil {
			if err.Error() != "EOF" {
				fmt.Printf("Node %s error decoding message from %s: %v\n", n.Address, initialRemoteAddr, err)
			}
			if peerAddr != "" {
				n.updatePeerStatus(peerAddr, false, true)
				n.disconnectPeer(peerAddr)
			}
			return
		}

		// Используем peer_list для идентификации адреса прослушивания
		if msg.Type == "peer_list" {
			var peers []string
			if err := json.Unmarshal(msg.Payload, &peers); err == nil {
				for _, addr := range peers {
					if addr != n.Address && !isEphemeralPort(addr) {
						peerAddr = addr // Предполагаем, что это адрес прослушивания отправителя
						n.mutex.Lock()
						if _, exists := n.Peers[peerAddr]; !exists {
							n.Peers[peerAddr] = conn
							fmt.Printf("Node %s added %s to Peers from peer_list\n", n.Address, peerAddr)
						}
						n.mutex.Unlock()
						break
					}
				}
			}
		}

		// Используем начальный адрес, если еще не идентифицирован
		if peerAddr == "" {
			peerAddr = initialRemoteAddr
		}

		n.updatePeerStatus(peerAddr, true, false)
		fmt.Printf("Node %s received message type %s from %s\n", n.Address, msg.Type, peerAddr)

		switch msg.Type {
		case "peer_list":
			var peers []string
			if err := json.Unmarshal(msg.Payload, &peers); err != nil {
				fmt.Println("Error unmarshaling peer list:", err)
				continue
			}
			n.handlePeerList(peers)
		case "new_block":
			var block blockchain.Block
			if err := json.Unmarshal(msg.Payload, &block); err != nil {
				fmt.Println("Error unmarshaling new block:", err)
				continue
			}
			n.handleNewBlock(block)
		case "chain_request":
			n.sendChain(conn)
		case "chain_response":
			var chain []blockchain.Block
			if err := json.Unmarshal(msg.Payload, &chain); err != nil {
				fmt.Println("Error unmarshaling chain response:", err)
				continue
			}
			n.handleChainResponse(chain)
		case "ping":
			n.sendPong(conn)
		case "pong":
			n.updatePeerStatus(peerAddr, true, false)
		}
	}
}

// broadcastPeerList периодически рассылает список известных пиров
func (n *Node) broadcastPeerList() {
	ticker := time.NewTicker(PeerBroadcastInterval)
	defer ticker.Stop()

	for {
		select {
		case <-n.shutdown:
			return
		case <-ticker.C:
			n.mutex.RLock()
			activePeers := make([]string, 0, len(n.KnownPeers))
			for addr, info := range n.KnownPeers {
				if info.Active && addr != n.Address {
					activePeers = append(activePeers, addr)
				}
			}
			n.mutex.RUnlock()

			if len(activePeers) > 0 {
				msg := NetworkMessage{
					Type:    "peer_list",
					Payload: mustMarshal(activePeers),
				}
				n.broadcastMessage(msg)
			}
		}
	}
}

// probePeers периодически проверяет доступность пиров
func (n *Node) probePeers() {
	ticker := time.NewTicker(PeerProbeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-n.shutdown:
			return
		case <-ticker.C:
			n.mutex.RLock()
			peers := make([]string, 0, len(n.Peers))
			for addr := range n.Peers {
				if addr != n.Address {
					peers = append(peers, addr)
				}
			}
			n.mutex.RUnlock()

			for _, addr := range peers {
				n.sendPing(addr)
			}

			n.mutex.Lock()
			now := time.Now()
			for addr, info := range n.KnownPeers {
				if addr == n.Address {
					continue
				}
				if info.Active && now.Sub(info.LastSeen) > PeerTimeout {
					n.KnownPeers[addr] = PeerInfo{
						Address:     info.Address,
						LastSeen:    info.LastSeen,
						Active:      info.Active,
						FailedPings: info.FailedPings + 1,
					}
					fmt.Printf("Node %s recorded failed ping for %s (count: %d)\n", n.Address, addr, n.KnownPeers[addr].FailedPings)
					if n.KnownPeers[addr].FailedPings >= MaxFailedPings {
						fmt.Printf("Node %s marking peer %s as inactive after %d failed pings (last seen: %v)\n", n.Address, addr, MaxFailedPings, info.LastSeen)
						n.KnownPeers[addr] = PeerInfo{Address: addr, LastSeen: info.LastSeen, Active: false, FailedPings: info.FailedPings}
						n.mutex.Unlock()
						n.disconnectPeer(addr)
						n.mutex.Lock()
					}
				}
			}
			n.mutex.Unlock()
		}
	}
}

// sendPing отправляет ping-сообщение пиру
func (n *Node) sendPing(peerAddress string) {
	msg := NetworkMessage{
		Type:    "ping",
		Payload: nil,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		fmt.Printf("Node %s error marshaling ping: %v\n", n.Address, err)
		return
	}
	data = append(data, '\n')

	n.mutex.RLock()
	conn, exists := n.Peers[peerAddress]
	n.mutex.RUnlock()
	if !exists {
		n.updatePeerStatus(peerAddress, false, true)
		return
	}

	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetWriteDeadline(time.Now().Add(WriteTimeout))
	}
	if _, err := conn.Write(data); err != nil {
		fmt.Printf("Node %s error sending ping to %s: %v\n", n.Address, peerAddress, err)
		n.updatePeerStatus(peerAddress, false, true)
		n.disconnectPeer(peerAddress)
	}
}

// sendPong отправляет pong-сообщение в ответ на ping
func (n *Node) sendPong(conn net.Conn) {
	msg := NetworkMessage{
		Type:    "pong",
		Payload: nil,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		fmt.Printf("Node %s error marshaling pong: %v\n", n.Address, err)
		return
	}
	data = append(data, '\n')

	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetWriteDeadline(time.Now().Add(WriteTimeout))
	}
	conn.Write(data)
}

// broadcastMessage рассылает сообщение всем пирам
func (n *Node) broadcastMessage(msg NetworkMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		fmt.Printf("Node %s error marshaling message: %v\n", n.Address, err)
		return
	}
	data = append(data, '\n')

	n.mutex.RLock()
	peers := make(map[string]net.Conn, len(n.Peers))
	for addr, conn := range n.Peers {
		peers[addr] = conn
	}
	n.mutex.RUnlock()

	if len(peers) == 0 {
		fmt.Printf("Node %s has no peers to broadcast %s to\n", n.Address, msg.Type)
		return
	}

	for addr, conn := range peers {
		if tcpConn, ok := conn.(*net.TCPConn); ok {
			tcpConn.SetWriteDeadline(time.Now().Add(WriteTimeout))
		}
		if _, err := conn.Write(data); err != nil {
			fmt.Printf("Node %s error sending %s to %s: %v\n", n.Address, msg.Type, addr, err)
			n.updatePeerStatus(addr, false, true)
			n.disconnectPeer(addr)
			continue
		}
		fmt.Printf("Node %s sent %s to %s\n", n.Address, msg.Type, addr)
	}
	if msg.Type != "ping" && msg.Type != "pong" {
		fmt.Printf("Node %s completed broadcasting %s\n", n.Address, msg.Type)
	}
}

// disconnectPeer отключает пира
func (n *Node) disconnectPeer(peerAddress string) {
	n.mutex.Lock()
	if conn, exists := n.Peers[peerAddress]; exists {
		conn.Close()
		delete(n.Peers, peerAddress)
	}
	n.mutex.Unlock()
}

// handlePeerList обрабатывает полученный список пиров
func (n *Node) handlePeerList(peers []string) {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	for _, addr := range peers {
		if _, _, err := net.SplitHostPort(addr); err == nil && !isEphemeralPort(addr) && addr != n.Address {
			if _, exists := n.KnownPeers[addr]; !exists {
				n.KnownPeers[addr] = PeerInfo{Address: addr, LastSeen: time.Now(), Active: false, FailedPings: 0}
				go n.ConnectToPeer(addr)
			}
		}
	}
}

// isEphemeralPort проверяет, является ли порт эфемерным
func isEphemeralPort(addr string) bool {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	p, err := net.LookupPort("tcp", port)
	if err != nil {
		return false
	}
	return p > 49152
}

// updatePeerStatus обновляет статус пира
func (n *Node) updatePeerStatus(peerAddress string, active bool, failedPing bool) {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	if info, exists := n.KnownPeers[peerAddress]; exists {
		failedPings := info.FailedPings
		if failedPing {
			failedPings++
		} else {
			failedPings = 0
		}
		n.KnownPeers[peerAddress] = PeerInfo{
			Address:     info.Address,
			LastSeen:    time.Now(),
			Active:      active,
			FailedPings: failedPings,
		}
	} else if !isEphemeralPort(peerAddress) {
		failedPings := 0
		if failedPing {
			failedPings = 1
		}
		n.KnownPeers[peerAddress] = PeerInfo{
			Address:     peerAddress,
			LastSeen:    time.Now(),
			Active:      active,
			FailedPings: failedPings,
		}
	}
}

// handleNewBlock обрабатывает новый блок, полученный от пира
func (n *Node) handleNewBlock(block blockchain.Block) {
	lastBlock := n.Blockchain.GetLastBlock()
	fmt.Printf("Node %s processing new block %d (PrevHash: %s, Expected: %s)\n",
		n.Address, block.Index, block.PrevHash, lastBlock.Hash)

	calculatedHash := blockchain.SimpleCalculateHash(block)
	if block.Hash != calculatedHash {
		fmt.Printf("Node %s rejected block %d: invalid hash (Expected: %s, Got: %s)\n",
			n.Address, block.Index, calculatedHash, block.Hash)
		return
	}

	if block.Index <= lastBlock.Index {
		fmt.Printf("Node %s ignored block %d: already have block at index %d or earlier\n",
			n.Address, block.Index, lastBlock.Index)
		return
	}

	if block.Index == lastBlock.Index+1 && block.PrevHash == lastBlock.Hash {
		chain := n.Blockchain.GetChain()
		chain = append(chain, block)
		if n.Blockchain.UpdateChain(chain) {
			fmt.Printf("Node %s accepted new block %d\n", n.Address, block.Index)
			go n.BroadcastBlockWithRetry(block)
		}
	} else {
		fmt.Printf("Node %s out of sync for block %d (index %d vs %d, prevHash mismatch)\n",
			n.Address, block.Index, block.Index, lastBlock.Index+1)
		n.requestChainFromPeers()
	}
}

// BroadcastBlock рассылает новый блок всем пирам
func (n *Node) BroadcastBlock(block blockchain.Block) {
	msg := NetworkMessage{
		Type:    "new_block",
		Payload: mustMarshal(block),
	}
	n.broadcastMessage(msg)
}

// BroadcastBlockWithRetry рассылает новый блок всем пирам с повторными попытками
func (n *Node) BroadcastBlockWithRetry(block blockchain.Block) {
	msg := NetworkMessage{
		Type:    "new_block",
		Payload: mustMarshal(block),
	}
	n.broadcastMessageWithRetry(msg, MaxBroadcastRetries)
}

// broadcastMessageWithRetry рассылает сообщение с повторными попытками
func (n *Node) broadcastMessageWithRetry(msg NetworkMessage, retries int) {
	data, err := json.Marshal(msg)
	if err != nil {
		fmt.Printf("Node %s error marshaling message: %v\n", n.Address, err)
		return
	}
	data = append(data, '\n')

	n.mutex.RLock()
	peers := make(map[string]net.Conn, len(n.Peers))
	for addr, conn := range n.Peers {
		peers[addr] = conn
	}
	n.mutex.RUnlock()

	if len(peers) == 0 {
		fmt.Printf("Node %s has no peers to broadcast %s to\n", n.Address, msg.Type)
		return
	}

	failedPeers := make(map[string]bool)
	for attempt := 0; attempt <= retries; attempt++ {
		for addr, conn := range peers {
			if failedPeers[addr] {
				continue
			}
			if tcpConn, ok := conn.(*net.TCPConn); ok {
				tcpConn.SetWriteDeadline(time.Now().Add(WriteTimeout))
			}
			if _, err := conn.Write(data); err != nil {
				fmt.Printf("Node %s error sending %s to %s (attempt %d/%d): %v\n", n.Address, msg.Type, addr, attempt+1, retries+1, err)
				failedPeers[addr] = true
				if attempt == retries {
					n.updatePeerStatus(addr, false, true)
					n.disconnectPeer(addr)
				}
				continue
			}
			fmt.Printf("Node %s sent %s to %s (attempt %d/%d)\n", n.Address, msg.Type, addr, attempt+1, retries+1)
			delete(failedPeers, addr)
		}
		if len(failedPeers) == 0 {
			break
		}
		if attempt < retries {
			fmt.Printf("Node %s retrying broadcast to %d failed peers after %v\n", n.Address, len(failedPeers), BroadcastRetryDelay)
			time.Sleep(BroadcastRetryDelay)
		}
	}
	if len(failedPeers) > 0 {
		fmt.Printf("Node %s failed to broadcast %s to %d peers after %d retries\n", n.Address, msg.Type, len(failedPeers), retries+1)
	} else {
		fmt.Printf("Node %s completed broadcasting %s to all peers\n", n.Address, msg.Type)
	}
}

// requestChain запрашивает цепочку блоков у пира
func (n *Node) requestChain(peerAddress string) {
	msg := NetworkMessage{
		Type:    "chain_request",
		Payload: nil,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		fmt.Printf("Node %s error marshaling chain request: %v\n", n.Address, err)
		return
	}
	data = append(data, '\n')

	n.mutex.RLock()
	if conn, exists := n.Peers[peerAddress]; exists {
		if tcpConn, ok := conn.(*net.TCPConn); ok {
			tcpConn.SetWriteDeadline(time.Now().Add(WriteTimeout))
		}
		conn.Write(data)
		fmt.Printf("Node %s requested chain from %s\n", n.Address, peerAddress)
	}
	n.mutex.RUnlock()
}

// requestChainFromPeers запрашивает цепочку блоков у всех пиров
func (n *Node) requestChainFromPeers() {
	n.mutex.RLock()
	peers := make([]string, 0, len(n.Peers))
	for addr := range n.Peers {
		peers = append(peers, addr)
	}
	n.mutex.RUnlock()

	for _, peer := range peers {
		n.requestChain(peer)
	}
}

// sendChain отправляет цепочку блоков пиру
func (n *Node) sendChain(conn net.Conn) {
	chain := n.Blockchain.GetChain()

	msg := NetworkMessage{
		Type:    "chain_response",
		Payload: mustMarshal(chain),
	}
	data, err := json.Marshal(msg)
	if err != nil {
		fmt.Printf("Node %s error marshaling chain response: %v\n", n.Address, err)
		return
	}
	data = append(data, '\n')
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetWriteDeadline(time.Now().Add(WriteTimeout))
	}
	conn.Write(data)
	fmt.Printf("Node %s sent chain (length %d) to %s\n", n.Address, len(chain), conn.RemoteAddr().String())
}

// handleChainResponse обрабатывает полученную цепочку блоков
func (n *Node) handleChainResponse(chain []blockchain.Block) {
	if n.Blockchain.UpdateChain(chain) {
		fmt.Printf("Node %s updated chain to length %d\n", n.Address, len(chain))
	} else {
		fmt.Printf("Node %s rejected chain response: invalid chain or not longer than current\n", n.Address)
	}
}

// GetPeers возвращает список активных пиров
func (n *Node) GetPeers() []PeerInfo {
	n.mutex.RLock()
	defer n.mutex.RUnlock()

	peers := make([]PeerInfo, 0, len(n.KnownPeers))
	for _, info := range n.KnownPeers {
		peers = append(peers, info)
	}
	return peers
}

// mustMarshal маршалит данные и паникует при ошибке
func mustMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		fmt.Printf("Error marshaling: %v\n", err)
		return nil
	}
	return data
}

// EnableDiscovery включает автоматическое обнаружение узлов
func (n *Node) EnableDiscovery() {
	n.useDiscovery = true
	if n.isRunning && n.discovery == nil {
		n.discovery = NewMDNSDiscovery(n)
		if err := n.discovery.Start(); err != nil {
			fmt.Printf("Warning: Failed to start mDNS discovery: %v\n", err)
		}
	}
}

// DisableDiscovery выключает автоматическое обнаружение узлов
func (n *Node) DisableDiscovery() {
	n.useDiscovery = false
	if n.discovery != nil {
		n.discovery.Stop()
		n.discovery = nil
	}
}

// GetDiscoveredNodes возвращает список узлов, обнаруженных через mDNS
func (n *Node) GetDiscoveredNodes() []string {
	if n.discovery != nil {
		return n.discovery.GetDiscoveredNodes()
	}
	return []string{}
}
