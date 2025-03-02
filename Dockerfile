FROM golang:1.21-alpine AS builder

WORKDIR /app

# Копируем только файлы зависимостей для кеширования слоев
COPY go.mod ./
COPY go.sum ./
RUN go mod download

# Копируем исходный код
COPY . .

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/aetherwave cmd/aetherwave/main.go

# Финальный образ
FROM alpine:latest

WORKDIR /app

# Копируем бинарный файл из предыдущего этапа
COPY --from=builder /app/bin/aetherwave .

# Создаем директории для данных
RUN mkdir -p /app/data /app/logs

# Открываем порт
EXPOSE 3000

# Запускаем приложение
ENTRYPOINT ["/app/aetherwave"]
CMD ["--address", ":3000"] 