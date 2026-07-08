# AGENTS.md

Guidance for agents working in `azure-exemption-cli`: a single-binary
Bubble Tea TUI that walks a user through creating an Azure Policy
exemption. Written in Go.

## Commands

```bash
go run main.go              # run the TUI (version reports "dev")
go build -o azure-exemption-cli main.go
go run main.go --version    # version/commit/date (only meaningful in release builds)
go vet ./...
go mod tidy                 # GoReleaser runs this before every build
```

- **Go 1.25+ is required** (`go.mod` says `go 1.25.5`). The README's
  "Go 1.21" line is stale — trust `go.mod`.
- **There is no test suite and no CI for tests.** The only workflow
  (`.github/workflows/release.yml`) runs GoReleaser on `v*` tags.
  If you add tests, run them with `go test ./...`.
- `version`/`commit`/`date` in `main.go` are injected via `-ldflags` by
  GoReleaser (`.goreleaser.yaml`); they stay at defaults under `go run`.

## Architecture

Everything Azure-related **shells out to the `az` CLI** via
`exec.CommandContext` — there is no Azure SDK dependency. `az` must be on
`PATH`; `EnsureLogin` runs `az login` interactively if no session exists.
When changing Azure behaviour, edit the `az` argument slices in
`azure/client.go`, not any SDK call.

Package layout:

- `main.go` — loads config, ensures `az` login, starts the Bubble Tea
  program.
- `azure/` — `client.go` (all `az` invocations) + `types.go`. Note
  `ListAssignments` calls `az rest` directly and paginates `nextLink`;
  most other calls use `az policy ...` subcommands.
- `tui/` — the UI is a **hand-rolled state machine**, not the Bubbles
  `list` component. It spans four files that must move together:
  - `model.go` — `Model` struct + the `Step` enum (the flow order).
  - `update.go` — `handleKey` has one `case` per `Step`; this is where
    navigation, validation, and back/`backspace` transitions live.
  - `messages.go` — async `tea.Cmd`s that call `azure.Client` and return
    `*LoadedMsg` messages.
  - `view.go` — rendering per step.
  - Adding or reordering a step means updating the enum, `handleKey`,
    the loaders, and `view.go` consistently.
- `config/config.go` — optional YAML config loader (see below).

## Conventions and gotchas

- **Blocked policies are keyed by policy *definition* ID, not assignment
  ID** (`BlockedDefinitionsMap`, `IsDefinitionBlocked`). One blocked
  definition disables every assignment that uses it. Matching is
  case-insensitive (keys lowercased).
- `azure.CreateExemption` builds and **sanitizes** the exemption name:
  whitelist of `[A-Za-z0-9-_.]`, spaces/`/` become `-`, hard-capped at
  64 chars (`sanitizeExemptionName`). Keep this if you touch naming.
- Exemption category is hard-coded to `Waiver`; an expiration date is
  bumped to 23:59:59 of that day before formatting as RFC3339.
- Config file is optional — a missing file returns an empty `Config`,
  **not** an error. Only parse errors propagate. Search order is in
  `DefaultConfigPaths`. `config.go` is split into `Load` /
  `LoadFromPaths` / `LoadFromFile` specifically so paths can be injected
  in tests.
- `config.yaml` at the repo root **is committed** (it is the example,
  not gitignored). Don't put real secrets there.
- Module path is `github.com/Lukas-Klein/azure-exemption-cli` even though
  the local checkout lives under a `Privat/` directory — keep import
  paths as-is.
