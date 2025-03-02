package tests

import (
	"encoding/hex"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/freeeakn/AetherWave/pkg/blockchain"
	"github.com/freeeakn/AetherWave/pkg/crypto"
	"github.com/freeeakn/AetherWave/pkg/network"
)

// mockListener реализует интерфейс net.Listener для тестирования
type mockListener struct {
	connCh chan net.Conn
	closed bool
	mu     sync.Mutex
	addr   net.Addr
}

func newMockListener(addr string) *mockListener {
	return &mockListener{
		connCh: make(chan net.Conn, 10),
		addr:   &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080},
	}
}

func (l *mockListener) Accept() (net.Conn, error) {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return nil, fmt.Errorf("listener closed")
	}
	l.mu.Unlock()
	conn, ok := <-l.connCh
	if !ok {
		return nil, fmt.Errorf("listener closed")
	}
	return conn, nil
}

func (l *mockListener) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.closed {
		l.closed = true
		close(l.connCh)
	}
	return nil
}

func (l *mockListener) Addr() net.Addr {
	return l.addr
}

func (l *mockListener) AddConnection(conn net.Conn) {
	l.connCh <- conn
}

// mockConn реализует интерфейс net.Conn для тестирования
type mockConn struct {
	readBuf    chan []byte
	writeBuf   chan []byte
	closed     bool
	mu         sync.Mutex
	localAddr  net.Addr
	remoteAddr net.Addr
}

func newMockConn(localAddr, remoteAddr string) *mockConn {
	return &mockConn{
		readBuf:    make(chan []byte, 100),
		writeBuf:   make(chan []byte, 100),
		localAddr:  &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080},
		remoteAddr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8081},
	}
}

func (c *mockConn) Read(b []byte) (n int, err error) {
	select {
	case data := <-c.readBuf:
		n = copy(b, data)
		return n, nil
	case <-time.After(10 * time.Millisecond):
		return 0, fmt.Errorf("timeout")
	}
}

func (c *mockConn) Write(b []byte) (n int, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return 0, fmt.Errorf("connection closed")
	}
	c.writeBuf <- append([]byte{}, b...)
	return len(b), nil
}

