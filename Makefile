.PHONY: run dev build test frontend-build

run:
	wails build
	open build/bin/ccusage-ui.app

dev:
	wails dev -noreload

build:
	wails build

test:
	go test ./...

frontend-build:
	cd frontend && bun run build
