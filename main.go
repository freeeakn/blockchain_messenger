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

	var bootstrapPeers []string
	if *peer != "" {
		bootstrapPeers = []string{*peer}
	}

	bc := NewBlockchain()
	node := NewNode(*address, bc, bootstrapPeers)
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
	fmt.Println("Commands: send <recipient> <message>, read, peers, blocks, debug, quit")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				fmt.Printf("Scanner error: %v\n", err)
			} else {
				fmt.Println("Input closed (EOF)")
			}
			return
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		parts := strings.SplitN(input, " ", 3)

		switch parts[0] {
		case "send":
			fmt.Println("Processing 'send' command...")
			if len(parts) < 3 {
				fmt.Println("Usage: send <recipient> <message>")
				continue
			}
			recipient := parts[1]
			message := parts[2]
			startTime := time.Now()
			if err := node.Blockchain.AddMessage(*name, recipient, message, key); err != nil {
				fmt.Printf("Failed to add message: %v\n", err)
				continue
			}
			miningDuration := time.Since(startTime)
			fmt.Printf("Message sent and block mined in %v\n", miningDuration)

			// Broadcast the new block
			node.blockchainMutex.Lock()
			newBlock := node.Blockchain.Chain[len(node.Blockchain.Chain)-1]
			node.blockchainMutex.Unlock()

			broadcastChan := make(chan bool, 1)
			go func() {
				node.BroadcastBlock(newBlock)
				broadcastChan <- true
			}()

			select {
			case <-broadcastChan:
				fmt.Println("Message broadcast completed")
			case <-time.After(10 * time.Second):
				fmt.Println("Timeout broadcasting message - network issue")
			}

		case "read":
			fmt.Println("Processing 'read' command...")
			var messages []string
			var chainCopy []Block

			chainCopyChan := make(chan []Block, 1)
			go func() {
				node.blockchainMutex.Lock()
				defer node.blockchainMutex.Unlock()
				chainCopy := make([]Block, len(node.Blockchain.Chain))
				copy(chainCopy, node.Blockchain.Chain)
				chainCopyChan <- chainCopy
			}()

			select {
			case chainCopy = <-chainCopyChan:
				fmt.Println("Chain copied successfully")
			case <-time.After(5 * time.Second):
				fmt.Println("Timeout copying chain - potential deadlock or slow operation")
				continue
			}

			msgChan := make(chan []string, 1)
			go func() {
				defer func() {
					if r := recover(); r != nil {
						fmt.Printf("Panic in read processing: %v\n", r)
						msgChan <- nil
					}
				}()
				var localMessages []string
				for i, block := range chainCopy {
					fmt.Printf("Processing block %d with %d messages\n", block.Index, len(block.Messages))
					for j, msg := range block.Messages {
						if msg.Recipient == *name {
							fmt.Printf("Decoding message %d in block %d\n", j, i)
							encryptedBytes, err := hex.DecodeString(msg.Content)
							if err != nil {
								fmt.Printf("Error decoding message %d in block %d: %v\n", j, i, err)
								continue
							}
							fmt.Printf("Decrypting message %d in block %d\n", j, i)
							decrypted, err := decryptMessage(encryptedBytes, key)
							if err != nil {
								fmt.Printf("Error decrypting message %d in block %d: %v\n", j, i, err)
								continue
							}
							localMessages = append(localMessages, fmt.Sprintf("From %s: %s", msg.Sender, decrypted))
						}
					}
				}
				msgChan <- localMessages
			}()

			select {
			case messages = <-msgChan:
				fmt.Println("Message processing completed")
			case <-time.After(5 * time.Second):
				fmt.Println("Timeout processing messages - decryption or decoding issue")
				continue
			}

			if len(messages) == 0 {
				fmt.Println("No messages found")
			} else {
				fmt.Println("Your messages:")
				for _, msg := range messages {
					fmt.Println(msg)
				}
			}
			fmt.Println("Read command completed")

		case "peers":
			fmt.Println("Processing 'peers' command...")
			node.mutex.Lock()
			defer node.mutex.Unlock()
			fmt.Println("Known peers:")
			for addr, info := range node.KnownPeers {
				status := "inactive"
				if info.Active {
					status = "active"
				}
				fmt.Printf("  %s (Last seen: %v, Status: %s)\n", addr, info.LastSeen, status)
			}
			fmt.Println("Peers command completed")

		case "blocks":
			fmt.Println("Processing 'blocks' command...")
			node.blockchainMutex.Lock()
			defer node.blockchainMutex.Unlock()
			if len(node.Blockchain.Chain) == 0 {
				fmt.Println("No blocks in the blockchain")
			} else {
				fmt.Println("Blockchain blocks:")
				for _, block := range node.Blockchain.Chain {
					fmt.Printf("Block %d:\n", block.Index)
					fmt.Printf("  Timestamp: %d\n", block.Timestamp)
					fmt.Printf("  Messages: [\n")
					for j, msg := range block.Messages {
						fmt.Printf("    %d: Sender: %s, Recipient: %s, Content: %s, Timestamp: %d\n",
							j, msg.Sender, msg.Recipient, msg.Content, msg.Timestamp)
					}
					fmt.Printf("  ]\n")
					fmt.Printf("  PrevHash: %s\n", block.PrevHash)
					fmt.Printf("  Hash: %s\n", block.Hash)
					fmt.Printf("  Nonce: %d\n", block.Nonce)
					fmt.Println()
				}
			}
			fmt.Println("Blocks command completed")

		case "debug":
			fmt.Println("Processing 'debug' command...")
			node.mutex.Lock()
			peerCount := len(node.Peers)
			knownPeerCount := len(node.KnownPeers)
			node.mutex.Unlock()

			node.blockchainMutex.Lock()
			defer node.blockchainMutex.Unlock()
			chainLength := len(node.Blockchain.Chain)
			var lastBlockHash string
			if chainLength > 0 {
				lastBlockHash = node.Blockchain.Chain[chainLength-1].Hash
			} else {
				lastBlockHash = "N/A (empty chain)"
			}

			fmt.Printf("Node %s state:\n", *address)
			fmt.Printf("  Chain length: %d\n", chainLength)
			fmt.Printf("  Peer count: %d\n", peerCount)
			fmt.Printf("  Known peers: %d\n", knownPeerCount)
			fmt.Printf("  Last block hash: %s\n", lastBlockHash)
			fmt.Println("Debug command completed")

		case "quit":
			fmt.Println("Shutting down...")
			return

		default:
			fmt.Println("Unknown command. Available: send, read, peers, blocks, debug, quit")
		}
	}
}
