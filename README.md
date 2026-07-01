# ccusage-ui

A Wails desktop GUI for [`ccusage`](https://github.com/ccusage/ccusage).

The app shows ccusage reports, indexes sessions locally for project views, supports configurable project grouping, and can display formatted session conversations from local transcripts.

## Screenshots

### Projects

![Projects view](docs/images/projects.png)

### Daily report

![Daily report](docs/images/daily.png)

## Requirements

- Mise for project tasks and tool version management. Managed by Mise:
  - Go + Wails CLI
  - Node
  - Bun
- A [ccusage](https://github.com/ccusage/ccusage) runner. The app detects one in this order:
  1. `ccusage` on `PATH`
  2. `bunx ccusage`
  3. `nix run github:ccusage/ccusage --`
  4. `npx ccusage@latest`
  5. `pnpm dlx ccusage`

Recommended local setup:

```bash
mise setup
```

Wails is run from the project's Go module with `go run`; it does not need to be installed globally.
The app can run `ccusage` from `PATH`, or fall back to `bunx`, `nix`, `npx`, or `pnpm` when available.

## Development

```bash
mise setup
mise dev
```

This starts Wails dev mode and Vite HMR for frontend changes.

## Build and run locally

```bash
mise local
```

This builds the Wails app and opens the generated macOS app bundle.

To only build:

```bash
mise build
```

or use the project-local Wails CLI directly:

```bash
go run github.com/wailsapp/wails/v2/cmd/wails build
```

## Tests

```bash
mise test
mise frontend-build
```

## GitHub Actions macOS build

A manual workflow is available under **Actions → macOS Build**. It builds a macOS `.app`, packages it as a zip, and uploads it as a workflow artifact.

The current artifact is unsigned/not notarized, so macOS Gatekeeper may require right-click → Open or local signing for distribution.
