.PHONY: build run test clean docker-build docker-run test-integration test-unit gitlab-start gitlab-stop

APP_NAME=gitlab-mr-conform
VERSION?=latest

build:
	go build -o bin/$(APP_NAME) ./cmd/bot

run:
	go run ./cmd/bot

test:
	go test -v ./...

test-unit:
	go test -v -short ./...

test-integration:
	@echo "Running integration tests..."
	@if [ ! -f test/docker/gitlab_url.txt ] || [ ! -f test/docker/gitlab_token.txt ]; then \
		echo "Error: GitLab test instance not running. Please run 'make gitlab-start' first."; \
		exit 1; \
	fi
	go test -v -timeout 30m ./test/integration/...

gitlab-start:
	@echo "Starting GitLab test instance (this may take 5-10 minutes)..."
	@chmod +x test/docker/run_gitlab.sh
	@./test/docker/run_gitlab.sh

gitlab-stop:
	@echo "Stopping GitLab test instance..."
	@chmod +x test/docker/stop_gitlab.sh
	@./test/docker/stop_gitlab.sh

gitlab-clean: gitlab-stop
	@echo "Cleaning up GitLab test data..."
	@rm -f test/docker/gitlab_url.txt
	@rm -f test/docker/gitlab_token.txt

clean:
	rm -rf bin/

docker-build:
	docker build -t $(APP_NAME):$(VERSION) .

docker-run:
	docker run -p 8080:8080 \
		-e GITLAB_MR_BOT_GITLAB_TOKEN=$(GITLAB_MR_BOT_GITLAB_TOKEN) \
		-e GITLAB_MR_BOT_GITLAB_SECRET_TOKEN=$(GITLAB_MR_BOT_GITLAB_SECRET_TOKEN) \
		$(APP_NAME):$(VERSION)

dev-setup:
	go mod tidy
	go mod download