package main

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"
)

type PeerInfo struct {
	Address  string
	LastSeen time.Time
	Active   bool
}

type NetworkMessage struct {
	Type    string
	Payload json.RawMessage
}

type Node struct {
	Address         string
	Peers           map[string]net.Conn
	KnownPeers      map[string]PeerInfo
	Blockchain      *Blockchain
	mutex           sync.Mutex
	blockchainMutex sync.Mutex
	bootstrapPeers  []string
}

const (
	DialTimeout           = 5 * time.Second
	WriteTimeout          = 2 * time.Second
	PeerBroadcastInterval = 10 * time.Second
	PeerProbeInterval     = 30 * time.Second
	PeerTimeout           = 15 * time.Second
	BroadcastRetryDelay   = 1 * time.Second
	MaxBroadcastRetries   = 3
)

func NewNode(address string, bc *Blockchain, bootstrapPeers []string) *Node {
	return &Node{
		Address:        address,
		Peers:          make(map[string]net.Conn),
		KnownPeers:     make(map[string]PeerInfo),
		Blockchain:     bc,
		bootstrapPeers: bootstrapPeers,
	}
}

func (n *Node) Start() {
	ln, err := net.Listen("tcp", n.Address)
	if err != nil {
		fmt.Println("Error starting node:", err)
		return
	}
	fmt.Printf("Node %s started, listening on %s\n", n.Address, n.Address)

	n.mutex.Lock()
	n.KnownPeers[n.Address] = PeerInfo{Address: n.Address, LastSeen: time.Now(), Active: true}
	n.mutex.Unlock()

	go n.bootstrapDiscovery()
	go n.broadcastPeerList()
	go n.probePeers()

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}
		fmt.Printf("Node %s accepted connection from %s\n", n.Address, conn.RemoteAddr().String())
		go n.handleConnection(conn)
	}
}

func (n *Node) bootstrapDiscovery() {
	for _, peerAddr := range n.bootstrapPeers {
		if peerAddr != n.Address {
			n.ConnectToPeer(peerAddr)
		}
	}
}

func (n *Node) ConnectToPeer(peerAddress string) error {
	if peerAddress == n.Address {
		return nil
	}

	n.mutex.Lock()
	if _, exists := n.KnownPeers[peerAddress]; exists && n.KnownPeers[peerAddress].Active {
		n.mutex.Unlock()
		return nil
	}
	n.mutex.Unlock()

	conn, err := net.DialTimeout("tcp", peerAddress, DialTimeout)
	if err != nil {
		fmt.Printf("Node %s failed to connect to %s: %v\n", n.Address, peerAddress, err)
		n.updatePeerStatus(peerAddress, false)
		return err
	}

	n.mutex.Lock()
	n.Peers[peerAddress] = conn
	n.KnownPeers[peerAddress] = PeerInfo{Address: peerAddress, LastSeen: time.Now(), Active: true}
	n.mutex.Unlock()

	fmt.Printf("Node %s connected to peer %s\n", n.Address, peerAddress)
	n.requestChain(peerAddress)
	go n.handleConnection(conn)
	return nil
}

func (n *Node) handleConnection(conn net.Conn) {
	defer conn.Close()
	decoder := json.NewDecoder(conn)
	remoteAddr := conn.RemoteAddr().String()

	for {
		var msg NetworkMessage
		if err := decoder.Decode(&msg); err != nil {
			if err.Error() != "EOF" {
				fmt.Printf("Node %s error decoding message from %s: %v\n", n.Address, remoteAddr, err)
			}
			n.updatePeerStatus(remoteAddr, false)
			return
		}

		n.updatePeerStatus(remoteAddr, true)
		fmt.Printf("Node %s received message type %s from %s\n", n.Address, msg.Type, remoteAddr)

		switch msg.Type {
		case "peer_list":
			var peers []string
			if err := json.Unmarshal(msg.Payload, &peers); err != nil {
				fmt.Println("Error unmarshaling peer list:", err)
				continue
			}
			n.handlePeerList(peers)
		case "new_block":
			var block Block
			if err := json.Unmarshal(msg.Payload, &block); err != nil {
				fmt.Println("Error unmarshaling new block:", err)
				continue
			}
			n.handleNewBlock(block)
		case "chain_request":
			n.sendChain(conn)
		case "chain_response":
			var chain []Block
			if err := json.Unmarshal(msg.Payload, &chain); err != nil {
				fmt.Println("Error unmarshaling chain response:", err)
				continue
			}
			n.handleChainResponse(chain)
		case "ping":
			n.sendPong(conn)
		case "pong":
			n.updatePeerStatus(remoteAddr, true)
		}
	}
}