func (c *mockConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

func (c *mockConn) LocalAddr() net.Addr                { return c.localAddr }
func (c *mockConn) RemoteAddr() net.Addr               { return c.remoteAddr }
func (c *mockConn) SetDeadline(t time.Time) error      { return nil }
func (c *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *mockConn) SetWriteDeadline(t time.Time) error { return nil }

// Переменная для замены функции net.Listen в тестах
var netListen = net.Listen

// Переменная для замены функции dialPeer в тестах
var dialPeer = func(address string) (net.Conn, error) {
	return nil, net.ErrClosed
}

// setupTestNodes создает несколько тестовых узлов для интеграционного тестирования
func setupTestNodes(t *testing.T, count int) []*network.Node {
	// Заменяем функции для тестирования
	originalNetListen := netListen
	originalDialPeer := network.DialPeer

	defer func() {
		netListen = originalNetListen
		network.DialPeer = originalDialPeer
	}()

	// Создаем мок-слушатели для каждого узла
	listeners := make([]*mockListener, count)
	for i := 0; i < count; i++ {
		listeners[i] = newMockListener("127.0.0.1:8080")
	}

	// Заменяем функцию net.Listen
	netListenIndex := 0
	netListen = func(network, address string) (net.Listener, error) {
		if netListenIndex < len(listeners) {
			listener := listeners[netListenIndex]
			netListenIndex++
			return listener, nil
		}
		return nil, fmt.Errorf("no more mock listeners")
	}

	// Создаем карту соединений между узлами
	connections := make(map[string]map[string]*mockConn)

	// Заменяем функцию dialPeer
	network.DialPeer = func(address string) (net.Conn, error) {
		for _, conns := range connections {
			if targetConn, ok := conns[address]; ok {
				return targetConn, nil
			}
		}

		return nil, fmt.Errorf("no mock connection for %s", address)
	}

	// Создаем узлы
	nodes := make([]*network.Node, count)
	for i := 0; i < count; i++ {
		bc := blockchain.NewBlockchain()
		address := fmt.Sprintf("127.0.0.1:%d", 8080+i)

		// Создаем список bootstrap-пиров для этого узла
		bootstrapPeers := make([]string, 0)
		if i > 0 {
			// Первый узел не имеет bootstrap-пиров
			for j := 0; j < i; j++ {
				bootstrapPeers = append(bootstrapPeers, fmt.Sprintf("127.0.0.1:%d", 8080+j))
			}
		}

		nodes[i] = network.NewNode(address, bc, bootstrapPeers)

		// Создаем соединения с другими узлами
		connections[address] = make(map[string]*mockConn)
		for j := 0; j < i; j++ {
			peerAddr := fmt.Sprintf("127.0.0.1:%d", 8080+j)

			// Создаем пару соединений между узлами
			conn1 := newMockConn(address, peerAddr)
			conn2 := newMockConn(peerAddr, address)

			// Связываем буферы чтения и записи
			conn1.readBuf = conn2.writeBuf
			conn2.readBuf = conn1.writeBuf

			// Сохраняем соединения
			connections[address][peerAddr] = conn1
			if _, ok := connections[peerAddr]; !ok {
				connections[peerAddr] = make(map[string]*mockConn)
			}
			connections[peerAddr][address] = conn2
		}
	}

	// Запускаем узлы
	for i, node := range nodes {
		go func(n *network.Node, index int) {
			err := n.Start()
			if err != nil {
				t.Errorf("Failed to start node %d: %v", index, err)
			}
		}(node, i)
	}

	// Даем время на запуск и установку соединений
	time.Sleep(500 * time.Millisecond)

	return nodes
}

// TestNodeCommunication проверяет обмен сообщениями между узлами
func TestNodeCommunication(t *testing.T) {
	// Сохраняем оригинальную функцию net.Listen
	originalNetListen := netListen
	defer func() { netListen = originalNetListen }()

	// Создаем мок-слушатели для узлов
	listener1 := newMockListener("127.0.0.1:8001")
	listener2 := newMockListener("127.0.0.1:8002")

	// Заменяем функцию net.Listen
	netListen = func(network, address string) (net.Listener, error) {
		if address == "127.0.0.1:8001" {
			return listener1, nil
		}
		return listener2, nil
	}

	// Заменяем функцию DialPeer
	originalDialPeer := network.DialPeer
	defer func() { network.DialPeer = originalDialPeer }()

	// Создаем мок-соединения для узлов
	conn1to2 := newMockConn("127.0.0.1:8001", "127.0.0.1:8002")
	conn2to1 := newMockConn("127.0.0.1:8002", "127.0.0.1:8001")

	// Настраиваем перенаправление данных между соединениями
	go func() {
		for {
			select {
			case data, ok := <-conn1to2.writeBuf:
				if !ok {
					return
				}
				conn2to1.readBuf <- data
			case data, ok := <-conn2to1.writeBuf:
				if !ok {
					return
				}
				conn1to2.readBuf <- data
			}
		}
	}()

	// Заменяем функцию DialPeer
	network.DialPeer = func(address string) (net.Conn, error) {
		if address == "127.0.0.1:8002" {
			return conn1to2, nil
		}
		if address == "127.0.0.1:8001" {
			return conn2to1, nil
		}
		return nil, fmt.Errorf("unknown address: %s", address)
	}

	// Создаем блокчейны и узлы
	bc1 := blockchain.NewBlockchain()
	bc2 := blockchain.NewBlockchain()

	node1 := network.NewNode("127.0.0.1:8001", bc1, []string{"127.0.0.1:8002"})
	node2 := network.NewNode("127.0.0.1:8002", bc2, []string{"127.0.0.1:8001"})

	// Запускаем узлы
	err := node1.Start()
	if err != nil {
		t.Fatalf("Failed to start node1: %v", err)
	}
	defer node1.Stop()

	err = node2.Start()
	if err != nil {
		t.Fatalf("Failed to start node2: %v", err)
	}
	defer node2.Stop()

	// Даем время на установление соединения
	time.Sleep(100 * time.Millisecond)

	// Генерируем ключ для шифрования
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	// Добавляем сообщение в первый узел
	err = bc1.AddMessage("Alice", "Bob", "Hello from node1!", key)
	if err != nil {
		t.Fatalf("Failed to add message to node1: %v", err)
	}

	// Получаем последний блок из первого узла
	lastBlock := bc1.GetLastBlock()

	// Отправляем блок второму узлу
	node1.BroadcastBlock(lastBlock)

	// Даем время на обработку блока
	time.Sleep(100 * time.Millisecond)

	// Проверяем, что блок был добавлен во второй узел
	lastBlock2 := bc2.GetLastBlock()
	if lastBlock2.Index != lastBlock.Index {
		t.Errorf("Expected node2 to have block with index %d, got %d", lastBlock.Index, lastBlock2.Index)
	}

	if lastBlock2.Hash != lastBlock.Hash {
		t.Errorf("Expected node2 to have block with hash %s, got %s", lastBlock.Hash, lastBlock2.Hash)
	}

	// Проверяем, что сообщение можно прочитать из второго узла
	messages := bc2.ReadMessages("Bob", key)
	if len(messages) != 1 {
		t.Errorf("Expected to read 1 message from node2, got %d", len(messages))
	}

	if len(messages) > 0 && !contains(messages[0], "Hello from node1!") {
		t.Errorf("Expected message to contain 'Hello from node1!', got %s", messages[0])
	}
}

// TestBlockchainSynchronization проверяет синхронизацию блокчейна между узлами
func TestBlockchainSynchronization(t *testing.T) {
	// Сохраняем оригинальную функцию net.Listen
	originalNetListen := netListen
	defer func() { netListen = originalNetListen }()

	// Создаем мок-слушатели для узлов
	listener1 := newMockListener("127.0.0.1:8001")
	listener2 := newMockListener("127.0.0.1:8002")
	listener3 := newMockListener("127.0.0.1:8003")

	// Заменяем функцию net.Listen
	netListen = func(network, address string) (net.Listener, error) {
		if address == "127.0.0.1:8001" {
			return listener1, nil
		}
		if address == "127.0.0.1:8002" {
			return listener2, nil
		}
		return listener3, nil
	}

	// Заменяем функцию DialPeer
	originalDialPeer := network.DialPeer
	defer func() { network.DialPeer = originalDialPeer }()

	// Создаем мок-соединения для узлов
	conn1to2 := newMockConn("127.0.0.1:8001", "127.0.0.1:8002")
	conn2to1 := newMockConn("127.0.0.1:8002", "127.0.0.1:8001")
	conn1to3 := newMockConn("127.0.0.1:8001", "127.0.0.1:8003")
	conn3to1 := newMockConn("127.0.0.1:8003", "127.0.0.1:8001")
	conn2to3 := newMockConn("127.0.0.1:8002", "127.0.0.1:8003")
	conn3to2 := newMockConn("127.0.0.1:8003", "127.0.0.1:8002")

	// Настраиваем перенаправление данных между соединениями
	go func() {
		for {
			select {
			case data, ok := <-conn1to2.writeBuf:
				if !ok {
					return
				}
				conn2to1.readBuf <- data
			case data, ok := <-conn2to1.writeBuf:
				if !ok {
					return
				}
				conn1to2.readBuf <- data
			case data, ok := <-conn1to3.writeBuf:
				if !ok {
					return
				}
				conn3to1.readBuf <- data
			case data, ok := <-conn3to1.writeBuf:
				if !ok {
					return
				}
				conn1to3.readBuf <- data
			case data, ok := <-conn2to3.writeBuf:
				if !ok {
					return
				}
				conn3to2.readBuf <- data
			case data, ok := <-conn3to2.writeBuf:
				if !ok {
					return
				}
				conn2to3.readBuf <- data
			}
		}
	}()

	// Заменяем функцию DialPeer
	network.DialPeer = func(address string) (net.Conn, error) {
		if address == "127.0.0.1:8001" {
			return conn3to1, nil
		}
		if address == "127.0.0.1:8002" {
			return conn1to2, nil
		}
		if address == "127.0.0.1:8003" {
			return conn2to3, nil
		}
		return nil, fmt.Errorf("unknown address: %s", address)
	}

	// Создаем блокчейны и узлы
	bc1 := blockchain.NewBlockchain()
	bc2 := blockchain.NewBlockchain()
	bc3 := blockchain.NewBlockchain()

	node1 := network.NewNode("127.0.0.1:8001", bc1, []string{"127.0.0.1:8002"})
	node2 := network.NewNode("127.0.0.1:8002", bc2, []string{"127.0.0.1:8003"})
	node3 := network.NewNode("127.0.0.1:8003", bc3, []string{"127.0.0.1:8001"})

	// Запускаем узлы
	err := node1.Start()
	if err != nil {
		t.Fatalf("Failed to start node1: %v", err)
	}
	defer node1.Stop()

	err = node2.Start()
	if err != nil {
		t.Fatalf("Failed to start node2: %v", err)
	}
	defer node2.Stop()

	err = node3.Start()
	if err != nil {
		t.Fatalf("Failed to start node3: %v", err)
	}
	defer node3.Stop()

	// Даем время на установление соединений
	time.Sleep(100 * time.Millisecond)

	// Генерируем ключ для шифрования
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	// Добавляем несколько сообщений в первый узел
	for i := 0; i < 3; i++ {
		err = bc1.AddMessage("Alice", "Bob", fmt.Sprintf("Message %d", i), key)
		if err != nil {
			t.Fatalf("Failed to add message %d to node1: %v", i, err)
		}
	}

	// Отправляем запрос на синхронизацию цепочки от узла 3 к узлу 1
	// Используем неэкспортированный метод requestChainFromPeers
	// Это требует изменения в коде или создания обертки для тестирования
	// Для тестов можно использовать другой подход, например, напрямую вызвать requestChain
	node3.BroadcastBlock(bc3.GetLastBlock()) // Вместо requestChainFromPeers используем другой подход

	// Даем время на синхронизацию
	time.Sleep(200 * time.Millisecond)

	// Проверяем, что все узлы имеют одинаковую длину цепочки
	if len(bc1.GetChain()) != len(bc3.GetChain()) {
		t.Errorf("Expected node3 to have the same chain length as node1: %d vs %d", len(bc1.GetChain()), len(bc3.GetChain()))
	}

	// Проверяем, что последний блок одинаковый у всех узлов
	lastBlock1 := bc1.GetLastBlock()
	lastBlock3 := bc3.GetLastBlock()

	if lastBlock1.Hash != lastBlock3.Hash {
		t.Errorf("Expected node3 to have the same last block hash as node1: %s vs %s", lastBlock1.Hash, lastBlock3.Hash)
	}

	// Проверяем, что сообщения можно прочитать из третьего узла
	messages := bc3.ReadMessages("Bob", key)
	if len(messages) != 3 {
		t.Errorf("Expected to read 3 messages from node3, got %d", len(messages))
	}
}

// TestEndToEndMessaging проверяет полный цикл отправки и получения сообщений
func TestEndToEndMessaging(t *testing.T) {
	// Настраиваем логирование
	logger, cleanup := SetupTestLogging(t)
	defer cleanup()

	logger.Log("Starting end-to-end messaging test")

	// Создаем блокчейн
	bc := blockchain.NewBlockchain()
	logger.Log("Created blockchain instance")

	// Генерируем ключ для шифрования
	key, err := crypto.GenerateKey()
	AssertNoError(t, err, "Failed to generate key")
	logger.Log("Generated encryption key")

	// Добавляем несколько сообщений
	messages := []struct {
		sender    string
		recipient string
		content   string
	}{
		{"Alice", "Bob", "Hello, Bob!"},
		{"Bob", "Alice", "Hi Alice, how are you?"},
		{"Alice", "Bob", "I'm fine, thanks!"},
		{"Charlie", "Alice", "Hello Alice, it's Charlie!"},
		{"Alice", "Charlie", "Hi Charlie!"},
	}

	logger.Log("Adding %d messages to blockchain", len(messages))
	for i, msg := range messages {
		err = bc.AddMessage(msg.sender, msg.recipient, msg.content, key)
		AssertNoError(t, err, fmt.Sprintf("Failed to add message %d", i))
		logger.Log("Added message %d: %s -> %s", i, msg.sender, msg.recipient)
	}

	// Проверяем, что блокчейн содержит правильное количество блоков
	expectedBlockCount := len(messages) + 1 // +1 для генезис-блока
	actualBlockCount := len(bc.GetChain())
	AssertEqual(t, expectedBlockCount, actualBlockCount, "Incorrect blockchain length")
	logger.Log("Blockchain contains %d blocks (including genesis block)", actualBlockCount)

	// Проверяем, что все сообщения можно прочитать
	bobMessages := bc.ReadMessages("Bob", key)
	AssertEqual(t, 2, len(bobMessages), "Incorrect number of messages for Bob")
	logger.Log("Bob has %d messages", len(bobMessages))

	aliceMessages := bc.ReadMessages("Alice", key)
	AssertEqual(t, 2, len(aliceMessages), "Incorrect number of messages for Alice")
	logger.Log("Alice has %d messages", len(aliceMessages))

	charlieMessages := bc.ReadMessages("Charlie", key)
	AssertEqual(t, 1, len(charlieMessages), "Incorrect number of messages for Charlie")
	logger.Log("Charlie has %d messages", len(charlieMessages))

	// Проверяем содержимое сообщений
	logger.Log("Verifying message contents")
	for i, msg := range bobMessages {
		if !contains(msg, "Hello, Bob!") && !contains(msg, "I'm fine, thanks!") {
			t.Errorf("Bob's message %d doesn't contain expected content: %s", i, msg)
			logger.Log("ERROR: Bob's message %d has unexpected content: %s", i, msg)
		} else {
			logger.Log("Bob's message %d has expected content", i)
		}
	}

	for i, msg := range aliceMessages {
		if !contains(msg, "Hi Alice, how are you?") && !contains(msg, "Hello Alice, it's Charlie!") {
			t.Errorf("Alice's message %d doesn't contain expected content: %s", i, msg)
			logger.Log("ERROR: Alice's message %d has unexpected content: %s", i, msg)
		} else {
			logger.Log("Alice's message %d has expected content", i)
		}
	}

	for i, msg := range charlieMessages {
		if !contains(msg, "Hi Charlie!") {
			t.Errorf("Charlie's message %d doesn't contain expected content: %s", i, msg)
			logger.Log("ERROR: Charlie's message %d has unexpected content: %s", i, msg)
		} else {
			logger.Log("Charlie's message %d has expected content", i)
		}
	}

	// Проверяем, что сообщения зашифрованы в блокчейне
	logger.Log("Verifying message encryption")
	for i, block := range bc.GetChain() {
		for j, msg := range block.Messages {
			if msg.Content == "" {
				logger.Log("Block %d, message %d: empty content (likely genesis block)", i, j)
				continue // Пропускаем пустые сообщения (например, в генезис-блоке)
			}

			// Проверяем, что содержимое сообщения - это шестнадцатеричная строка
			_, err := hex.DecodeString(msg.Content)
			if err != nil {
				t.Errorf("Message content is not a valid hex string: %v", err)
				logger.Log("ERROR: Block %d, message %d: content is not a valid hex string", i, j)
			} else {
				logger.Log("Block %d, message %d: content is a valid hex string", i, j)
			}

			// Проверяем, что содержимое сообщения не совпадает с исходным текстом
			for k, origMsg := range messages {
				if msg.Sender == origMsg.sender && msg.Recipient == origMsg.recipient && msg.Content == origMsg.content {
					t.Errorf("Message content is not encrypted: %s", msg.Content)
					logger.Log("ERROR: Block %d, message %d (original message %d): content is not encrypted", i, j, k)
				}
			}
		}
	}

	logger.Log("End-to-end messaging test completed successfully")
}

// Вспомогательная функция для проверки, содержит ли строка подстроку
func contains(s, substr string) bool {
	return s != "" && s != substr && len(s) >= len(substr) && s[len(s)-len(substr):] == substr
}
