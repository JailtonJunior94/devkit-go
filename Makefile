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
	docker compose up --build -d --remove-orphans rabbitmq 

start_kafka:
	docker compose up --build -d --remove-orphans zookeeper broker kafka_ui 

stop_docker:
	docker compose down