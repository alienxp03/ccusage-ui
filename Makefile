.PHONY: setup run dev build test frontend-build destroy-db

TOOLS := go@1.26.4 node@24 bun@1.3.14
MISE := mise --no-config exec $(TOOLS) --
WAILS := $(MISE) go run github.com/wailsapp/wails/v2/cmd/wails

setup:
	mise --no-config install --quiet $(TOOLS)
	$(MISE) go mod download
	$(MISE) bun install --cwd frontend
	$(WAILS) version

run:
	$(WAILS) build
	open build/bin/ccusage-ui.app

dev:
	$(WAILS) dev

build:
	$(WAILS) build

test:
	$(MISE) go test ./...

frontend-build:
	$(MISE) bun run --cwd frontend build

destroy-db:
	rm -f "$(HOME)/Library/Caches/ccusage-ui/usage-index.sqlite"*
