package blockchain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Константы для настройки майнинга
const (
	// MaxNonce - максимальное значение nonce для перебора
	MaxNonce = math.MaxInt64

	// MaxMiningDuration - максимальное время майнинга блока
	MaxMiningDuration = 60 * time.Second

	// MiningDifficultyAdjustmentInterval - интервал блоков для корректировки сложности
	MiningDifficultyAdjustmentInterval = 10

	// TargetBlockTime - целевое время создания блока в секундах
	TargetBlockTime = 30 * time.Second

	// MiningStatusInterval - интервал для вывода статуса майнинга
	MiningStatusInterval = 5 * time.Second

	// TargetBlockCreationTime - целевое время создания блока в секундах
	TargetBlockCreationTime = 30
)

// ParallelMineBlock выполняет майнинг блока с использованием Proof of Work
func (bc *Blockchain) ParallelMineBlock(block *Block) bool {
	startTime := time.Now()

	// Подготавливаем префикс блока (все данные, кроме nonce и hash)
	prefix := prepareBlockPrefix(block)

	// Определяем количество горутин для майнинга
	numCPU := runtime.NumCPU()
	numWorkers := numCPU

	// Используем каналы для координации горутин
	resultChan := make(chan int, numWorkers)
	stopChan := make(chan struct{})
	var wg sync.WaitGroup

	// Создаем счетчик для хешей в секунду
	var hashCounter int64

	// Запускаем горутину для статистики
	go func() {
		ticker := time.NewTicker(MiningStatusInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				duration := time.Since(startTime).Seconds()
				if duration > 0 {
					hashRate := float64(atomic.LoadInt64(&hashCounter)) / duration
					fmt.Printf("Mining... Hash rate: %.2f h/s, Time elapsed: %.2fs\n",
						hashRate, duration)
				}
			case <-stopChan:
				return
			}
		}
	}()

	// Разделяем пространство nonce между воркерами
	noncePerWorker := MaxNonce / numWorkers

	// Запускаем воркеры для майнинга
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		startNonce := i * noncePerWorker
		endNonce := (i + 1) * noncePerWorker

		go func(start, end int) {
			defer wg.Done()

			// Создаем локальный хешер для каждой горутины
			hasher := sha256.New()

			for nonce := start; nonce < end; nonce++ {
				select {
				case <-stopChan:
					return
				default:
					// Инкрементируем счетчик хешей
					atomic.AddInt64(&hashCounter, 1)

					// Пробуем nonce
					hasher.Reset()
					hasher.Write([]byte(fmt.Sprintf("%s%d", prefix, nonce)))
					hash := hex.EncodeToString(hasher.Sum(nil))

					// Проверяем, удовлетворяет ли хеш требуемой сложности
					if isValidHash(hash, bc.Difficulty) {
						resultChan <- nonce
						return
					}

					// Прерываем майнинг, если превышено максимальное время
					if time.Since(startTime) > MaxMiningDuration {
						return
					}
				}
			}
		}(startNonce, endNonce)
	}

	// Запускаем горутину для ожидания завершения
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Ожидаем результат или таймаут
	select {
	case nonce, ok := <-resultChan:
		if ok {
			// Найдено решение
			block.Nonce = nonce
			block.Hash = MineBlockHash(*block)

			// Закрываем все остальные горутины
			close(stopChan)

			duration := time.Since(startTime)
			fmt.Printf("Block mined! Nonce: %d, Hash: %s, Time: %v\n",
				nonce, block.Hash, duration)

			// Корректируем сложность каждые N блоков
			if bc.shouldAdjustDifficulty() {
				bc.adjustDifficulty(duration)
			}

			return true
		}
	case <-time.After(MaxMiningDuration):
		// Превышено максимальное время майнинга
		close(stopChan)
		fmt.Println("Mining timed out after", MaxMiningDuration)
		return false
	}

	return false
}

// isValidHash проверяет, удовлетворяет ли хеш требуемой сложности
func isValidHash(hash string, difficulty int) bool {
	prefix := strings.Repeat("0", difficulty)
	return strings.HasPrefix(hash, prefix)
}

// prepareBlockPrefix подготавливает строковое представление блока без nonce и hash
func prepareBlockPrefix(block *Block) string {
	var data string
	for _, msg := range block.Messages {
		data += fmt.Sprintf("%s%s%s%d", msg.Sender, msg.Recipient, msg.Content, msg.Timestamp)
	}
	return fmt.Sprintf("%d%d%s%s", block.Index, block.Timestamp, block.PrevHash, data)
}

// shouldAdjustDifficulty определяет, нужно ли корректировать сложность
func (bc *Blockchain) shouldAdjustDifficulty() bool {
	return len(bc.Chain) > 0 && len(bc.Chain)%MiningDifficultyAdjustmentInterval == 0
}

// adjustDifficulty корректирует сложность майнинга на основе времени создания блоков
func (bc *Blockchain) adjustDifficulty(lastBlockTime time.Duration) {
	bc.mutex.Lock()
	defer bc.mutex.Unlock()

	// Если блоков меньше интервала корректировки, пропускаем
	if len(bc.Chain) < MiningDifficultyAdjustmentInterval {
		return
	}

	// Рассчитываем среднее время создания блока
	startIndex := len(bc.Chain) - MiningDifficultyAdjustmentInterval
	startBlock := bc.Chain[startIndex]
	endBlock := bc.Chain[len(bc.Chain)-1]

	// Считаем время в секундах
	timeSpent := float64(endBlock.Timestamp-startBlock.Timestamp) / float64(MiningDifficultyAdjustmentInterval)
	targetTime := TargetBlockTime.Seconds()

	// Корректируем сложность
	if timeSpent < targetTime/2 {
		// Блоки создаются слишком быстро, увеличиваем сложность
		bc.Difficulty++
		fmt.Printf("Difficulty increased to %d (blocks too fast: %.2fs vs %.2fs target)\n",
			bc.Difficulty, timeSpent, targetTime)
	} else if timeSpent > targetTime*2 {
		// Блоки создаются слишком медленно, уменьшаем сложность
		if bc.Difficulty > 1 {
			bc.Difficulty--
			fmt.Printf("Difficulty decreased to %d (blocks too slow: %.2fs vs %.2fs target)\n",
				bc.Difficulty, timeSpent, targetTime)
		}
	}
}

// MineBlockHash вычисляет хеш блока для майнинга
func MineBlockHash(block Block) string {
	// Сериализуем все данные блока
	data := fmt.Sprintf("%d%d%s%d%s",
		block.Index,
		block.Timestamp,
		block.PrevHash,
		block.Nonce,
		serializeMessages(block.Messages))

	// Вычисляем SHA-256 хеш
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// serializeMessages сериализует сообщения блока в строку
func serializeMessages(messages []Message) string {
	var result string
	for _, msg := range messages {
		result += fmt.Sprintf("%s%s%s%d", msg.Sender, msg.Recipient, msg.Content, msg.Timestamp)
	}
	return result
}
