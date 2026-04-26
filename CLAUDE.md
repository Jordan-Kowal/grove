# Grove - Instructions for Claude Code

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Context Files

Reference these for project details:

- @Taskfile.yml - Developer commands (dev, build, lint, test, check)
- @package.json - Dependencies and their versions

## Project Overview

Sidebar app for managing git worktrees per workspace and monitoring Claude Code sessions running in those directories.

Tech stack: SolidJS, Vite, Tailwind CSS v4 + DaisyUI, Wails v3 (Go), Lucide Solid, Bun

## Project Structure

```txt
src/
  features/{name}/       # Feature-specific: components/ (always), contexts/ + hooks/ (only when feature-local state/logic is needed — e.g. dashboard/)
  components/ui/         # Shared UI primitives
  contexts/              # Shared contexts (Context.ts + Provider.tsx pattern)
  hooks/                 # Shared hooks
  styles/                # Global styles (Tailwind entry point)
  types/                 # Shared types (types.ts)
  utils/                 # Shared utilities (ANSI parsing, version check)
  backend.d.ts           # Type declarations for Wails-generated bindings

backend/
  workspace_service.go          # Workspace/worktree CRUD, async create/remove flows
  workspace_git.go              # Git ops: rebase, checkout, new-branch, list-branches, sync-main, fetch-remote, force-remove, resolve-git-dir
  workspace_script.go           # Script execution + log streaming (runScriptTracked, WorktreeLogEvent)
  workspace_validate.go         # Name + branch name validation, regex consts
  workspace_types.go            # Workspace, WorkspaceConfig, BranchInfo types
  monitor_service.go            # Polling, workspace/git/editor refresh, event emission
  monitor_claude.go             # Claude status detection (sessions, aggregation, dismiss)
  monitor_hook.go               # Claude hook script + installHook
  app_service.go                # Version info, auto-update via GitHub releases
  editor_service.go             # Editor focus/open (configurable app)
  snap_service.go               # Window edge snapping, editor positioning
  sound_service.go              # macOS system sound playback (bundled .aiff files)
  tray_service.go               # macOS menu bar (system tray) icon
  claude_settings.go            # Claude Code hook registration and validation
  accessibility_darwin.go       # macOS AXIsProcessTrusted permission check
  accessibility_other.go        # Non-Darwin stub for the accessibility check
  clickthrough_darwin.go        # macOS click-through (bring-to-front) handling
  types.go                      # Shared types (ClaudeStatus, WorktreeInfo, task events)
  git.go                        # Git diff stat parsing
  fix_path.go                   # PATH resolution for GUI apps
```

**Shared** (`src/{type}/`, `backend/`) — used by 2+ features
**Feature-specific** (`src/features/{name}/{type}/`) — used by single feature only

When in doubt: default to feature-specific (easier to promote later)

## Developer Commands

