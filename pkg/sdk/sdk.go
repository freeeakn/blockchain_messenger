// Package sdk предоставляет возможности для интеграции с блокчейном AetherWave
package sdk

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/freeeakn/AetherWave/pkg/crypto"
)

// ClientOptions содержит настройки клиента
type ClientOptions struct {
	NodeURL    string
	Timeout    time.Duration
	MaxRetries int
	RetryDelay time.Duration
}

// Message представляет сообщение в блокчейне
type Message struct {
	Sender     string
	Recipient  string
	Content    string
	TimeString string
	Time       time.Time
	Encrypted  bool
}

// BlockInfo представляет информацию о блоке
type BlockInfo struct {
	Index     int
	Hash      string
	PrevHash  string
	Timestamp time.Time
	Nonce     int
	Messages  []Message
}

// BlockchainInfo представляет общую информацию о блокчейне
type BlockchainInfo struct {
	BlockCount   int
	MessageCount int
	Difficulty   int
	LastBlock    BlockInfo
}

// PeerInfo представляет информацию о пире в сети
type PeerInfo struct {
	Address  string
	LastSeen time.Time
	Active   bool
}

// Client предоставляет API для взаимодействия с блокчейном AetherWave
type Client struct {
	nodeURL       string
	httpClient    *http.Client
	encryptionKey []byte
	username      string
	maxRetries    int
	retryDelay    time.Duration
}

// NewClient создает новый экземпляр клиента SDK
func NewClient(options ClientOptions) *Client {
	// Устанавливаем значения по умолчанию, если они не указаны
	if options.Timeout == 0 {
		options.Timeout = 10 * time.Second
	}
	if options.MaxRetries == 0 {
		options.MaxRetries = 3
	}
	if options.RetryDelay == 0 {
		options.RetryDelay = 1 * time.Second
	}

	return &Client{
		nodeURL: options.NodeURL,
		httpClient: &http.Client{
			Timeout: options.Timeout,
		},
		maxRetries: options.MaxRetries,
		retryDelay: options.RetryDelay,
	}
}

// SetEncryptionKey устанавливает ключ шифрования для сообщений
func (c *Client) SetEncryptionKey(key string) error {
	// Преобразуем Base64-encoded строку в []byte
	keyBytes, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return fmt.Errorf("ошибка декодирования ключа: %v", err)
	}
	c.encryptionKey = keyBytes
	return nil
}

// SetUsername устанавливает имя пользователя
func (c *Client) SetUsername(username string) {
	c.username = username
}

// GetUsername возвращает текущее имя пользователя
func (c *Client) GetUsername() string {
	return c.username
}

// GenerateEncryptionKey генерирует новый ключ шифрования
func (c *Client) GenerateEncryptionKey() (string, error) {
	key, err := crypto.GenerateKey()
	if err != nil {
		return "", fmt.Errorf("ошибка генерации ключа: %v", err)
	}
	c.encryptionKey = key
	// Преобразуем []byte в Base64-encoded строку для удобства передачи
	keyString := base64.StdEncoding.EncodeToString(key)
	return keyString, nil
}

// GetMessages получает сообщения для текущего пользователя
func (c *Client) GetMessages() ([]Message, error) {
	if c.username == "" {
		return nil, errors.New("имя пользователя не установлено")
	}

	endpoint := fmt.Sprintf("%s/messages/%s", c.nodeURL, c.username)
	var messages []Message

	err := c.makeRequest("GET", endpoint, nil, &messages)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения сообщений: %v", err)
	}

	// Расшифровываем сообщения, если установлен ключ
	if c.encryptionKey != nil {
		for i, msg := range messages {
			if msg.Encrypted {
				// Преобразуем строку в []byte для дешифрования
				contentBytes, err := base64.StdEncoding.DecodeString(msg.Content)
				if err != nil {
					continue
				}

				decrypted, err := crypto.DecryptMessage(contentBytes, c.encryptionKey)
				if err == nil {
					messages[i].Content = string(decrypted)
				}
			}
		}
	}

	return messages, nil
}

