package main

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"
)

// PeerInfo for discovery
type PeerInfo struct {
	Address  string
	LastSeen time.Time
}

// MessageType for network communication
type NetworkMessage struct {
	Type    string          // "peer_list", "new_message", "block"
	Payload json.RawMessage // Raw JSON to be unmarshaled based on Type
}

// Node represents a network node
type Node struct {
	Address    string
	Peers      map[string]*net.Conn
	KnownPeers map[string]PeerInfo
	Blockchain *Blockchain
	mutex      sync.Mutex
}

// NewNode creates a new network node
func NewNode(address string, bc *Blockchain) *Node {
	return &Node{
		Address:    address,
		Peers:      make(map[string]*net.Conn),
		KnownPeers: make(map[string]PeerInfo),
		Blockchain: bc,
	}
}

// Start starts the node
func (n *Node) Start() {
	ln, err := net.Listen("tcp", n.Address)
	if err != nil {
		fmt.Println("Error starting node:", err)
		return
	}

	n.mutex.Lock()
	n.KnownPeers[n.Address] = PeerInfo{Address: n.Address, LastSeen: time.Now()}
	n.mutex.Unlock()

	go n.broadcastPeerList()

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}
		go n.handleConnection(conn)
	}
}

// ConnectToPeer connects to another node
func (n *Node) ConnectToPeer(peerAddress string) error {
	if peerAddress == n.Address {
		return nil
	}

	conn, err := net.Dial("tcp", peerAddress)
	if err != nil {
		return err
	}

	n.mutex.Lock()
	n.Peers[peerAddress] = &conn
	n.KnownPeers[peerAddress] = PeerInfo{Address: peerAddress, LastSeen: time.Now()}
	n.mutex.Unlock()

	go n.handleConnection(conn)
	return nil
}

// handleConnection manages peer connections
func (n *Node) handleConnection(conn net.Conn) {
	defer conn.Close()
	decoder := json.NewDecoder(conn)

	for {
		var msg NetworkMessage
		if err := decoder.Decode(&msg); err != nil {
			fmt.Println("Error decoding message:", err)
			return
		}

		switch msg.Type {
		case "peer_list":
			var peers []string
			if err := json.Unmarshal(msg.Payload, &peers); err != nil {
				continue
			}
			n.handlePeerList(peers)
		}
	}
}

func (n *Node) broadcastPeerList() {
	for range time.Tick(10 * time.Second) {
		n.mutex.Lock()
		var peerList []string
		for addr := range n.KnownPeers {
			peerList = append(peerList, addr)
		}
		n.mutex.Unlock()

		msg := NetworkMessage{
			Type:    "peer_list",
			Payload: mustMarshal(peerList),
		}

		n.broadcastMessage(msg)
	}
}

func (n *Node) broadcastMessage(msg NetworkMessage) {
	data, _ := json.Marshal(msg)
	n.mutex.Lock()
	defer n.mutex.Unlock()

	for addr, conn := range n.Peers {
		_, err := (*conn).Write(append(data, '\n'))
		if err != nil {
			delete(n.Peers, addr)
			continue
		}
	}
}

func (n *Node) handlePeerList(peers []string) {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	for _, addr := range peers {
		if _, exists := n.KnownPeers[addr]; !exists {
			n.KnownPeers[addr] = PeerInfo{Address: addr, LastSeen: time.Now()}
			go n.ConnectToPeer(addr)
		}
	}
}

func mustMarshal(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

// startNetwork sets up the P2P network
func startNetwork() {
	bc := NewBlockchain()
	node1 := NewNode(":3000", bc)
	node2 := NewNode(":3001", bc)
	node3 := NewNode(":3002", bc)

	go node1.Start()
	go node2.Start()
	go node3.Start()

	node1.ConnectToPeer(":3001")
	node2.ConnectToPeer(":3000")
	node3.ConnectToPeer(":3001")
}