All commands go through [Task](https://taskfile.dev/). See `Taskfile.yml` for full list.

## Code Style

### Global

- Descriptive names: `isLoading`, `hasError`, `canSubmit`
- Named constants over magic numbers
- Minimal external dependencies — prefer standard library / built-in solutions

### Frontend (TypeScript / SolidJS)

**File Naming:**

- Components: `PascalCase.tsx`
- Hooks: `useCamelCase.ts`
- Utilities/Types/Config: `camelCase.ts`

**Barrel Exports:**

Use `index.ts` at every level **except** `src/components/`:

```txt
src/components/ui/index.ts        (yes)
src/components/index.ts           (no root barrel)
```

**TypeScript:**

- Use `type` over `interface`
- Arrow functions for pure functions
- Named exports only (no default exports, except for page components)
- No SSR/server components — this is a static frontend

**Linting & Formatting:**

- Biome handles both linting and formatting (see `biome.json`)
- Biome auto-runs on save: removes unused imports, sorts imports alphabetically
- When editing: add usage first, then import (otherwise Biome removes the "unused" import)

**Testing:**

- Vitest for pure utilities (`src/**/*.test.ts`), node environment (no jsdom), config in `vitest.config.ts`
- Run via `bun run test` or `task test` (which also runs the Go suite)
- No component-rendering tests yet — add jsdom + `@solidjs/testing-library` if SolidJS DOM tests are needed later

**Styling (DaisyUI + Tailwind):**

- DaisyUI first for UI elements (`btn`, `card`, `badge`) and semantic colors (`bg-base-100`, `text-base-content`)
- Refer to DaisyUI documentation for available modifiers and components
- Tailwind for layout (`flex`, `grid`, `gap-*`), positioning, spacing, transitions
- Avoid raw Tailwind for things DaisyUI handles

**SolidJS Control Flow (Critical):**

- `<Show>` for conditionals, `<For>` for lists, `<Switch>`/`<Match>` for multiple conditions
- NEVER use ternaries for component rendering
- NEVER use `.map()` for rendering lists

**SolidJS Reactivity:**

- `createSignal` for primitive local state
- `createStore` for complex/nested objects
- `createMemo` for derived values (avoid inline computations in JSX)
- `createResource` for async data fetching (not `createEffect` + `createSignal`)
- Signals called as functions in JSX: `{count()}` not `{count}`
- **Avoid `createEffect`**: prefer event handlers to push state to backend/DOM. Only use `createEffect` for genuine DOM side effects with no triggering event (auto-scroll, focus management). Never use it to sync state to backend — do that explicitly in the handler that caused the change.

**Project Patterns:**

- **Context**: `ContextName.ts` (types + createContext) + `ContextNameProvider.tsx` (provider + useHook export)
- **localStorage**: Use `useLocalStorage` hook from `src/hooks/`
- **Navigation**: Signal-based view switching in App.tsx (no router needed)
- **Window resize**: Use `Window` from `@wailsio/runtime` for SetSize/SetAlwaysOnTop

**Wails Bindings:**

- Frontend imports services from `@backend` path alias (-> `frontend/bindings/.../backend`)
- Type declarations in `src/backend.d.ts` (Wails generates `.js` bindings, needs manual `.d.ts`)
- Events from `@wailsio/runtime`: `Events.On()` returns unsubscribe function
- Shared types in `src/types/types.ts`

### Backend (Go / Wails)

**File Naming:**

- Source: `snake_case.go`, tests: `snake_case_test.go`
- Exported types/functions: `PascalCase`, unexported: `camelCase`
- Avoid redundancy in package context (e.g. `workspace.GetWorkspaces` not `workspace.WorkspaceGetWorkspaces`)

**Structure & Wails Integration:**

- Services as Go structs registered with Wails via `application.NewService()`
- One service per domain: `workspace_service.go`, `monitor_service.go`, `editor_service.go`
- Exported methods auto-generate TypeScript bindings via `wails3 generate bindings`
- Main -> renderer streaming uses Wails events (`app.Event.Emit()`)
- Lifecycle hooks: `ServiceStartup(ctx, opts)` / `ServiceShutdown()`

**Linting & Formatting:**

- `gofmt` for formatting, `golangci-lint` for linting (configured via `.golangci.yml`)
- Fix command: `gofmt -w . && golangci-lint run --fix`

**Testing:**

- Table-driven tests with subtests (`t.Run`)
- Race detector: `go test -race ./...`
- Mock external dependencies via interfaces

## Documentation

**You own documentation.** After any impactful change — feature, enhancement, bugfix, refactor, setting change, renamed command, new helper script — you MUST check whether these four files need updating and update them as part of the same change. Do not defer this to a follow-up.

- **README.md** — update if user-facing features, settings, installation, requirements, or "How It Works" internals changed. Keep the Features list, Settings tables, and How It Works section in sync with actual behavior.
- **CHANGELOG.md** — add an entry under the `TBD` section for every user-facing change (feature, enhancement, bugfix, breaking change, removal). Use the emoji legend at the top. Non-user-facing internal refactors do not need an entry.
- **TODO.md** — remove items you complete. Add items for known follow-ups discovered during the change.
- **CONTRIBUTING.md** — update if prerequisites, setup steps, developer commands, environment variables, or helper scripts changed.

Rule of thumb: if a contributor or user would be surprised by the new behavior after reading the current docs, the docs are out of date and you must fix them now.
