.PHONY: build run test clean docker-build docker-run

APP_NAME=gitlab-mr-conform
VERSION?=latest

build:
	go build -o bin/$(APP_NAME) ./cmd/bot

run:
	go run ./cmd/bot

test:
	go test -v ./...

clean:
	rm -rf bin/

docker-build:
	docker build -t $(APP_NAME):$(VERSION) .

docker-run:
	docker run -p 8080:8080 \
		-e GITLAB_MR_BOT_GITLAB_TOKEN=$(GITLAB_BOT_GITLAB_TOKEN) \
		-e GITLAB_MR_BOT_GITLAB_SECRET_TOKEN=$(GITLAB_BOT_GITLAB_SECRET_TOKEN) \
		$(APP_NAME):$(VERSION)

dev-setup:
	go mod tidy
	go mod download