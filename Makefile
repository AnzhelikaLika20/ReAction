.PHONY: run stop logs clean

run:
	docker compose up -d --build

build:
	docker compose build

stop:
	docker compose down

logs:
	docker logs reaction

clean:
	docker compose down -v
	docker system prune -f

swag:
	swag init -g ./cmd/main.go -o ./docs --quiet

format:
	gofmt -w .

up: format swag run