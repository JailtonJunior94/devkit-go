dotenv:
	@echo "Creating .env file..."
	cp .env.example .env

lint:
	@echo "Running linter..."
	@echo "Installing golangci-lint..."
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.2.1
	GOGC=20 golangci-lint run --config .golangci.yml ./...

.PHONY: mocks-clean mocks-generate mocks-reset mocks
mocks-clean:
	@echo "Removing generated mocks..."
	find ./pkg/observability ./pkg/database -type d -name mocks -prune -exec rm -rf {} +

mocks-generate:
	@echo "Generating mocks..."
	go install github.com/vektra/mockery/v3@v3.7.0
	mockery

mocks-reset: mocks-clean mocks-generate

mocks: mocks-generate

bench:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem -benchtime=3s ./pkg/database/...

bench-check:
	@echo "Running benchmark gate (absolute thresholds)..."
	go test -bench=. -benchmem -benchtime=3s ./pkg/database/... | tee /tmp/bench_out.txt
	@echo "Checking BenchmarkDBTX_PoolPath ns/op <= 500..."
	@awk '/BenchmarkDBTX_PoolPath[^a-zA-Z]/ {if ($$3+0 > 500) {print "FAIL: BenchmarkDBTX_PoolPath exceeded 500 ns/op: " $$3; exit 1}}' /tmp/bench_out.txt
	@echo "Checking BenchmarkDBTX_TxInCtxPath ns/op <= 200..."
	@awk '/BenchmarkDBTX_TxInCtxPath/ {if ($$3+0 > 200) {print "FAIL: BenchmarkDBTX_TxInCtxPath exceeded 200 ns/op: " $$3; exit 1}}' /tmp/bench_out.txt
	@echo "Checking BenchmarkUoW_Do_Commit ns/op <= 1500..."
	@awk '/BenchmarkUoW_Do_Commit[^a-zA-Z]/ {if ($$3+0 > 1500) {print "FAIL: BenchmarkUoW_Do_Commit exceeded 1500 ns/op: " $$3; exit 1}}' /tmp/bench_out.txt
	@echo "Checking BenchmarkUoW_Do_Rollback ns/op <= 1500..."
	@awk '/BenchmarkUoW_Do_Rollback[^a-zA-Z]/ {if ($$3+0 > 1500) {print "FAIL: BenchmarkUoW_Do_Rollback exceeded 1500 ns/op: " $$3; exit 1}}' /tmp/bench_out.txt
	@echo "Benchmark gate passed."

test:
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...

test-integration:
	@echo "Running integration tests..."
	go test -v -race -tags=integration -coverprofile=coverage_integration.txt -covermode=atomic ./...

cover:
	@echo "Generating coverage report..."
	go tool cover -html=coverage.out
	
vulncheck:
	@echo "Running vulnerability check..."
	go install golang.org/x/vuln/cmd/govulncheck@latest
	govulncheck ./...

start_rabbitmq:
	@echo "Starting RabbitMQ..."
	docker compose up --build -d --remove-orphans rabbitmq

start_kafka:
	@echo "Starting Kafka..."
	docker compose up --build -d --remove-orphans kafka kafka-init redpandadata

start_o11y:
	@echo "Starting O11y..."
	docker compose up --build -d --remove-orphans otel_collector prometheus jaeger grafana loki

stop_docker:
	@echo "Stopping Docker containers..."
	docker compose down
