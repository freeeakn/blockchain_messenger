package blockchain

import (
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

func TestNewBlockchain(t *testing.T) {
	bc := NewBlockchain()

	if bc == nil {
		t.Fatal("NewBlockchain() returned nil")
	}

	if len(bc.Chain) != 1 {
		t.Errorf("Expected blockchain to have 1 genesis block, got %d", len(bc.Chain))
	}

	genesisBlock := bc.Chain[0]
	if genesisBlock.Index != 0 {
		t.Errorf("Expected genesis block index to be 0, got %d", genesisBlock.Index)
	}

	if genesisBlock.PrevHash != "0" {
		t.Errorf("Expected genesis block PrevHash to be '0', got %s", genesisBlock.PrevHash)
	}

	if genesisBlock.Timestamp != GenesisTimestamp {
		t.Errorf("Expected genesis block timestamp to be %d, got %d", GenesisTimestamp, genesisBlock.Timestamp)
	}

	if len(genesisBlock.Messages) != 0 {
		t.Errorf("Expected genesis block to have 0 messages, got %d", len(genesisBlock.Messages))
	}

	if genesisBlock.Hash != CalculateHash(genesisBlock) {
		t.Errorf("Expected genesis block hash to match calculated hash")
	}
}

func TestCalculateHash(t *testing.T) {
	block := Block{
		Index:     1,
		Timestamp: 1677654322,
		Messages:  []Message{{Sender: "Alice", Recipient: "Bob", Content: "Test", Timestamp: 1677654322}},
		PrevHash:  "0000abcdef0123456789abcdef0123456789abcdef0123456789abcdef012345",
		Nonce:     42,
	}

	hash := CalculateHash(block)

	if len(hash) != 64 {
		t.Errorf("Expected hash length to be 64 characters, got %d", len(hash))
	}

	// Проверяем, что хеш изменяется при изменении данных блока
	block.Nonce = 43
	newHash := CalculateHash(block)

	if hash == newHash {
		t.Errorf("Expected hash to change when block data changes")
	}
}

func TestMineBlock(t *testing.T) {
	block := Block{
		Index:     1,
		Timestamp: time.Now().Unix(),
		Messages:  []Message{{Sender: "Alice", Recipient: "Bob", Content: "Test", Timestamp: time.Now().Unix()}},
		PrevHash:  "0000abcdef0123456789abcdef0123456789abcdef0123456789abcdef012345",
	}

	difficulty := 2
	minedBlock := MineBlock(block, difficulty)

	// Проверяем, что хеш начинается с нужного количества нулей
	if !strings.HasPrefix(minedBlock.Hash, strings.Repeat("0", difficulty)) {
		t.Errorf("Expected mined block hash to start with %d zeros, got %s", difficulty, minedBlock.Hash)
	}

	// Проверяем, что хеш соответствует блоку
	if minedBlock.Hash != CalculateHash(minedBlock) {
		t.Errorf("Expected mined block hash to match calculated hash")
	}
}

func TestAddMessage(t *testing.T) {
	bc := NewBlockchain()
	key := make([]byte, 32) // Нулевой ключ для тестирования

	err := bc.AddMessage("Alice", "Bob", "Hello, Bob!", key)
	if err != nil {
		t.Fatalf("Failed to add message: %v", err)
	}

	if len(bc.Chain) != 2 {
		t.Errorf("Expected blockchain to have 2 blocks after adding message, got %d", len(bc.Chain))
	}

	newBlock := bc.Chain[1]
	if newBlock.Index != 1 {
		t.Errorf("Expected new block index to be 1, got %d", newBlock.Index)
	}

	if len(newBlock.Messages) != 1 {
		t.Errorf("Expected new block to have 1 message, got %d", len(newBlock.Messages))
	}

	msg := newBlock.Messages[0]
	if msg.Sender != "Alice" {
		t.Errorf("Expected message sender to be 'Alice', got %s", msg.Sender)
	}

	if msg.Recipient != "Bob" {
		t.Errorf("Expected message recipient to be 'Bob', got %s", msg.Recipient)
	}

	// Проверяем, что сообщение зашифровано
	_, err = hex.DecodeString(msg.Content)
	if err != nil {
		t.Errorf("Expected message content to be hex-encoded encrypted data, got error: %v", err)
	}
}

func TestReadMessages(t *testing.T) {
	bc := NewBlockchain()
	key := make([]byte, 32) // Нулевой ключ для тестирования

	// Добавляем несколько сообщений
	bc.AddMessage("Alice", "Bob", "Hello, Bob!", key)
	bc.AddMessage("Charlie", "Bob", "Hi Bob, it's Charlie", key)
	bc.AddMessage("Alice", "Charlie", "Hello, Charlie!", key) // Это сообщение не для Bob

	// Читаем сообщения для Bob
	messages := bc.ReadMessages("Bob", key)

	if len(messages) != 2 {
		t.Errorf("Expected to read 2 messages for Bob, got %d", len(messages))
	}

	// Проверяем содержимое сообщений
	expectedPrefixes := []string{"From Alice:", "From Charlie:"}
	expectedContents := []string{"Hello, Bob!", "Hi Bob, it's Charlie"}

	for i, msg := range messages {
		if !strings.HasPrefix(msg, expectedPrefixes[i]) {
			t.Errorf("Expected message %d to start with '%s', got '%s'", i, expectedPrefixes[i], msg)
		}

		if !strings.Contains(msg, expectedContents[i]) {
			t.Errorf("Expected message %d to contain '%s', got '%s'", i, expectedContents[i], msg)
		}
	}

	// Проверяем чтение с неправильным ключом
	// Для AES неправильный ключ может привести к успешной расшифровке, но с неправильными данными
	// Поэтому мы не проверяем количество сообщений, а проверяем, что содержимое отличается
	wrongKey := make([]byte, 32)
	wrongKey[0] = 1 // Изменяем ключ

	wrongMessages := bc.ReadMessages("Bob", wrongKey)

	// Проверяем, что расшифрованные сообщения отличаются от оригинальных
	if len(wrongMessages) > 0 {
		for i, msg := range wrongMessages {
			if i < len(messages) && msg == messages[i] {
				t.Errorf("Expected message with wrong key to be different from original message")
			}
		}
	}
}

func TestVerifyChain(t *testing.T) {
	bc := NewBlockchain()
	key := make([]byte, 32)

	// Добавляем несколько сообщений для создания блоков
	bc.AddMessage("Alice", "Bob", "Message 1", key)
	bc.AddMessage("Bob", "Alice", "Message 2", key)

	// Проверяем, что цепочка валидна
	if !bc.VerifyChain() {
		t.Errorf("Expected chain to be valid")
	}

	// Изменяем данные в блоке и проверяем, что цепочка становится невалидной
	bc.mutex.Lock()
	bc.Chain[1].Messages[0].Content = "Tampered content"
	bc.mutex.Unlock()

	if bc.VerifyChain() {
		t.Errorf("Expected chain to be invalid after tampering")
	}
}

func TestGetLastBlock(t *testing.T) {
	bc := NewBlockchain()
	key := make([]byte, 32)

	// Добавляем сообщение для создания нового блока
	bc.AddMessage("Alice", "Bob", "Test message", key)

	lastBlock := bc.GetLastBlock()

	if lastBlock.Index != 1 {
		t.Errorf("Expected last block index to be 1, got %d", lastBlock.Index)
	}

	if len(lastBlock.Messages) != 1 || lastBlock.Messages[0].Sender != "Alice" {
		t.Errorf("Expected last block to contain message from Alice")
	}
}

func TestGetChain(t *testing.T) {
	bc := NewBlockchain()
	key := make([]byte, 32)

	// Добавляем сообщение для создания нового блока
	bc.AddMessage("Alice", "Bob", "Test message", key)

	chain := bc.GetChain()

	if len(chain) != 2 {
		t.Errorf("Expected chain length to be 2, got %d", len(chain))
	}

	// Проверяем, что GetChain возвращает копию цепочки
	chain[0].Index = 999

	if bc.Chain[0].Index == 999 {
		t.Errorf("Expected GetChain to return a copy of the chain, not a reference")
	}
}

func TestUpdateChain(t *testing.T) {
	bc := NewBlockchain()

	// Создаем новую, более длинную цепочку
	longerChain := make([]Block, 3)
	longerChain[0] = bc.Chain[0] // Генезис-блок

	// Добавляем два новых блока
	longerChain[1] = Block{
		Index:     1,
		Timestamp: time.Now().Unix(),
		Messages:  []Message{{Sender: "Alice", Recipient: "Bob", Content: "Test1", Timestamp: time.Now().Unix()}},
		PrevHash:  longerChain[0].Hash,
	}
	longerChain[1].Hash = CalculateHash(longerChain[1])

	longerChain[2] = Block{
		Index:     2,
		Timestamp: time.Now().Unix(),
		Messages:  []Message{{Sender: "Bob", Recipient: "Alice", Content: "Test2", Timestamp: time.Now().Unix()}},
		PrevHash:  longerChain[1].Hash,
	}
	longerChain[2].Hash = CalculateHash(longerChain[2])

	// Обновляем цепочку
	result := bc.UpdateChain(longerChain)

	if !result {
		t.Errorf("Expected UpdateChain to return true for valid longer chain")
	}

	if len(bc.Chain) != 3 {
		t.Errorf("Expected chain length to be 3 after update, got %d", len(bc.Chain))
	}

	// Проверяем, что не обновляется на более короткую цепочку
	shorterChain := make([]Block, 1)
	shorterChain[0] = bc.Chain[0]

	result = bc.UpdateChain(shorterChain)

	if result {
		t.Errorf("Expected UpdateChain to return false for shorter chain")
	}

	if len(bc.Chain) != 3 {
		t.Errorf("Expected chain length to still be 3 after failed update, got %d", len(bc.Chain))
	}

	// Проверяем, что не обновляется на невалидную цепочку
	invalidChain := make([]Block, 4)
	copy(invalidChain, longerChain)

	invalidChain[3] = Block{
		Index:     3,
		Timestamp: time.Now().Unix(),
		Messages:  []Message{{Sender: "Charlie", Recipient: "Dave", Content: "Test3", Timestamp: time.Now().Unix()}},
		PrevHash:  "invalid_prev_hash", // Неправильный хеш предыдущего блока
	}
	invalidChain[3].Hash = CalculateHash(invalidChain[3])

	result = bc.UpdateChain(invalidChain)

	if result {
		t.Errorf("Expected UpdateChain to return false for invalid chain")
	}

	if len(bc.Chain) != 3 {
		t.Errorf("Expected chain length to still be 3 after failed update, got %d", len(bc.Chain))
	}
}
