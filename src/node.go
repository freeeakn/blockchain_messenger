package main

import (
	"fmt"
	"net"
	"sync"
)

// Node represents a network node
type Node struct {
	Address    string
	Peers      map[string]*net.Conn
	Blockchain *Blockchain
	mutex      sync.Mutex
}

// NewNode creates a new network node
func NewNode(address string, bc *Blockchain) *Node {
	return &Node{
		Address:    address,
		Peers:      make(map[string]*net.Conn),
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

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				fmt.Println("Error accepting connection:", err)
				continue
			}
			go n.handleConnection(conn)
		}
	}()
}

// ConnectToPeer connects to another node
func (n *Node) ConnectToPeer(peerAddress string) error {
	conn, err := net.Dial("tcp", peerAddress)
	if err != nil {
		return err
	}

	n.mutex.Lock()
	n.Peers[peerAddress] = &conn
	n.mutex.Unlock()

	go n.handleConnection(conn)
	return nil
}

// handleConnection manages peer connections
func (n *Node) handleConnection(conn net.Conn) {
	defer conn.Close()
	fmt.Println("New connection from:", conn.RemoteAddr().String())
}

// startNetwork sets up the P2P network
func startNetwork() {
	bc := NewBlockchain()
	node1 := NewNode(":3000", bc)
	node2 := NewNode(":3001", bc)

	go node1.Start()
	go node2.Start()

	node1.ConnectToPeer(":3001")
}