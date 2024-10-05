start_rabbitmq:
	docker compose up --build -d rabbitmq 

stop_docker:
	docker compose down