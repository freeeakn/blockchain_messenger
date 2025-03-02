#!/bin/bash

# Создаем директорию для PID файлов, если она не существует
mkdir -p .pid

# Останавливаем предыдущие экземпляры, если они запущены
./scripts/stop_network.sh

echo "Запуск сети AetherWave с автоматическим обнаружением узлов..."

# Генерируем ключ шифрования
KEY=$(go run cmd/aetherwave/main.go -address :3000 -name KeyGen | grep "Generated key" | cut -d' ' -f3)
if [ -z "$KEY" ]; then
    echo "Ошибка генерации ключа шифрования"
    exit 1
fi
echo "Сгенерирован ключ шифрования: $KEY"

# Запускаем первый узел (лидер)
go run cmd/aetherwave/main.go -address :3000 -name Node1 -key $KEY -discovery > logs/node1.log 2>&1 &
PID1=$!
echo $PID1 > .pid/node1.pid
echo "Узел 1 запущен с PID $PID1"
sleep 2

# Запускаем второй узел
go run cmd/aetherwave/main.go -address :3001 -name Node2 -key $KEY -discovery > logs/node2.log 2>&1 &
PID2=$!
echo $PID2 > .pid/node2.pid
echo "Узел 2 запущен с PID $PID2"
sleep 1

# Запускаем третий узел
go run cmd/aetherwave/main.go -address :3002 -name Node3 -key $KEY -discovery > logs/node3.log 2>&1 &
PID3=$!
echo $PID3 > .pid/node3.pid
echo "Узел 3 запущен с PID $PID3"

echo "Сеть AetherWave запущена с автоматическим обнаружением узлов"
echo "Для остановки сети выполните: ./scripts/stop_network.sh" 