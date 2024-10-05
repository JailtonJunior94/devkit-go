golint:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.61.0
	golangci-lint run ./...

start_rabbitmq:
	docker compose up --build -d rabbitmq 

stop_docker:
	docker compose down