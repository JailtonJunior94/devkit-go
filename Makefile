dotenv:
	@echo "Creating .env file..."
	cp .env.example .env

lint:
	@echo "Running linter..."
	@echo "Installing golangci-lint..."
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.2.1
	GOGC=20 golangci-lint run --config .golangci.yml ./...

test:
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...

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