// SendMessage отправляет сообщение
func (c *Client) SendMessage(recipient, content string) error {
	if c.username == "" {
		return errors.New("имя пользователя не установлено")
	}

	// Шифруем сообщение, если установлен ключ
	encryptedContent := content
	encrypted := false

	if c.encryptionKey != nil {
		var err error
		encryptedBytes, err := crypto.EncryptMessage(content, c.encryptionKey)
		if err != nil {
			return fmt.Errorf("ошибка шифрования сообщения: %v", err)
		}
		// Преобразуем []byte в Base64-encoded строку для хранения
		encryptedContent = base64.StdEncoding.EncodeToString(encryptedBytes)
		encrypted = true
	}

	message := struct {
		Sender    string `json:"sender"`
		Recipient string `json:"recipient"`
		Content   string `json:"content"`
		Encrypted bool   `json:"encrypted"`
	}{
		Sender:    c.username,
		Recipient: recipient,
		Content:   encryptedContent,
		Encrypted: encrypted,
	}

	endpoint := fmt.Sprintf("%s/messages", c.nodeURL)
	err := c.makeRequest("POST", endpoint, message, nil)
	if err != nil {
		return fmt.Errorf("ошибка отправки сообщения: %v", err)
	}

	return nil
}

// GetBlockchainInfo получает информацию о блокчейне
func (c *Client) GetBlockchainInfo() (BlockchainInfo, error) {
	endpoint := fmt.Sprintf("%s/blockchain/info", c.nodeURL)
	var info BlockchainInfo

	err := c.makeRequest("GET", endpoint, nil, &info)
	if err != nil {
		return BlockchainInfo{}, fmt.Errorf("ошибка получения информации о блокчейне: %v", err)
	}

	return info, nil
}

// GetPeers получает список пиров в сети
func (c *Client) GetPeers() ([]PeerInfo, error) {
	endpoint := fmt.Sprintf("%s/peers", c.nodeURL)
	var peers []PeerInfo

	err := c.makeRequest("GET", endpoint, nil, &peers)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения списка пиров: %v", err)
	}

	return peers, nil
}

// AddPeer добавляет новый пир в сеть
func (c *Client) AddPeer(address string) error {
	peer := struct {
		Address string `json:"address"`
	}{
		Address: address,
	}

	endpoint := fmt.Sprintf("%s/peers", c.nodeURL)
	err := c.makeRequest("POST", endpoint, peer, nil)
	if err != nil {
		return fmt.Errorf("ошибка добавления пира: %v", err)
	}

	return nil
}

// Внутренние вспомогательные методы

// makeRequest выполняет HTTP-запрос к API узла
func (c *Client) makeRequest(method, url string, body interface{}, result interface{}) error {
	var reqBody []byte
	var err error
	var req *http.Request

	if body != nil {
		reqBody, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("ошибка маршалинга JSON: %w", err)
		}
		req, err = http.NewRequest(method, c.nodeURL+url, bytes.NewBuffer(reqBody))
	} else {
		req, err = http.NewRequest(method, c.nodeURL+url, nil)
	}

	if err != nil {
		return fmt.Errorf("ошибка создания запроса: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	var resp *http.Response
	var retries int

	for retries = 0; retries <= c.maxRetries; retries++ {
		resp, err = c.httpClient.Do(req)
		if err == nil {
			break
		}
		if retries == c.maxRetries {
			return fmt.Errorf("ошибка выполнения запроса после %d попыток: %w", retries, err)
		}
		time.Sleep(c.retryDelay)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ошибка API: %s (код %d)", string(bodyBytes), resp.StatusCode)
	}

	if result != nil {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("ошибка чтения ответа: %w", err)
		}
		err = json.Unmarshal(bodyBytes, result)
		if err != nil {
			return fmt.Errorf("ошибка демаршалинга JSON: %w", err)
		}
	}

	return nil
}
