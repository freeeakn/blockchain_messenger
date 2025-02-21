run:
	go run ./src

build:
	go build -o blockchapa

test:
	go test ./src -v -cover