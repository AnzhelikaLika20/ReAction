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

# go install github.com/swaggo/swag/cmd/swag@latest
# echo 'export PATH=$PATH:$HOME/go/bin' >> ~/.bashrc
# source ~/.bashrc
# which swag
swag:
	swag init -g ./cmd/main.go -o ./docs --quiet

format:
	gofmt -w .

up: format swag run