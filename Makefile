.PHONY: build run test clean docker-build docker-run test-integration test-unit gitlab-start gitlab-stop test-env-start test-env-stop test-env-restart test-env-status test-env-logs

APP_NAME=gitlab-mr-conform
VERSION?=latest

build:
	go build -o bin/$(APP_NAME) ./cmd/bot

run:
	go run ./cmd/bot

test:
	go test -v ./...

test-unit:
	@echo "Running unit tests..."
	go test -v -short ./internal/... ./pkg/...

test-integration:
	@echo "Running integration tests..."
	@if [ ! -f test/docker/gitlab_url.txt ] || [ ! -f test/docker/gitlab_token.txt ]; then \
		echo "Error: GitLab test instance not running. Please run 'make gitlab-start' first."; \
		exit 1; \
	fi
	go test -v -timeout 30m ./test/integration/...

gitlab-start:
	@chmod +x test/docker/test_env.sh
	@./test/docker/test_env.sh start

gitlab-stop:
	@chmod +x test/docker/test_env.sh
	@./test/docker/test_env.sh stop

gitlab-clean: gitlab-stop
	@echo "Cleaning up GitLab test data..."
	@rm -f test/docker/gitlab_url.txt
	@rm -f test/docker/gitlab_token.txt

test-env-start:
	@chmod +x test/docker/test_env.sh
	@./test/docker/test_env.sh start

test-env-restart:
	@chmod +x test/docker/test_env.sh
	@./test/docker/test_env.sh restart

test-env-status:
	@chmod +x test/docker/test_env.sh
	@./test/docker/test_env.sh status

test-env-logs:
	@chmod +x test/docker/test_env.sh
	@./test/docker/test_env.sh logs

test-env-stop:
	@chmod +x test/docker/test_env.sh
	@./test/docker/test_env.sh stop

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