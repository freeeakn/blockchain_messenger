package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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
	for range difficulty {
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
