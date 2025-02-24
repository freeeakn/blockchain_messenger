package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"
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

	if addedMessage.Content == content {
		t.Error("Message content was not encrypted")
	}

	encryptedBytes, err := hex.DecodeString(addedMessage.Content)
	if err != nil {
		t.Fatalf("Failed to decode encrypted content: %v", err)
	}

	decryptedContent, err := decryptMessage(encryptedBytes, key)
	if err != nil {
		t.Fatalf("Failed to decrypt message: %v", err)
	}

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

	expectedPrefix := strings.Repeat("0", bc.Difficulty)
	if !strings.HasPrefix(newBlock.Hash, expectedPrefix) {
		t.Errorf("New block hash does not have the required difficulty. Expected prefix: %s, Got: %s",
			expectedPrefix, newBlock.Hash[:bc.Difficulty])
	}

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

func TestReadMessagesNoMessagesForRecipient(t *testing.T) {
	bc := NewBlockchain()
	recipient := "Alice"
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	// Add a message for a different recipient
	bc.AddMessage("Bob", "Charlie", "Hello Charlie!", key)

	messages := bc.ReadMessages(recipient, key)

	if len(messages) != 0 {
		t.Errorf("Expected empty slice of messages, got %d messages", len(messages))
	}
}

func TestReadMessagesMultipleMessagesAcrossBlocks(t *testing.T) {
	bc := NewBlockchain()
	recipient := "Alice"
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	// Add messages for Alice in different blocks
	bc.AddMessage("Bob", recipient, "Hello Alice!", key)
	bc.AddMessage("Charlie", recipient, "How are you?", key)
	bc.AddMessage("David", "Eve", "This shouldn't be read", key)
	bc.AddMessage("Frank", recipient, "Nice to meet you!", key)

	messages := bc.ReadMessages(recipient, key)

	expectedCount := 3
	if len(messages) != expectedCount {
		t.Errorf("Expected %d messages, got %d", expectedCount, len(messages))
	}

	expectedMessages := []string{
		"From Bob: Hello Alice!",
		"From Charlie: How are you?",
		"From Frank: Nice to meet you!",
	}

	for i, expectedMsg := range expectedMessages {
		if i >= len(messages) {
			t.Errorf("Missing expected message: %s", expectedMsg)
			continue
		}
		if messages[i] != expectedMsg {
			t.Errorf("Message mismatch. Expected: %s, Got: %s", expectedMsg, messages[i])
		}
	}
}

func TestReadMessagesIgnoresInvalidHexContent(t *testing.T) {
	bc := NewBlockchain()
	recipient := "Alice"
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	// Add a valid message
	bc.AddMessage("Bob", recipient, "Valid message", key)

	// Manually add a block with an invalid hex-encoded message
	invalidBlock := Block{
		Index:     len(bc.Chain),
		Timestamp: time.Now().Unix(),
		Messages: []Message{
			{
				Sender:    "Charlie",
				Recipient: recipient,
				Content:   "This is not valid hex",
				Timestamp: time.Now().Unix(),
			},
		},
		PrevHash: bc.Chain[len(bc.Chain)-1].Hash,
	}
	invalidBlock.Hash = calculateHash(invalidBlock)
	bc.Chain = append(bc.Chain, invalidBlock)

	messages := bc.ReadMessages(recipient, key)

	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}

	expectedMessage := "From Bob: Valid message"
	if messages[0] != expectedMessage {
		t.Errorf("Expected message '%s', got '%s'", expectedMessage, messages[0])
	}
}

func TestReadMessagesHandlesDecryptionErrors(t *testing.T) {
	bc := NewBlockchain()
	recipient := "Alice"
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	bc.AddMessage("Bob", recipient, "Valid message", key)

	// Manually add a block with an invalid encrypted message
	invalidBlock := Block{
		Index:     len(bc.Chain),
		Timestamp: time.Now().Unix(),
		Messages: []Message{
			{
				Sender:    "Charlie",
				Recipient: recipient,
				Content:   hex.EncodeToString([]byte("Invalid encrypted content")),
				Timestamp: time.Now().Unix(),
			},
		},
		PrevHash: bc.Chain[len(bc.Chain)-1].Hash,
	}
	invalidBlock.Hash = calculateHash(invalidBlock)
	bc.Chain = append(bc.Chain, invalidBlock)

	// Add another valid message
	bc.AddMessage("David", recipient, "Another valid message", key)

	messages := bc.ReadMessages(recipient, key)

	expectedCount := 2
	if len(messages) != expectedCount {
		t.Errorf("Expected %d messages, got %d", expectedCount, len(messages))
	}

	expectedMessages := []string{
		"From Bob: Valid message",
		"From David: Another valid message",
	}

	for i, expectedMsg := range expectedMessages {
		if i >= len(messages) {
			t.Errorf("Missing expected message: %s", expectedMsg)
			continue
		}
		if messages[i] != expectedMsg {
			t.Errorf("Message mismatch. Expected: %s, Got: %s", expectedMsg, messages[i])
		}
	}
}

