# 🌌 AetherWave Blockchain Messenger Network

A secure and decentralized peer-to-peer messaging system built on blockchain technology. Send encrypted messages across a distributed network while maintaining transparency and integrity through blockchain verification.

## ✨ Features

This project demonstrates a blockchain-based messaging system with the following features:

- Distributed blockchain for message storage
- Encrypted messaging using AES-256
- Peer-to-peer network with gossip-based peer discovery
- Proof-of-work consensus mechanism

The system is split into four main components:

- `blockchain`: Core blockchain implementation
- `crypto`: Encryption/decryption utilities
- `network`: P2P networking and peer discovery
- `cmd/aetherwave`: Application entry point and CLI

## 🚀 Getting Started

### Prerequisites

- Go 1.21 or higher
- Basic understanding of blockchain concepts
- Basic knowledge of Go programming

## Installation

1. Clone the repository:

```bash
git clone https://github.com/freeeakn/AetherWave.git
cd AetherWave
```

## Запуск

```bash
make run
```

## Тестирование

AetherWave включает в себя обширный набор тестов, включая модульные и интеграционные тесты.

### Запуск всех тестов

```bash
make test
```

### Запуск только модульных тестов

```bash
make test-unit
```

### Запуск только интеграционных тестов

```bash
make test-integration
```

### Запуск тестов с покрытием кода

```bash
make test-coverage
```

Отчет о покрытии кода будет сгенерирован в директории `coverage/` в формате HTML.

### Запуск тестов с расширенным логированием

```bash
make test-with-script
```

Этот вариант запускает тесты с использованием специального скрипта, который:
- Генерирует подробные логи для каждого теста
- Создает отчеты о покрытии кода
- Записывает информацию об ошибках в отдельный файл
- Выводит статистику покрытия кода

Все логи и отчеты сохраняются в директориях `test-logs/` и `coverage/`.

## Структура проекта

- `cmd/` - исполняемые файлы
- `pkg/` - основные пакеты
  - `blockchain/` - реализация блокчейна
  - `crypto/` - криптографические функции
  - `network/` - сетевое взаимодействие
- `tests/` - интеграционные тесты
- `scripts/` - вспомогательные скрипты

## Лицензия

MIT