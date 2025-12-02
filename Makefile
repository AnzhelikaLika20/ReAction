.PHONY: run stop logs clean

run:
	docker-compose run --rm reaction

build:
	docker-compose build

stop:
	docker-compose down

logs:
	docker-compose logs -f

clean:
	docker-compose down -v
	docker system prune -f