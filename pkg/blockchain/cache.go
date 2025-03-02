package blockchain

import (
	"fmt"
	"sync"
	"time"
)

// CacheEntry представляет запись в кэше
type CacheEntry struct {
	Value      interface{}
	Expiration int64
}

// Cache представляет простой кэш с ограниченным временем жизни записей
type Cache struct {
	entries  map[string]CacheEntry
	mutex    sync.RWMutex
	janitor  *time.Ticker
	stopChan chan struct{}
}

// NewCache создает новый кэш с указанным интервалом очистки
func NewCache(cleanupInterval time.Duration) *Cache {
	cache := &Cache{
		entries:  make(map[string]CacheEntry),
		janitor:  time.NewTicker(cleanupInterval),
		stopChan: make(chan struct{}),
	}

	// Запускаем горутину для очистки устаревших записей
	go func() {
		for {
			select {
			case <-cache.janitor.C:
				cache.cleanup()
			case <-cache.stopChan:
				cache.janitor.Stop()
				return
			}
		}
	}()

	return cache
}

// Set добавляет значение в кэш с указанным временем жизни
func (c *Cache) Set(key string, value interface{}, ttl time.Duration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	var expiration int64
	if ttl > 0 {
		expiration = time.Now().Add(ttl).UnixNano()
	}

	c.entries[key] = CacheEntry{
		Value:      value,
		Expiration: expiration,
	}
}

// Get получает значение из кэша
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	entry, found := c.entries[key]
	if !found {
		return nil, false
	}

	// Проверяем, не истекло ли время жизни записи
	if entry.Expiration > 0 && time.Now().UnixNano() > entry.Expiration {
		return nil, false
	}

	return entry.Value, true
}

// Delete удаляет запись из кэша
func (c *Cache) Delete(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.entries, key)
}

// Size возвращает количество записей в кэше
func (c *Cache) Size() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return len(c.entries)
}

// cleanup удаляет устаревшие записи из кэша
func (c *Cache) cleanup() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	now := time.Now().UnixNano()
	for key, entry := range c.entries {
		if entry.Expiration > 0 && now > entry.Expiration {
			delete(c.entries, key)
		}
	}
}

// Close останавливает очистку кэша
func (c *Cache) Close() {
	close(c.stopChan)
}

// BlockchainCache содержит кэши для различных операций блокчейна
type BlockchainCache struct {
	messageCache    *Cache
	blockCache      *Cache
	validationCache *Cache
}

// NewBlockchainCache создает новый кэш для блокчейна
func NewBlockchainCache() *BlockchainCache {
	return &BlockchainCache{
		messageCache:    NewCache(5 * time.Minute),
		blockCache:      NewCache(10 * time.Minute),
		validationCache: NewCache(30 * time.Minute),
	}
}

// GetMessages получает сообщения из кэша или добавляет их, если их нет
func (bc *BlockchainCache) GetMessages(recipient string, decryptedMessages []string, key []byte) []string {
	cacheKey := recipient + string(key)

	// Пробуем получить из кэша
	if cachedMessages, found := bc.messageCache.Get(cacheKey); found {
		return cachedMessages.([]string)
	}

	// Кэшируем результат
	bc.messageCache.Set(cacheKey, decryptedMessages, 5*time.Minute)
	return decryptedMessages
}

// GetBlock получает блок из кэша или добавляет его, если его нет
func (bc *BlockchainCache) GetBlock(index int, block Block) Block {
	cacheKey := string(rune(index))

	// Пробуем получить из кэша
	if cachedBlock, found := bc.blockCache.Get(cacheKey); found {
		return cachedBlock.(Block)
	}

	// Кэшируем результат
	bc.blockCache.Set(cacheKey, block, 10*time.Minute)
	return block
}

// IsValidChain проверяет, является ли цепочка валидной, используя кэш
func (bc *BlockchainCache) IsValidChain(chain []Block, isValid bool) bool {
	cacheKey := fmt.Sprintf("chain_length_%d", len(chain)) // Используем fmt.Sprintf для корректного преобразования

	// Пробуем получить из кэша
	if cachedResult, found := bc.validationCache.Get(cacheKey); found {
		return cachedResult.(bool)
	}

	// Кэшируем результат
	bc.validationCache.Set(cacheKey, isValid, 30*time.Minute)
	return isValid
}

// ClearCaches очищает все кэши
func (bc *BlockchainCache) ClearCaches() {
	// Создаем новые кэши
	bc.messageCache = NewCache(5 * time.Minute)
	bc.blockCache = NewCache(10 * time.Minute)
	bc.validationCache = NewCache(30 * time.Minute)
}

// Close закрывает все кэши
func (bc *BlockchainCache) Close() {
	bc.messageCache.Close()
	bc.blockCache.Close()
	bc.validationCache.Close()
}