func TestReadMessagesFormatsOutputCorrectly(t *testing.T) {
	bc := NewBlockchain()
	recipient := "Alice"
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	sender := "Bob"
	content := "Hello, Alice!"
	bc.AddMessage(sender, recipient, content, key)

	messages := bc.ReadMessages(recipient, key)

	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}

	expectedFormat := fmt.Sprintf("From %s: %s", sender, content)
	if messages[0] != expectedFormat {
		t.Errorf("Message format incorrect. Expected: %s, Got: %s", expectedFormat, messages[0])
	}
}

func TestReadMessagesNonConsecutiveBlocks(t *testing.T) {
	bc := NewBlockchain()
	recipient := "Alice"
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	// Add messages for Alice in non-consecutive blocks
	bc.AddMessage("Bob", recipient, "Message 1", key)
	bc.AddMessage("Charlie", "Dave", "Unrelated message", key)
	bc.AddMessage("Eve", recipient, "Message 2", key)
	bc.AddMessage("Frank", "George", "Another unrelated message", key)
	bc.AddMessage("Harry", recipient, "Message 3", key)

	messages := bc.ReadMessages(recipient, key)

	expectedCount := 3
	if len(messages) != expectedCount {
		t.Errorf("Expected %d messages, got %d", expectedCount, len(messages))
	}

	expectedMessages := []string{
		"From Bob: Message 1",
		"From Eve: Message 2",
		"From Harry: Message 3",
	}

	for i, expectedMsg := range expectedMessages {
		if i >= len(messages) {
			t.Errorf("Missing expected message: %s", expectedMsg)
			continue
		}
		if messages[i] != expectedMsg {
			t.Errorf("Message mismatch. Expected: %s, Got: %s", expectedMsg, messages[i])
		}
	}
}

func TestReadMessagesMultipleRecipients(t *testing.T) {
	bc := NewBlockchain()
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	// Add messages for different recipients
	bc.AddMessage("Alice", "Bob", "Message for Bob", key)
	bc.AddMessage("Charlie", "Dave", "Message for Dave", key)
	bc.AddMessage("Eve", "Bob", "Another message for Bob", key)
	bc.AddMessage("Frank", "Alice", "Message for Alice", key)

	// Read messages for Bob
	bobMessages := bc.ReadMessages("Bob", key)
	if len(bobMessages) != 2 {
		t.Errorf("Expected 2 messages for Bob, got %d", len(bobMessages))
	}
	expectedBobMessages := []string{
		"From Alice: Message for Bob",
		"From Eve: Another message for Bob",
	}
	for i, msg := range bobMessages {
		if msg != expectedBobMessages[i] {
			t.Errorf("Unexpected message for Bob. Expected: %s, Got: %s", expectedBobMessages[i], msg)
		}
	}

	// Read messages for Dave
	daveMessages := bc.ReadMessages("Dave", key)
	if len(daveMessages) != 1 {
		t.Errorf("Expected 1 message for Dave, got %d", len(daveMessages))
	}
	if daveMessages[0] != "From Charlie: Message for Dave" {
		t.Errorf("Unexpected message for Dave. Expected: %s, Got: %s", "From Charlie: Message for Dave", daveMessages[0])
	}

	// Read messages for Alice
	aliceMessages := bc.ReadMessages("Alice", key)
	if len(aliceMessages) != 1 {
		t.Errorf("Expected 1 message for Alice, got %d", len(aliceMessages))
	}
	if aliceMessages[0] != "From Frank: Message for Alice" {
		t.Errorf("Unexpected message for Alice. Expected: %s, Got: %s", "From Frank: Message for Alice", aliceMessages[0])
	}
}

func TestReadMessagesOrderPreservation(t *testing.T) {
	bc := NewBlockchain()
	recipient := "Alice"
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	// Add messages in a specific order
	messages := []struct {
		sender  string
		content string
	}{
		{"Bob", "First message"},
		{"Charlie", "Second message"},
		{"David", "Third message"},
	}

	for _, msg := range messages {
		bc.AddMessage(msg.sender, recipient, msg.content, key)
	}

	// Read messages for Alice
	receivedMessages := bc.ReadMessages(recipient, key)

	if len(receivedMessages) != len(messages) {
		t.Errorf("Expected %d messages, got %d", len(messages), len(receivedMessages))
	}

	for i, expectedMsg := range messages {
		expected := fmt.Sprintf("From %s: %s", expectedMsg.sender, expectedMsg.content)
		if receivedMessages[i] != expected {
			t.Errorf("Message at index %d does not match. Expected: %s, Got: %s", i, expected, receivedMessages[i])
		}
	}
}

