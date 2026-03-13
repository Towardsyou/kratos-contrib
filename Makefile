PLUGINS := otel/grafana auth/supabase swagger/ui

.PHONY: tidy test lint all e2e

all: tidy test lint

tidy:
	@for p in $(PLUGINS); do \
		echo "==> tidy $$p"; \
		(cd $$p && go mod tidy); \
	done

test:
	@for p in $(PLUGINS); do \
		echo "==> test $$p"; \
		(cd $$p && go test ./...); \
	done

lint:
	@for p in $(PLUGINS); do \
		echo "==> lint $$p"; \
		(cd $$p && golangci-lint run ./...); \
	done

build:
	@for p in $(PLUGINS); do \
		echo "==> build $$p"; \
		(cd $$p && go build ./...); \
	done

# e2e: spin up docker services, run all E2E tests, tear down regardless of outcome.
# Prerequisites: docker with compose plugin.
e2e:
	docker compose -f e2e/docker-compose.yml up -d --wait
	cd e2e && go test -v -tags e2e -count=1 ./... ; \
	  STATUS=$$?; \
	  cd .. && docker compose -f e2e/docker-compose.yml down; \
	  exit $$STATUS
