package main

import (
	"bufio"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/freeeakn/AetherWave/pkg/blockchain"
	"github.com/freeeakn/AetherWave/pkg/crypto"
	"github.com/freeeakn/AetherWave/pkg/network"
)

func main() {
	// Парсинг аргументов командной строки
	address := flag.String("address", ":3000", "Node address (e.g., :3000 or 192.168.1.100:3000)")
	peer := flag.String("peer", "", "Initial peer address to connect to (e.g., 192.168.1.101:3000)")
	name := flag.String("name", "User", "Your username")
	keyStr := flag.String("key", "", "Shared encryption key (hex-encoded, 32 bytes); if empty, one will be generated")
	flag.Parse()

	// Инициализация ключа шифрования
	var key []byte
	var err error
	if *keyStr != "" {
		key, err = hex.DecodeString(*keyStr)
		if err != nil || len(key) != 32 {
			fmt.Println("Error: Key must be a 32-byte hex string")
			return
		}
	} else {
		key, err = crypto.GenerateKey()
		if err != nil {
			fmt.Println("Error generating key:", err)
			return
		}
		fmt.Printf("Generated key (share this with peers): %s\n", hex.EncodeToString(key))
	}

	// Инициализация блокчейна и узла
	var bootstrapPeers []string
	if *peer != "" {
		bootstrapPeers = []string{*peer}
	}

	bc := blockchain.NewBlockchain()
	node := network.NewNode(*address, bc, bootstrapPeers)

	// Запуск узла
	if err := node.Start(); err != nil {
		fmt.Printf("Failed to start node: %v\n", err)
		return
	}

	// Подключение к начальному пиру
	if *peer != "" {
		time.Sleep(1 * time.Second)
		if err := node.ConnectToPeer(*peer); err != nil {
			fmt.Printf("Failed to connect to peer %s: %v\n", *peer, err)
		} else {
			fmt.Printf("Connected to peer %s\n", *peer)
		}
	}

	fmt.Println("Waiting for network stabilization (5 seconds)...")
	time.Sleep(5 * time.Second)

	// Обработка сигналов для корректного завершения
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nShutting down...")
		node.Stop()
		os.Exit(0)
	}()

	// Интерактивный режим
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
			handleSendCommand(parts, *name, bc, node, key)
		case "read":
			handleReadCommand(*name, bc, key)
		case "peers":
			handlePeersCommand(node)
		case "blocks":
			handleBlocksCommand(bc)
		case "debug":
			handleDebugCommand(*address, bc, node)
		case "quit":
			fmt.Println("Shutting down...")
			node.Stop()
			return
		default:
			fmt.Println("Unknown command. Available: send, read, peers, blocks, debug, quit")
		}
	}
}

// handleSendCommand обрабатывает команду отправки сообщения
func handleSendCommand(parts []string, name string, bc *blockchain.Blockchain, node *network.Node, key []byte) {
	fmt.Println("Processing 'send' command...")
	if len(parts) < 3 {
		fmt.Println("Usage: send <recipient> <message>")
		return
	}
	recipient := parts[1]
	message := parts[2]

	startTime := time.Now()
	if err := bc.AddMessage(name, recipient, message, key); err != nil {
		fmt.Printf("Failed to add message: %v\n", err)
		return
	}
	miningDuration := time.Since(startTime)
	fmt.Printf("Message sent and block mined in %v\n", miningDuration)

	// Рассылка нового блока
	lastBlock := bc.GetLastBlock()

	broadcastChan := make(chan bool, 1)
	go func() {
		node.BroadcastBlockWithRetry(lastBlock)
		broadcastChan <- true
	}()

	select {
	case <-broadcastChan:
		fmt.Println("Message broadcast completed")
	case <-time.After(5 * time.Second):
		fmt.Println("Timeout broadcasting message - network issue")
	}
}

// handleReadCommand обрабатывает команду чтения сообщений
func handleReadCommand(name string, bc *blockchain.Blockchain, key []byte) {
	fmt.Println("Processing 'read' command...")
	messages := bc.ReadMessages(name, key)

	if len(messages) == 0 {
		fmt.Println("No messages found")
	} else {
		fmt.Println("Your messages:")
		for _, msg := range messages {
			fmt.Println(msg)
		}
	}
	fmt.Println("Read command completed")
}

// handlePeersCommand обрабатывает команду просмотра пиров
func handlePeersCommand(node *network.Node) {
	fmt.Println("Processing 'peers' command...")
	peers := node.GetPeers()

	fmt.Println("Known peers:")
	for _, info := range peers {
		status := "inactive"
		if info.Active {
			status = "active"
		}
		fmt.Printf("  %s (Last seen: %v, Status: %s)\n", info.Address, info.LastSeen, status)
	}
	fmt.Println("Peers command completed")
}

// handleBlocksCommand обрабатывает команду просмотра блоков
func handleBlocksCommand(bc *blockchain.Blockchain) {
	fmt.Println("Processing 'blocks' command...")
	chain := bc.GetChain()

	if len(chain) == 0 {
		fmt.Println("No blocks in the blockchain")
	} else {
		fmt.Println("Blockchain blocks:")
		for _, block := range chain {
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
}

// handleDebugCommand обрабатывает команду отладки
func handleDebugCommand(address string, bc *blockchain.Blockchain, node *network.Node) {
	fmt.Println("Processing 'debug' command...")
	peers := node.GetPeers()
	peerCount := 0
	knownPeerCount := len(peers)

	for _, info := range peers {
		if info.Active {
			peerCount++
		}
	}

	chain := bc.GetChain()
	chainLength := len(chain)
	var lastBlockHash string
	if chainLength > 0 {
		lastBlockHash = chain[chainLength-1].Hash
	} else {
		lastBlockHash = "N/A (empty chain)"
	}

	fmt.Printf("Node %s state:\n", address)
	fmt.Printf("  Chain length: %d\n", chainLength)
	fmt.Printf("  Active peer count: %d\n", peerCount)
	fmt.Printf("  Known peers: %d\n", knownPeerCount)
	fmt.Printf("  Last block hash: %s\n", lastBlockHash)
	fmt.Println("Debug command completed")
}
