PLUGINS := otel/grafana auth/supabase swagger/ui

.PHONY: tidy test lint all

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