func TestReadMessagesCaseSensitiveRecipient(t *testing.T) {
	bc := NewBlockchain()
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	// Add messages for different recipients with case variations
	bc.AddMessage("Alice", "Bob", "Message for Bob", key)
	bc.AddMessage("Charlie", "bob", "Message for lowercase bob", key)
	bc.AddMessage("David", "BOB", "Message for uppercase BOB", key)

	// Read messages for "Bob" (exact case match)
	bobMessages := bc.ReadMessages("Bob", key)

	if len(bobMessages) != 1 {
		t.Errorf("Expected 1 message for 'Bob', got %d", len(bobMessages))
	}

	expectedMessage := "From Alice: Message for Bob"
	if len(bobMessages) > 0 && bobMessages[0] != expectedMessage {
		t.Errorf("Unexpected message for 'Bob'. Expected: %s, Got: %s", expectedMessage, bobMessages[0])
	}

	// Verify no messages for "bob" or "BOB"
	lowercaseBobMessages := bc.ReadMessages("bob", key)
	if len(lowercaseBobMessages) != 0 {
		t.Errorf("Expected 0 messages for 'bob', got %d", len(lowercaseBobMessages))
	}

	uppercaseBobMessages := bc.ReadMessages("BOB", key)
	if len(uppercaseBobMessages) != 0 {
		t.Errorf("Expected 0 messages for 'BOB', got %d", len(uppercaseBobMessages))
	}
}

func TestMineBlockLowDifficulty(t *testing.T) {
	block := Block{
		Index:     1,
		Timestamp: time.Now().Unix(),
		Messages:  []Message{},
		PrevHash:  "previoushash",
		Nonce:     0,
	}
	difficulty := 1

	minedBlock := mineBlock(block, difficulty)

	if !strings.HasPrefix(minedBlock.Hash, "0") {
		t.Errorf("Mined block hash does not have the required difficulty. Expected prefix: '0', Got: %s", minedBlock.Hash[:1])
	}

	if minedBlock.Hash != calculateHash(minedBlock) {
		t.Errorf("Mined block hash is invalid. Expected: %s, Got: %s", calculateHash(minedBlock), minedBlock.Hash)
	}

	if minedBlock.Nonce == 0 {
		t.Error("Block nonce was not incremented during mining")
	}
}

func TestMineBlockHighDifficulty(t *testing.T) {
	block := Block{
		Index:     1,
		Timestamp: time.Now().Unix(),
		Messages:  []Message{},
		PrevHash:  "previoushash",
		Nonce:     0,
	}
	difficulty := 8

	minedBlock := mineBlock(block, difficulty)

	if !strings.HasPrefix(minedBlock.Hash, strings.Repeat("0", difficulty)) {
		t.Errorf("Mined block hash does not have the required difficulty. Expected prefix: '%s', Got: %s",
			strings.Repeat("0", difficulty), minedBlock.Hash[:difficulty])
	}

	if minedBlock.Hash != calculateHash(minedBlock) {
		t.Errorf("Mined block hash is invalid. Expected: %s, Got: %s", calculateHash(minedBlock), minedBlock.Hash)
	}

	if minedBlock.Nonce == 0 {
		t.Error("Block nonce was not incremented during mining")
	}

	// Check that the mining process took a significant amount of time
	start := time.Now()
	mineBlock(block, difficulty)
	duration := time.Since(start)

	if duration < 100*time.Millisecond {
		t.Errorf("Mining with high difficulty was too quick. Duration: %v", duration)
	}
}

func TestMineBlockEmptyContent(t *testing.T) {
	emptyBlock := Block{
		Index:     1,
		Timestamp: time.Now().Unix(),
		Messages:  []Message{},
		PrevHash:  "previoushash",
		Nonce:     0,
	}
	difficulty := 4

	minedBlock := mineBlock(emptyBlock, difficulty)

	if !strings.HasPrefix(minedBlock.Hash, strings.Repeat("0", difficulty)) {
		t.Errorf("Mined block hash does not have the required difficulty. Expected prefix: '%s', Got: %s",
			strings.Repeat("0", difficulty), minedBlock.Hash[:difficulty])
	}

	if minedBlock.Hash != calculateHash(minedBlock) {
		t.Errorf("Mined block hash is invalid. Expected: %s, Got: %s", calculateHash(minedBlock), minedBlock.Hash)
	}

	if minedBlock.Nonce == 0 {
		t.Error("Block nonce was not incremented during mining")
	}

	if len(minedBlock.Messages) != 0 {
		t.Errorf("Expected empty Messages slice, got %d messages", len(minedBlock.Messages))
	}
}

