package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

func main() {
	address := flag.String("address", ":3000", "Node address (e.g., :3000 or 192.168.1.100:3000)")
	peer := flag.String("peer", "", "Initial peer address to connect to (e.g., 192.168.1.101:3000)")
	name := flag.String("name", "User", "Your username")
	keyStr := flag.String("key", "", "Shared encryption key (hex-encoded, 32 bytes); if empty, one will be generated")
	flag.Parse()

	var key []byte
	if *keyStr != "" {
		var err error
		key, err = hex.DecodeString(*keyStr)
		if err != nil || len(key) != 32 {
			fmt.Println("Error: Key must be a 32-byte hex string")
			return
		}
	} else {
		key = make([]byte, 32)
		_, err := rand.Read(key)
		if err != nil {
			fmt.Println("Error generating key:", err)
			return
		}
		fmt.Printf("Generated key (share this with peers): %s\n", hex.EncodeToString(key))
	}

	bc := NewBlockchain()
	node := NewNode(*address, bc)
	go node.Start()

	if *peer != "" {
		time.Sleep(1 * time.Second)
		if err := node.ConnectToPeer(*peer); err != nil {
			fmt.Printf("Failed to connect to peer %s: %v\n", *peer, err)
		} else {
			fmt.Printf("Connected to peer %s\n", *peer)
		}
	}

	fmt.Println("Waiting for network stabilization (10 seconds)...")
	time.Sleep(10 * time.Second)

	fmt.Printf("Welcome, %s! Running on %s\n", *name, *address)
	fmt.Println("Commands: send <recipient> <message>, read, peers, quit")

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())
		parts := strings.SplitN(input, " ", 3)

		switch parts[0] {
		case "send":
			if len(parts) < 3 {
				fmt.Println("Usage: send <recipient> <message>")
				continue
			}
			recipient := parts[1]
			message := parts[2]
			startTime := time.Now()
			node.Blockchain.AddMessage(*name, recipient, message, key)
			miningDuration := time.Since(startTime)
			fmt.Printf("Message sent and block mined in %v\n", miningDuration)
			newBlock := node.Blockchain.Chain[len(node.Blockchain.Chain)-1]
			node.BroadcastBlock(newBlock)

		case "read":
			messages := node.Blockchain.ReadMessages(*name, key)
			if len(messages) == 0 {
				fmt.Println("No messages found")
			} else {
				fmt.Println("Your messages:")
				for _, msg := range messages {
					fmt.Println(msg)
				}
			}

		case "peers":
			node.mutex.Lock()
			fmt.Println("Known peers:")
			for addr, info := range node.KnownPeers {
				fmt.Printf("  %s (Last seen: %v)\n", addr, info.LastSeen)
			}
			node.mutex.Unlock()

		case "quit":
			fmt.Println("Shutting down...")
			return

		default:
			fmt.Println("Unknown command. Available: send, read, peers, quit")
		}
	}
}
