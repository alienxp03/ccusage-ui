# ccusage-ui

A Wails desktop GUI for [`ccusage`](https://github.com/ccusage/ccusage).

The app shows ccusage reports, indexes sessions locally for project views, supports configurable project grouping, and can display formatted session conversations from local transcripts.

## Runner Detection

The Go backend detects a runner in this order:

1. `ccusage` on `PATH`
2. `bunx ccusage`
3. `nix run github:ccusage/ccusage --`
4. `npx ccusage@latest`
5. `pnpm dlx ccusage`

## Development

```bash
make dev
```

This starts Wails dev mode and Vite HMR for frontend changes.

## Build / Run

```bash
make run
```

or:

```bash
wails build
```