func (n *Node) broadcastPeerList() {
	for range time.Tick(PeerBroadcastInterval) {
		n.mutex.Lock()
		activePeers := make([]string, 0, len(n.KnownPeers))
		for addr, info := range n.KnownPeers {
			if info.Active {
				activePeers = append(activePeers, addr)
			}
		}
		n.mutex.Unlock()

		if len(activePeers) > 0 {
			msg := NetworkMessage{
				Type:    "peer_list",
				Payload: mustMarshal(activePeers),
			}
			n.broadcastMessage(msg)
		}
	}
}

func (n *Node) probePeers() {
	for range time.Tick(PeerProbeInterval) {
		n.mutex.Lock()
		peers := make([]string, 0, len(n.Peers))
		for addr := range n.Peers {
			peers = append(peers, addr)
		}
		n.mutex.Unlock()

		for _, addr := range peers {
			n.sendPing(addr)
		}

		n.mutex.Lock()
		now := time.Now()
		for addr, info := range n.KnownPeers {
			if info.Active && now.Sub(info.LastSeen) > PeerTimeout {
				fmt.Printf("Node %s marking peer %s as inactive (last seen: %v)\n", n.Address, addr, info.LastSeen)
				n.KnownPeers[addr] = PeerInfo{Address: addr, LastSeen: info.LastSeen, Active: false}
				n.mutex.Unlock()
				n.disconnectPeer(addr)
				n.mutex.Lock()
			}
		}
		n.mutex.Unlock()
	}
}

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

	n.mutex.Lock()
	conn, exists := n.Peers[peerAddress]
	n.mutex.Unlock()
	if !exists {
		return
	}

	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetWriteDeadline(time.Now().Add(WriteTimeout))
	}
	if _, err := conn.Write(data); err != nil {
		fmt.Printf("Node %s error sending ping to %s: %v\n", n.Address, peerAddress, err)
		n.updatePeerStatus(peerAddress, false)
		n.disconnectPeer(peerAddress)
	}
}

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

func (n *Node) broadcastMessage(msg NetworkMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		fmt.Printf("Node %s error marshaling message: %v\n", n.Address, err)
		return
	}
	data = append(data, '\n')

	n.mutex.Lock()
	peers := make(map[string]net.Conn, len(n.Peers))
	for addr, conn := range n.Peers {
		peers[addr] = conn
	}
	n.mutex.Unlock()

	for addr, conn := range peers {
		if tcpConn, ok := conn.(*net.TCPConn); ok {
			tcpConn.SetWriteDeadline(time.Now().Add(WriteTimeout))
		}
		if _, err := conn.Write(data); err != nil {
			fmt.Printf("Node %s error sending %s to %s: %v\n", n.Address, msg.Type, addr, err)
			n.disconnectPeer(addr)
			continue
		}
		fmt.Printf("Node %s sent %s to %s\n", n.Address, msg.Type, addr)
	}
	if msg.Type != "ping" && msg.Type != "pong" {
		fmt.Printf("Node %s completed broadcasting %s\n", n.Address, msg.Type)
	}
}

func (n *Node) disconnectPeer(peerAddress string) {
	n.mutex.Lock()
	if conn, exists := n.Peers[peerAddress]; exists {
		conn.Close()
		delete(n.Peers, peerAddress)
	}
	n.mutex.Unlock()
}

func (n *Node) handlePeerList(peers []string) {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	for _, addr := range peers {
		if _, exists := n.KnownPeers[addr]; !exists {
			n.KnownPeers[addr] = PeerInfo{Address: addr, LastSeen: time.Now(), Active: false}
			go n.ConnectToPeer(addr)
		}
	}
}

func (n *Node) updatePeerStatus(peerAddress string, active bool) {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	if info, exists := n.KnownPeers[peerAddress]; exists {
		n.KnownPeers[peerAddress] = PeerInfo{
			Address:  info.Address,
			LastSeen: time.Now(),
			Active:   active,
		}
	} else {
		n.KnownPeers[peerAddress] = PeerInfo{
			Address:  peerAddress,
			LastSeen: time.Now(),
			Active:   active,
		}
	}
}

func (n *Node) handleNewBlock(block Block) {
	n.blockchainMutex.Lock()
	defer n.blockchainMutex.Unlock()

	lastBlock := n.Blockchain.Chain[len(n.Blockchain.Chain)-1]
	fmt.Printf("Node %s processing new block %d (PrevHash: %s, Expected: %s)\n",
		n.Address, block.Index, block.PrevHash, lastBlock.Hash)

	calculatedHash := calculateHash(block)
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
		n.Blockchain.Chain = append(n.Blockchain.Chain, block)
		fmt.Printf("Node %s accepted new block %d\n", n.Address, block.Index)
		go n.BroadcastBlockWithRetry(block) // Enhanced broadcast with retry
	} else {
		fmt.Printf("Node %s out of sync for block %d (index %d vs %d, prevHash mismatch)\n",
			n.Address, block.Index, block.Index, lastBlock.Index+1)
		n.requestChainFromPeers()
	}
}

