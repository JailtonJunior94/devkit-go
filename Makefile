lint:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.61.0
	golangci-lint run ./...

test:
	go test -v -race -coverprofile coverage.out -failfast ./...
	go tool cover -html=coverage.out

bench:
    go test -bench=. ./...

start_rabbitmq:
	docker compose up --build -d rabbitmq 

stop_docker:
	docker compose down