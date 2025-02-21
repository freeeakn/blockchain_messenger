package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// Message represents a chat message
type Message struct {
	Sender    string
	Recipient string
	Content   string
	Timestamp int64
}

// Block represents a block in our blockchain
type Block struct {
	Index     int
	Timestamp int64
	Messages  []Message
	PrevHash  string
	Hash      string
	Nonce     int
}

// Blockchain holds the chain and related data
type Blockchain struct {
	Chain      []Block
	Difficulty int
}

// Node represents a network node
type Node struct {
	Address    string
	Peers      map[string]*net.Conn
	Blockchain *Blockchain
	mutex      sync.Mutex
}

// NewBlockchain creates a new blockchain with genesis block
func NewBlockchain() *Blockchain {
	genesisBlock := Block{
		Index:     0,
		Timestamp: time.Now().Unix(),
		Messages:  []Message{},
		PrevHash:  "0",
	}
	genesisBlock.Hash = calculateHash(genesisBlock)

	return &Blockchain{
		Chain:      []Block{genesisBlock},
		Difficulty: 4,
	}
}

// calculateHash generates a hash for a block
func calculateHash(block Block) string {
	record := fmt.Sprintf("%d%d%v%s%d",
		block.Index,
		block.Timestamp,
		block.Messages,
		block.PrevHash,
		block.Nonce)
	h := sha256.New()
	h.Write([]byte(record))
	return hex.EncodeToString(h.Sum(nil))
}

// AddMessage adds a new message to the blockchain
func (bc *Blockchain) AddMessage(sender, recipient, content string, key []byte) {
	encryptedContent, err := encryptMessage(content, key)
	if err != nil {
		fmt.Println("Encryption error:", err)
		return
	}

	newMessage := Message{
		Sender:    sender,
		Recipient: recipient,
		Content:   hex.EncodeToString(encryptedContent),
		Timestamp: time.Now().Unix(),
	}

	lastBlock := bc.Chain[len(bc.Chain)-1]
	newBlock := Block{
		Index:     lastBlock.Index + 1,
		Timestamp: time.Now().Unix(),
		Messages:  []Message{newMessage},
		PrevHash:  lastBlock.Hash,
	}

	newBlock = mineBlock(newBlock, bc.Difficulty)
	bc.Chain = append(bc.Chain, newBlock)
}

// ReadMessages retrieves and decrypts messages for a recipient
func (bc *Blockchain) ReadMessages(recipient string, key []byte) []string {
	var messages []string

	for _, block := range bc.Chain {
		for _, msg := range block.Messages {
			if msg.Recipient == recipient {
				encryptedBytes, err := hex.DecodeString(msg.Content)
				if err != nil {
					continue
				}

				decrypted, err := decryptMessage(encryptedBytes, key)
				if err != nil {
					continue
				}
				messages = append(messages, fmt.Sprintf("From %s: %s", msg.Sender, decrypted))
			}
		}
	}
	return messages
}

// mineBlock finds a valid hash for the block
func mineBlock(block Block, difficulty int) Block {
	target := ""
	for i := 0; i < difficulty; i++ {
		target += "0"
	}

	for {
		hash := calculateHash(block)
		if hash[:difficulty] == target {
			block.Hash = hash
			return block
		}
		block.Nonce++
	}
}

// VerifyChain checks the integrity of the blockchain
func (bc *Blockchain) VerifyChain() bool {
	for i := 1; i < len(bc.Chain); i++ {
		current := bc.Chain[i]
		previous := bc.Chain[i-1]

		if current.Hash != calculateHash(current) {
			return false
		}
		if current.PrevHash != previous.Hash {
			return false
		}
	}
	return true
}

// encryptMessage encrypts a message
func encryptMessage(content string, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	plaintext := []byte(content)
	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]

	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintext)

	return ciphertext, nil
}

// decryptMessage decrypts a message
func decryptMessage(ciphertext, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	if len(ciphertext) < aes.BlockSize {
		return "", errors.New("ciphertext too short")
	}

	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)

	return string(ciphertext), nil
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

func main() {
	bc := NewBlockchain()

	// Generate a 32-byte key for AES-256
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		fmt.Println("Error generating key:", err)
		return
	}

	// Send encrypted messages
	bc.AddMessage("Alice", "Bob", "Hello Bob!", key)
	time.Sleep(time.Second)
	bc.AddMessage("Bob", "Alice", "Hi Alice, how are you?", key)

	// Print the blockchain
	fmt.Println("Blockchain contents (encrypted):")
	for _, block := range bc.Chain {
		fmt.Printf("Index: %d\n", block.Index)
		fmt.Printf("Hash: %s\n", block.Hash)
		fmt.Printf("PrevHash: %s\n", block.PrevHash)
		fmt.Printf("Messages: %v\n\n", block.Messages)
	}

	// Read and decrypt messages
	fmt.Println("Decrypted messages for Alice:")
	aliceMessages := bc.ReadMessages("Alice", key)
	for _, msg := range aliceMessages {
		fmt.Println(msg)
	}

	fmt.Println("\nDecrypted messages for Bob:")
	bobMessages := bc.ReadMessages("Bob", key)
	for _, msg := range bobMessages {
		fmt.Println(msg)
	}

	fmt.Println("\nBlockchain valid?", bc.VerifyChain())

	// Start the network
	startNetwork()

	// Keep the program running to maintain network
	select {}
}
