include .env

.PHONY: build build-ko push run test clean up down deploy

build:
	docker compose build

build-ko:
	ko build --local ./cmd/bot

push:
	ko build ./cmd/bot

run:
	@if [ -f .env ]; then set -a && . ./.env && set +a; fi && go run ./cmd/bot

test:
	go test ./...

clean:
	rm -f admin.db

up: down
	docker compose up --build -d

down:
	docker compose down

deploy:
	ssh ${DEPLOY_HOST} "cd ~/go-telegram-admin/; git pull; go build -o bot ./cmd/bot/main.go; sudo systemctl restart telegram-bot"

