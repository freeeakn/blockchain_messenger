.PHONY: build run clean test test-unit test-integration test-coverage test-with-script

build:
	go build -o bin/aetherwave cmd/aetherwave/main.go

run:
	go run cmd/aetherwave/main.go

clean:
	rm -rf bin/ coverage/ test-logs/

test:
	mkdir -p test-logs
	go test ./... -v 2>&1 | tee test-logs/all_tests.log

test-unit:
	mkdir -p test-logs
	go test ./pkg/... -v 2>&1 | tee test-logs/unit_tests.log

test-integration:
	mkdir -p test-logs
	go test ./tests/... -v 2>&1 | tee test-logs/integration_tests.log

test-coverage:
	mkdir -p coverage test-logs
	go test ./... -coverprofile=coverage/coverage.out -covermode=atomic -v 2>&1 | tee test-logs/coverage_tests.log
	go tool cover -html=coverage/coverage.out -o coverage/coverage.html
	@echo "Coverage report generated at coverage/coverage.html"

test-coverage-unit:
	mkdir -p coverage test-logs
	go test ./pkg/... -coverprofile=coverage/unit_coverage.out -covermode=atomic -v 2>&1 | tee test-logs/unit_coverage_tests.log
	go tool cover -html=coverage/unit_coverage.out -o coverage/unit_coverage.html
	@echo "Unit test coverage report generated at coverage/unit_coverage.html"

test-coverage-integration:
	mkdir -p coverage test-logs
	go test ./tests/... -coverprofile=coverage/integration_coverage.out -covermode=atomic -v 2>&1 | tee test-logs/integration_coverage_tests.log
	go tool cover -html=coverage/integration_coverage.out -o coverage/integration_coverage.html
	@echo "Integration test coverage report generated at coverage/integration_coverage.html"

test-with-script:
	./scripts/run_tests.sh

.DEFAULT_GOAL := build