.PHONY: play server logs stop reset test

play:
	docker compose run --rm worldseed

server:
	docker compose up -d worldseedd

logs:
	docker compose logs -f worldseedd

stop:
	docker compose down

reset:
	docker compose down -v

test:
	go test ./...