func (n *Node) BroadcastBlock(block Block) {
	msg := NetworkMessage{
		Type:    "new_block",
		Payload: mustMarshal(block),
	}
	n.broadcastMessage(msg)
}

func (n *Node) BroadcastBlockWithRetry(block Block) {
	msg := NetworkMessage{
		Type:    "new_block",
		Payload: mustMarshal(block),
	}
	n.broadcastMessageWithRetry(msg, MaxBroadcastRetries)
}

func (n *Node) broadcastMessageWithRetry(msg NetworkMessage, retries int) {
	data, err := json.Marshal(msg)
	if err != nil {
		fmt.Printf("Node %s error marshaling message: %v\n", n.Address, err)
		return
	}
	data = append(data, '\n')

	n.mutex.Lock()
	peers := make(map[string]net.Conn, len(n.Peers))
	for addr, conn := range n.Peers {
		peers[addr] = conn
	}
	n.mutex.Unlock()

	failedPeers := make(map[string]bool)
	for attempt := 0; attempt <= retries; attempt++ {
		for addr, conn := range peers {
			if failedPeers[addr] {
				continue // Skip already failed peers in this retry cycle
			}
			if tcpConn, ok := conn.(*net.TCPConn); ok {
				tcpConn.SetWriteDeadline(time.Now().Add(WriteTimeout))
			}
			if _, err := conn.Write(data); err != nil {
				fmt.Printf("Node %s error sending %s to %s (attempt %d/%d): %v\n", n.Address, msg.Type, addr, attempt+1, retries+1, err)
				failedPeers[addr] = true
				if attempt == retries {
					n.disconnectPeer(addr)
				}
				continue
			}
			fmt.Printf("Node %s sent %s to %s (attempt %d/%d)\n", n.Address, msg.Type, addr, attempt+1, retries+1)
			delete(failedPeers, addr) // Success, remove from failed list
		}
		if len(failedPeers) == 0 {
			break // All peers succeeded
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

	n.mutex.Lock()
	if conn, exists := n.Peers[peerAddress]; exists {
		if tcpConn, ok := conn.(*net.TCPConn); ok {
			tcpConn.SetWriteDeadline(time.Now().Add(WriteTimeout))
		}
		conn.Write(data)
		fmt.Printf("Node %s requested chain from %s\n", n.Address, peerAddress)
	}
	n.mutex.Unlock()
}

func (n *Node) requestChainFromPeers() {
	n.mutex.Lock()
	peers := make([]string, 0, len(n.Peers))
	for addr := range n.Peers {
		peers = append(peers, addr)
	}
	n.mutex.Unlock()

	for _, peer := range peers {
		n.requestChain(peer)
	}
}

func (n *Node) sendChain(conn net.Conn) {
	n.blockchainMutex.Lock()
	chain := make([]Block, len(n.Blockchain.Chain))
	copy(chain, n.Blockchain.Chain)
	n.blockchainMutex.Unlock()

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

func (n *Node) handleChainResponse(chain []Block) {
	n.blockchainMutex.Lock()
	defer n.blockchainMutex.Unlock()

	currentLength := len(n.Blockchain.Chain)
	newLength := len(chain)
	fmt.Printf("Node %s received chain response (length %d, current %d)\n", n.Address, newLength, currentLength)

	if newLength <= currentLength {
		fmt.Printf("Node %s ignored chain response: not longer than current chain\n", n.Address)
		return
	}

	valid := true
	expectedPrevHash := "0"
	for i, block := range chain {
		if block.PrevHash != expectedPrevHash {
			valid = false
			fmt.Printf("Node %s rejected chain: prevHash mismatch at block %d (Expected: %s, Got: %s)\n",
				n.Address, i, expectedPrevHash, block.PrevHash)
			break
		}
		if block.Hash != calculateHash(block) {
			valid = false
			fmt.Printf("Node %s rejected chain: invalid hash at block %d\n", n.Address, i)
			break
		}
		expectedPrevHash = block.Hash
	}

	if valid {
		n.Blockchain.Chain = chain
		fmt.Printf("Node %s updated chain to length %d\n", n.Address, newLength)
	} else {
		fmt.Printf("Node %s rejected chain response: invalid chain\n", n.Address)
	}
}

func mustMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		fmt.Printf("Error marshaling: %v\n", err)
		return nil
	}
	return data
}
