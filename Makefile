PLUGINS := otel/grafana auth/supabase swagger/ui storage/s3
MODULES := $(PLUGINS) examples/upload

export GOWORK := off

.PHONY: all tidy test lint build e2e e2e-upload hooks

all: tidy test lint

tidy:
	@set -e; for p in $(MODULES); do \
		echo "==> tidy $$p"; \
		(cd $$p && go mod tidy); \
	done

test:
	@set -e; for p in $(MODULES); do \
		echo "==> test $$p"; \
		(cd $$p && go test ./...); \
	done

lint:
	@set -e; for p in $(PLUGINS); do \
		echo "==> lint $$p"; \
		(cd $$p && golangci-lint run ./...); \
	done

build:
	@set -e; for p in $(MODULES); do \
		echo "==> build $$p"; \
		(cd $$p && go build ./...); \
	done

# hooks: install the pre-commit hook from .github/pre-commit into .git/hooks/.
hooks:
	cp .github/pre-commit .git/hooks/pre-commit
	chmod +x .git/hooks/pre-commit
	@echo "pre-commit hook installed"

# e2e: spin up docker services, run all E2E tests, tear down regardless of outcome.
# Prerequisites: docker with compose plugin.
e2e:
	docker compose -f e2e/docker-compose.yml up -d --wait
	cd e2e && go test -v -tags e2e -count=1 ./... ; \
	  STATUS=$$?; \
	  cd .. && docker compose -f e2e/docker-compose.yml down; \
	  exit $$STATUS

e2e-upload:
	sh examples/upload/scripts/with-compose.sh