func TestMineBlockUpdatesNonce(t *testing.T) {
	block := Block{
		Index:     1,
		Timestamp: time.Now().Unix(),
		Messages:  []Message{},
		PrevHash:  "previoushash",
		Nonce:     0,
	}
	difficulty := 4

	minedBlock := mineBlock(block, difficulty)

	if minedBlock.Nonce == 0 {
		t.Error("Block nonce was not updated during mining")
	}

	if minedBlock.Nonce <= block.Nonce {
		t.Errorf("Block nonce was not incremented. Initial: %d, After mining: %d", block.Nonce, minedBlock.Nonce)
	}

	if !strings.HasPrefix(minedBlock.Hash, strings.Repeat("0", difficulty)) {
		t.Errorf("Mined block hash does not have the required difficulty. Expected prefix: %s, Got: %s",
			strings.Repeat("0", difficulty), minedBlock.Hash[:difficulty])
	}
}

func TestMineBlockWithPrefilledNonce(t *testing.T) {
	block := Block{
		Index:     1,
		Timestamp: time.Now().Unix(),
		Messages:  []Message{},
		PrevHash:  "previoushash",
		Nonce:     1000, // Pre-filled Nonce value
	}
	difficulty := 4

	minedBlock := mineBlock(block, difficulty)

	if !strings.HasPrefix(minedBlock.Hash, strings.Repeat("0", difficulty)) {
		t.Errorf("Mined block hash does not have the required difficulty. Expected prefix: '%s', Got: %s",
			strings.Repeat("0", difficulty), minedBlock.Hash[:difficulty])
	}

	if minedBlock.Hash != calculateHash(minedBlock) {
		t.Errorf("Mined block hash is invalid. Expected: %s, Got: %s", calculateHash(minedBlock), minedBlock.Hash)
	}

	if minedBlock.Nonce <= block.Nonce {
		t.Errorf("Block nonce was not incremented. Initial: %d, After mining: %d", block.Nonce, minedBlock.Nonce)
	}
}

func TestMineBlockPerformanceWithDifferentBlockSizes(t *testing.T) {
	difficulty := 4
	sizes := []int{1, 10, 100, 1000}

	for _, size := range sizes {
		messages := make([]Message, size)
		for i := 0; i < size; i++ {
			messages[i] = Message{
				Sender:    fmt.Sprintf("Sender%d", i),
				Recipient: fmt.Sprintf("Recipient%d", i),
				Content:   fmt.Sprintf("Content%d", i),
				Timestamp: time.Now().Unix(),
			}
		}

		block := Block{
			Index:     1,
			Timestamp: time.Now().Unix(),
			Messages:  messages,
			PrevHash:  "previoushash",
			Nonce:     0,
		}

		start := time.Now()
		minedBlock := mineBlock(block, difficulty)
		duration := time.Since(start)

		if !strings.HasPrefix(minedBlock.Hash, strings.Repeat("0", difficulty)) {
			t.Errorf("Mined block hash does not have the required difficulty for size %d. Hash: %s", size, minedBlock.Hash)
		}

		t.Logf("Mining time for block with %d messages: %v", size, duration)

		if size > 1 && duration < 10*time.Millisecond {
			t.Errorf("Mining time for block with %d messages was unexpectedly short: %v", size, duration)
		}
	}
}

func TestMineBlockMaximumDifficulty(t *testing.T) {
	block := Block{
		Index:     1,
		Timestamp: time.Now().Unix(),
		Messages:  []Message{},
		PrevHash:  "previoushash",
		Nonce:     0,
	}
	difficulty := 64

	start := time.Now()
	minedBlock := mineBlock(block, difficulty)
	duration := time.Since(start)

	if !strings.HasPrefix(minedBlock.Hash, strings.Repeat("0", difficulty)) {
		t.Errorf("Mined block hash does not have the required difficulty. Expected prefix: '%s', Got: %s",
			strings.Repeat("0", difficulty), minedBlock.Hash)
	}

	if minedBlock.Hash != calculateHash(minedBlock) {
		t.Errorf("Mined block hash is invalid. Expected: %s, Got: %s", calculateHash(minedBlock), minedBlock.Hash)
	}

	if minedBlock.Nonce == 0 {
		t.Error("Block nonce was not incremented during mining")
	}

	t.Logf("Mining time for maximum difficulty: %v", duration)
	t.Logf("Final nonce: %d", minedBlock.Nonce)
	t.Logf("Mined hash: %s", minedBlock.Hash)
}
