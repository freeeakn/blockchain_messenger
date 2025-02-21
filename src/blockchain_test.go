package main

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"testing"
)

func TestNewBlockchain(t *testing.T) {
	bc := NewBlockchain()

	if bc == nil {
		t.Fatal("NewBlockchain returned nil")
	}

	if len(bc.Chain) != 1 {
		t.Errorf("Expected chain length of 1, got %d", len(bc.Chain))
	}

	genesisBlock := bc.Chain[0]

	if genesisBlock.Index != 0 {
		t.Errorf("Expected genesis block index to be 0, got %d", genesisBlock.Index)
	}

	if genesisBlock.PrevHash != "0" {
		t.Errorf("Expected genesis block previous hash to be '0', got %s", genesisBlock.PrevHash)
	}

	if len(genesisBlock.Messages) != 0 {
		t.Errorf("Expected genesis block to have no messages, got %d", len(genesisBlock.Messages))
	}

	if genesisBlock.Hash == "" {
		t.Error("Genesis block hash is empty")
	}

	if bc.Difficulty != 4 {
		t.Errorf("Expected blockchain difficulty to be 4, got %d", bc.Difficulty)
	}
}

func TestNewBlockchainGenesisHash(t *testing.T) {
	bc := NewBlockchain()

	if bc == nil {
		t.Fatal("NewBlockchain returned nil")
	}

	genesisBlock := bc.Chain[0]
	expectedHash := calculateHash(genesisBlock)

	if genesisBlock.Hash != expectedHash {
		t.Errorf("Genesis block hash mismatch. Expected: %s, Got: %s", expectedHash, genesisBlock.Hash)
	}
}

func TestAddMessage(t *testing.T) {
	bc := NewBlockchain()
	initialChainLength := len(bc.Chain)

	sender := "Alice"
	recipient := "Bob"
	content := "Hello, Bob!"
	key := make([]byte, 32) // AES-256 requires 32 bytes
	_, err := rand.Read(key)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	bc.AddMessage(sender, recipient, content, key)

	if len(bc.Chain) != initialChainLength+1 {
		t.Errorf("Expected chain length to increase by 1, got %d", len(bc.Chain))
	}

	newBlock := bc.Chain[len(bc.Chain)-1]

	if len(newBlock.Messages) != 1 {
		t.Errorf("Expected new block to have 1 message, got %d", len(newBlock.Messages))
	}

	newMessage := newBlock.Messages[0]

	if newMessage.Sender != sender {
		t.Errorf("Expected sender to be %s, got %s", sender, newMessage.Sender)
	}

	if newMessage.Recipient != recipient {
		t.Errorf("Expected recipient to be %s, got %s", recipient, newMessage.Recipient)
	}

	if newMessage.Content == content {
		t.Error("Message content should be encrypted")
	}

	if newMessage.Timestamp == 0 {
		t.Error("Message timestamp should not be zero")
	}

	if newBlock.PrevHash != bc.Chain[initialChainLength-1].Hash {
		t.Errorf("New block's previous hash does not match the last block's hash. Expected: %s, Got: %s",
			bc.Chain[initialChainLength-1].Hash, newBlock.PrevHash)
	}

	if !strings.HasPrefix(newBlock.Hash, strings.Repeat("0", bc.Difficulty)) {
		t.Errorf("New block's hash does not have the required number of leading zeros. Got: %s", newBlock.Hash)
	}
}

func TestAddMessageEncryption(t *testing.T) {
	bc := NewBlockchain()
	sender := "Alice"
	recipient := "Bob"
	content := "Hello, Bob!"
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	bc.AddMessage(sender, recipient, content, key)

	lastBlock := bc.Chain[len(bc.Chain)-1]
	addedMessage := lastBlock.Messages[0]

	// Check if the content is encrypted (not equal to the original content)
	if addedMessage.Content == content {
		t.Error("Message content was not encrypted")
	}

	// Attempt to decrypt the message
	encryptedBytes, err := hex.DecodeString(addedMessage.Content)
	if err != nil {
		t.Fatalf("Failed to decode encrypted content: %v", err)
	}

	decryptedContent, err := decryptMessage(encryptedBytes, key)
	if err != nil {
		t.Fatalf("Failed to decrypt message: %v", err)
	}

	// Check if the decrypted content matches the original
	if decryptedContent != content {
		t.Errorf("Decrypted content does not match original. Expected: %s, Got: %s", content, decryptedContent)
	}
}

func TestAddMessageMiningDifficulty(t *testing.T) {
	bc := NewBlockchain()
	initialChainLength := len(bc.Chain)

	sender := "Alice"
	recipient := "Bob"
	content := "Hello, Bob!"
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	bc.AddMessage(sender, recipient, content, key)

	if len(bc.Chain) != initialChainLength+1 {
		t.Errorf("Expected chain length to increase by 1, got %d", len(bc.Chain))
	}

	newBlock := bc.Chain[len(bc.Chain)-1]

	// Check if the new block's hash starts with the correct number of zeros
	expectedPrefix := strings.Repeat("0", bc.Difficulty)
	if !strings.HasPrefix(newBlock.Hash, expectedPrefix) {
		t.Errorf("New block hash does not have the required difficulty. Expected prefix: %s, Got: %s",
			expectedPrefix, newBlock.Hash[:bc.Difficulty])
	}

	// Verify that the hash is valid for the block
	calculatedHash := calculateHash(newBlock)
	if calculatedHash != newBlock.Hash {
		t.Errorf("New block hash is invalid. Expected: %s, Got: %s", calculatedHash, newBlock.Hash)
	}
}

func TestAddMessageAppendsBlock(t *testing.T) {
	bc := NewBlockchain()
	initialChainLength := len(bc.Chain)

	sender := "Alice"
	recipient := "Bob"
	content := "Hello, Bob!"
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	bc.AddMessage(sender, recipient, content, key)

	if len(bc.Chain) != initialChainLength+1 {
		t.Errorf("Expected chain length to increase by 1, got %d", len(bc.Chain))
	}

	newBlock := bc.Chain[len(bc.Chain)-1]

	if newBlock.Index != initialChainLength {
		t.Errorf("Expected new block index to be %d, got %d", initialChainLength, newBlock.Index)
	}

	if newBlock.PrevHash != bc.Chain[initialChainLength-1].Hash {
		t.Errorf("New block's previous hash does not match the last block's hash. Expected: %s, Got: %s",
			bc.Chain[initialChainLength-1].Hash, newBlock.PrevHash)
	}

	if len(newBlock.Messages) != 1 {
		t.Errorf("Expected new block to have 1 message, got %d", len(newBlock.Messages))
	}

	if !strings.HasPrefix(newBlock.Hash, strings.Repeat("0", bc.Difficulty)) {
		t.Errorf("New block's hash does not have the required number of leading zeros. Got: %s", newBlock.Hash)
	}
}
