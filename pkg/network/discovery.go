package network

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
)

const (
	// ServiceType определяет тип сервиса для mDNS
	ServiceType = "_aetherwave._tcp"
	// ServiceDomain определяет домен для mDNS
	ServiceDomain = "local."
	// DiscoveryTimeout определяет таймаут для обнаружения сервисов
	DiscoveryTimeout = 5 * time.Second
	// ServiceTTL определяет время жизни записи сервиса
	ServiceTTL = 120
)

// MDNSDiscovery представляет механизм обнаружения узлов с использованием mDNS
type MDNSDiscovery struct {
	node          *Node
	server        *zeroconf.Server
	resolver      *zeroconf.Resolver
	mutex         sync.RWMutex
	discoveredMap map[string]bool
	shutdown      chan struct{}
}

// NewMDNSDiscovery создает новый экземпляр MDNSDiscovery
func NewMDNSDiscovery(node *Node) *MDNSDiscovery {
	return &MDNSDiscovery{
		node:          node,
		discoveredMap: make(map[string]bool),
		shutdown:      make(chan struct{}),
	}
}

// Start запускает сервис mDNS для обнаружения и регистрации
func (md *MDNSDiscovery) Start() error {
	// Извлекаем порт из адреса узла
	_, portStr, err := net.SplitHostPort(md.node.Address)
	if err != nil {
		return fmt.Errorf("неверный формат адреса узла: %v", err)
	}

	// Получаем имя хоста
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "aetherwave-node"
	}

	// Регистрируем сервис
	port, _ := strconv.Atoi(portStr)
	server, err := zeroconf.Register(
		hostname,                // Имя сервиса
		ServiceType,             // Тип сервиса
		ServiceDomain,           // Домен
		port,                    // Порт
		[]string{"version=1.0"}, // Метаданные
		nil,                     // Интерфейсы (nil = все)
	)
	if err != nil {
		return fmt.Errorf("ошибка регистрации mDNS сервиса: %v", err)
	}
	md.server = server

	// Создаем резолвер для обнаружения других узлов
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		md.server.Shutdown()
		return fmt.Errorf("ошибка создания mDNS резолвера: %v", err)
	}
	md.resolver = resolver

	// Запускаем периодическое обнаружение узлов
	go md.discoverLoop()

	fmt.Printf("mDNS сервис запущен для узла %s\n", md.node.Address)
	return nil
}

// Stop останавливает сервис mDNS
func (md *MDNSDiscovery) Stop() {
	close(md.shutdown)
	if md.server != nil {
		md.server.Shutdown()
	}
	fmt.Printf("mDNS сервис остановлен для узла %s\n", md.node.Address)
}

// discoverLoop периодически запускает обнаружение узлов
func (md *MDNSDiscovery) discoverLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Запускаем первое обнаружение сразу
	md.discoverNodes()

	for {
		select {
		case <-md.shutdown:
			return
		case <-ticker.C:
			md.discoverNodes()
		}
	}
}

// discoverNodes запускает процесс обнаружения узлов
func (md *MDNSDiscovery) discoverNodes() {
	entries := make(chan *zeroconf.ServiceEntry)

	// Обрабатываем найденные сервисы в отдельной горутине
	go func(results <-chan *zeroconf.ServiceEntry) {
		for entry := range results {
			md.handleDiscoveredNode(entry)
		}
	}(entries)

	// Создаем контекст с таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), DiscoveryTimeout)
	defer cancel()

	// Запускаем поиск сервисов
	err := md.resolver.Browse(ctx, ServiceType, ServiceDomain, entries)
	if err != nil {
		fmt.Printf("Ошибка при поиске узлов: %v\n", err)
		return
	}

	// Ждем завершения таймаута
	<-ctx.Done()
}

// handleDiscoveredNode обрабатывает обнаруженный узел
func (md *MDNSDiscovery) handleDiscoveredNode(entry *zeroconf.ServiceEntry) {
	// Формируем адрес узла
	var ipAddr string
	if len(entry.AddrIPv4) > 0 {
		ipAddr = entry.AddrIPv4[0].String()
	} else if len(entry.AddrIPv6) > 0 {
		ipAddr = entry.AddrIPv6[0].String()
	} else {
		fmt.Printf("Обнаружен узел без IP адреса: %s\n", entry.Instance)
		return
	}

	nodeAddr := fmt.Sprintf("%s:%d", ipAddr, entry.Port)

	// Проверяем, не является ли это текущим узлом
	if nodeAddr == md.node.Address {
		return
	}

	// Проверяем, не обнаружили ли мы уже этот узел
	md.mutex.Lock()
	if _, exists := md.discoveredMap[nodeAddr]; exists {
		md.mutex.Unlock()
		return
	}
	md.discoveredMap[nodeAddr] = true
	md.mutex.Unlock()

	fmt.Printf("Обнаружен новый узел: %s\n", nodeAddr)

	// Подключаемся к обнаруженному узлу
	go md.node.ConnectToPeer(nodeAddr)
}

// GetDiscoveredNodes возвращает список обнаруженных узлов
func (md *MDNSDiscovery) GetDiscoveredNodes() []string {
	md.mutex.RLock()
	defer md.mutex.RUnlock()

	nodes := make([]string, 0, len(md.discoveredMap))
	for node := range md.discoveredMap {
		nodes = append(nodes, node)
	}
	return nodes
}
