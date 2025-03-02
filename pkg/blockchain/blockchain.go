package blockchain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/freeeakn/AetherWave/pkg/crypto"
)

// Message представляет сообщение в блокчейне
type Message struct {
	Sender    string
	Recipient string
	Content   string
	Timestamp int64
}

// Block представляет блок в блокчейне
type Block struct {
	Index     int
	Timestamp int64
	Messages  []Message
	PrevHash  string
	Hash      string
	Nonce     int
}

// Blockchain представляет цепочку блоков
type Blockchain struct {
	Chain      []Block
	Difficulty int
	mutex      sync.RWMutex
}

// GenesisTimestamp - фиксированная временная метка для генезис-блока
const GenesisTimestamp = 1677654321

// NewBlockchain создает новый экземпляр блокчейна с генезис-блоком
func NewBlockchain() *Blockchain {
	genesisBlock := Block{
		Index:     0,
		Timestamp: GenesisTimestamp,
		Messages:  []Message{},
		PrevHash:  "0",
	}
	genesisBlock.Hash = CalculateHash(genesisBlock)
	return &Blockchain{
		Chain:      []Block{genesisBlock},
		Difficulty: 4,
	}
}

// CalculateHash вычисляет хеш блока
func CalculateHash(block Block) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d%d%v%s%d", block.Index, block.Timestamp, block.Messages, block.PrevHash, block.Nonce)
	h := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(h[:])
}

// AddMessage добавляет новое сообщение в блокчейн
func (bc *Blockchain) AddMessage(sender, recipient, content string, key []byte) error {
	bc.mutex.Lock()
	defer bc.mutex.Unlock()

	encryptedContent, err := crypto.EncryptMessage(content, key)
	if err != nil {
		return fmt.Errorf("encryption error: %v", err)
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

	newBlock = MineBlock(newBlock, bc.Difficulty)
	bc.Chain = append(bc.Chain, newBlock)
	return nil
}

// ReadMessages читает сообщения для указанного получателя
func (bc *Blockchain) ReadMessages(recipient string, key []byte) []string {
	bc.mutex.RLock()
	defer bc.mutex.RUnlock()

	var messages []string
	for _, block := range bc.Chain {
		for _, msg := range block.Messages {
			if msg.Recipient == recipient {
				encryptedBytes, err := hex.DecodeString(msg.Content)
				if err != nil {
					continue
				}
				decrypted, err := crypto.DecryptMessage(encryptedBytes, key)
				if err != nil {
					continue
				}
				messages = append(messages, fmt.Sprintf("From %s: %s", msg.Sender, decrypted))
			}
		}
	}
	return messages
}

// MineBlock выполняет майнинг блока с заданной сложностью
func MineBlock(block Block, difficulty int) Block {
	target := strings.Repeat("0", difficulty)
	for {
		hash := CalculateHash(block)
		if hash[:difficulty] == target {
			block.Hash = hash
			return block
		}
		block.Nonce++
	}
}

// VerifyChain проверяет целостность блокчейна
func (bc *Blockchain) VerifyChain() bool {
	bc.mutex.RLock()
	defer bc.mutex.RUnlock()

	for i := 1; i < len(bc.Chain); i++ {
		current := bc.Chain[i]
		previous := bc.Chain[i-1]
		if current.Hash != CalculateHash(current) || current.PrevHash != previous.Hash {
			return false
		}
	}
	return true
}

// GetLastBlock возвращает последний блок в цепочке
func (bc *Blockchain) GetLastBlock() Block {
	bc.mutex.RLock()
	defer bc.mutex.RUnlock()
	return bc.Chain[len(bc.Chain)-1]
}

// GetChain возвращает копию всей цепочки блоков
func (bc *Blockchain) GetChain() []Block {
	bc.mutex.RLock()
	defer bc.mutex.RUnlock()

	chainCopy := make([]Block, len(bc.Chain))
	copy(chainCopy, bc.Chain)
	return chainCopy
}

// UpdateChain обновляет цепочку блоков, если новая цепочка длиннее и валидна
func (bc *Blockchain) UpdateChain(newChain []Block) bool {
	bc.mutex.Lock()
	defer bc.mutex.Unlock()

	if len(newChain) <= len(bc.Chain) {
		return false
	}

	valid := true
	expectedPrevHash := "0"
	for _, block := range newChain {
		if block.PrevHash != expectedPrevHash {
			valid = false
			break
		}
		if block.Hash != CalculateHash(block) {
			valid = false
			break
		}
		expectedPrevHash = block.Hash
	}

	if valid {
		bc.Chain = newChain
		return true
	}
	return false
}
