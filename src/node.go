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
}

type NetworkMessage struct {
	Type    string // "peer_list", "new_block", "chain_request", "chain_response"
	Payload json.RawMessage
}

type Node struct {
	Address    string
	Peers      map[string]*net.Conn
	KnownPeers map[string]PeerInfo
	Blockchain *Blockchain
	mutex      sync.Mutex
}

func NewNode(address string, bc *Blockchain) *Node {
	return &Node{
		Address:    address,
		Peers:      make(map[string]*net.Conn),
		KnownPeers: make(map[string]PeerInfo),
		Blockchain: bc,
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
	n.KnownPeers[n.Address] = PeerInfo{Address: n.Address, LastSeen: time.Now()}
	n.mutex.Unlock()

	go n.broadcastPeerList()

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

func (n *Node) ConnectToPeer(peerAddress string) error {
	if peerAddress == n.Address {
		return nil
	}

	conn, err := net.Dial("tcp", peerAddress)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %v", peerAddress, err)
	}

	n.mutex.Lock()
	n.Peers[peerAddress] = &conn
	n.KnownPeers[peerAddress] = PeerInfo{Address: peerAddress, LastSeen: time.Now()}
	n.mutex.Unlock()

	fmt.Printf("Node %s connected to peer %s\n", n.Address, peerAddress)
	n.requestChain(peerAddress)

	go n.handleConnection(conn)
	return nil
}

func (n *Node) handleConnection(conn net.Conn) {
	defer conn.Close()
	decoder := json.NewDecoder(conn)

	for {
		var msg NetworkMessage
		if err := decoder.Decode(&msg); err != nil {
			if err.Error() != "EOF" {
				fmt.Printf("Node %s error decoding message from %s: %v\n", n.Address, conn.RemoteAddr().String(), err)
			}
			return
		}

		fmt.Printf("Node %s received message type %s from %s\n", n.Address, msg.Type, conn.RemoteAddr().String())

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
	data, err := json.Marshal(msg)
	if err != nil {
		fmt.Printf("Node %s error marshaling message: %v\n", n.Address, err)
		return
	}
	data = append(data, '\n')

	n.mutex.Lock()
	defer n.mutex.Unlock()

	for addr, conn := range n.Peers {
		_, err := (*conn).Write(data)
		if err != nil {
			fmt.Printf("Node %s error sending to %s: %v\n", n.Address, addr, err)
			delete(n.Peers, addr)
			continue
		}
		fmt.Printf("Node %s sent %s to %s\n", n.Address, msg.Type, addr)
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

func (n *Node) handleNewBlock(block Block) {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	lastBlock := n.Blockchain.Chain[len(n.Blockchain.Chain)-1]
	fmt.Printf("Node %s processing new block %d (PrevHash: %s, Expected: %s)\n",
		n.Address, block.Index, block.PrevHash, lastBlock.Hash)

	calculatedHash := calculateHash(block)
	if block.Hash != calculatedHash {
		fmt.Printf("Node %s rejected block %d: invalid hash (Expected: %s, Got: %s)\n",
			n.Address, block.Index, calculatedHash, block.Hash)
		return
	}

	if block.Index == lastBlock.Index+1 && block.PrevHash == lastBlock.Hash {
		n.Blockchain.Chain = append(n.Blockchain.Chain, block)
		fmt.Printf("Node %s accepted new block %d\n", n.Address, block.Index)
		n.BroadcastBlock(block)
	} else {
		fmt.Printf("Node %s rejected block %d: out of sync (index %d, prevHash mismatch)\n",
			n.Address, block.Index, block.Index)
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

func (n *Node) requestChain(peerAddress string) {
	msg := NetworkMessage{
		Type:    "chain_request",
		Payload: nil,
	}
	data, _ := json.Marshal(msg)
	data = append(data, '\n')

	n.mutex.Lock()
	if conn, exists := n.Peers[peerAddress]; exists {
		(*conn).Write(data)
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
	n.mutex.Lock()
	chain := n.Blockchain.Chain
	n.mutex.Unlock()

	msg := NetworkMessage{
		Type:    "chain_response",
		Payload: mustMarshal(chain),
	}
	data, _ := json.Marshal(msg)
	data = append(data, '\n')
	conn.Write(data)
	fmt.Printf("Node %s sent chain (length %d) to %s\n", n.Address, len(chain), conn.RemoteAddr().String())
}

func (n *Node) handleChainResponse(chain []Block) {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	currentLength := len(n.Blockchain.Chain)
	newLength := len(chain)
	fmt.Printf("Node %s received chain response (length %d, current %d)\n", n.Address, newLength, currentLength)

	if newLength <= currentLength {
		fmt.Printf("Node %s ignored chain response: not longer than current chain\n", n.Address)
		return
	}

	valid := true
	expectedPrevHash := "0" // For genesis block
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
		fmt.Println("Error marshaling:", err)
		return nil
	}
	return data
}
