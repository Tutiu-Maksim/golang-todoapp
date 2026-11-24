-include .env
export

PROJECT_ROOT := $(CURDIR)
.DEFAULT_GOAL := env-up

.PHONY: env-up env-down logs-clear env-cleanup env-port-forward env-port-close \
	migrate-create migrate-up migrate-down migrate-action todoapp-run \
	todoapp-deploy todoapp-undeploy swagger-gen ps test test-cover

env-up:
	@docker compose up -d --wait todoapp-postgres

env-down:
	@docker compose --profile tools stop port-forwarder todoapp-postgres

logs-clear:
	@printf '%s' 'Очистить logs? Опасность утери данных. [y/N] '; \
	read -r ans; \
	if [ "$$ans" = "y" ]; then \
		rm -rf "$(PROJECT_ROOT)/out/logs" && echo "logs очищен"; \
	else \
		echo "Очистка logs отменена"; \
	fi

env-cleanup:
	@printf '%s' 'Удалить данные PostgreSQL? Опасность утери данных. [y/N] '; \
	read -r ans; \
	if [ "$$ans" = "y" ]; then \
		docker compose --profile tools down && \
		rm -rf "$(PROJECT_ROOT)/out/pgdata" && echo "Данные PostgreSQL удалены"; \
	else \
		echo "Очистка окружения отменена"; \
	fi

env-port-forward:
	@docker compose up -d port-forwarder

env-port-close:
	@docker compose --profile tools stop port-forwarder

migrate-create:
	@if [ -z "$(seq)" ]; then \
		echo "Отсутствует необходимый параметр seq. Пример: make migrate-create seq=init"; \
		exit 1; \
	fi
	@docker compose run --rm --no-deps todoapp-postgres-migrate \
		create -ext sql -dir /migrations -seq "$(seq)"

migrate-up:
	@docker compose run --rm todoapp-postgres-migrate

migrate-down:
	@$(MAKE) migrate-action action=down

migrate-action:
	@if [ -z "$(action)" ]; then \
		echo "Отсутствует необходимый параметр action. Пример: make migrate-action action=up"; \
		exit 1; \
	fi
	@docker compose run --rm todoapp-postgres-migrate \
		-path /migrations \
		-database "postgres://$$POSTGRES_USER:$$POSTGRES_PASSWORD@todoapp-postgres:5432/$$POSTGRES_DB?sslmode=disable&search_path=public" \
		"$(action)"

todoapp-run:
	@LOGGER_FOLDER="$(PROJECT_ROOT)/out/logs" POSTGRES_HOST=127.0.0.1 \
		go run ./cmd/todoapp

todoapp-deploy:
	@docker compose up -d --build todoapp

todoapp-undeploy:
	@docker compose stop todoapp

swagger-gen:
	@docker compose run --rm --build swagger \
		init -g cmd/todoapp/main.go -o docs --parseInternal --parseDependency

ps:
	@docker compose --profile tools ps

test:
	@go test ./...

test-cover:
	@mkdir -p out
	@go test -count=1 -coverpkg=./internal/... -coverprofile=out/coverage.out ./...
	@go tool cover -func=out/coverage.out
	@go tool cover -html=out/coverage.out -o out/coverage.html
