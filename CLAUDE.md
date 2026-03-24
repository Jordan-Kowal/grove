# Grove - Instructions for Claude Code

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Context Files

Reference these for project details:

- @Taskfile.yml - Developer commands (dev, build, lint, test, check)
- @package.json - Dependencies and their versions

## Project Overview

Lightweight worktree dashboard — manage workspaces, monitor Claude Code sessions, git diffs, and /tmp disk across multiple worktrees.

Tech stack: SolidJS, Vite, Tailwind CSS v4 + DaisyUI, Wails v3 (Go), Lucide Solid, Bun

## Project Structure

```txt
src/
  features/{name}/       # Feature-specific: components/, contexts/
  components/ui/         # Shared UI primitives
  contexts/              # Shared contexts (Context.ts + Provider.tsx pattern)
  hooks/                 # Shared hooks
  types/                 # Shared types (types.ts)
  backend.d.ts           # Type declarations for Wails-generated bindings

backend/
  workspace_service.go   # Workspace/worktree CRUD, git operations, script execution, log streaming
  workspace_types.go     # Workspace, WorkspaceConfig types
  monitor_service.go     # Polling, Claude status detection, event emission
  app_service.go         # Version info, auto-update via GitHub releases
  editor_service.go      # Editor focus/open (configurable app)
  sound_service.go       # macOS system sound playback (bundled .aiff files)
  tray_service.go        # macOS menu bar (system tray) icon
  types.go               # Shared types (ClaudeStatus, WorktreeInfo, TmpUsage)
  git.go                 # Git diff stat parsing
  fix_path.go            # PATH resolution for GUI apps
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

- Table-driven tests with subtests (`t.Run`), test fixtures in `testdata/`
- Race detector: `go test -race ./...`
- Mock external dependencies via interfaces
