# Copilot Instructions for Phœbus

## Project Overview

Phœbus is an open-source e-learning platform for DevOps training with a content-as-code philosophy. Content is authored as Markdown in Git repositories and synced automatically.

## Architecture

- **Backend**: Go 1.23+, Chi router, sqlx, PostgreSQL
- **Frontend**: React 19, TypeScript, Vite, Ant Design 5, embedded via `go:embed`
- **Content**: Markdown with YAML front matter in external Git repos (see `phoebus-content-samples`)
- **Auth**: Local (bcrypt), reverse proxy headers, JWT session cookies
- **Deployment**: Single Docker image, `docker compose up`

## Repository Structure

```
cmd/                    # CLI entrypoint
internal/
  config/               # Configuration loading (YAML files)
  handler/              # HTTP handlers (Chi routes, middleware, auth)
  model/                # Database models (sqlx structs)
  database/migrations/  # SQL migrations (applied at startup)
  syncer/               # Git clone + content parser
  auth/                 # JWT + bcrypt helpers
  crypto/               # AES-256-GCM encryption
frontend/               # React SPA (Vite)
  src/
    components/         # Reusable components (CodeExercise, MarkdownRenderer, etc.)
    pages/              # Route pages (Login, Dashboard, Catalog, Admin, etc.)
    contexts/           # React contexts (Auth, Theme)
e2e/                    # Playwright E2E tests (Dockerized)
docs/                   # Product spec, detailed spec, technical architecture
config.example/         # Example configuration files
```

## Working Conventions

- **Language**: User communicates in French; code, comments, and docs are in English.
- **After every push**: Check GitHub Actions workflow status (CI + E2E) and wait for completion. Report results to the user. If a workflow fails, investigate and fix immediately.
- **Minimal changes**: Make the smallest possible changes to fix issues. Do not refactor unrelated code.
- **Content format**: Learning content uses Markdown with YAML front matter. Step types: `lesson`, `quiz`, `terminal-exercise`, `code-exercise`.

## Key Technical Details

- **CSS**: `frontend/src/index.css` must be imported in `main.tsx` — Vite only bundles imported CSS.
- **Monaco Editor**: Decorations use `radial-gradient` for glyph dots (absolute positioning, no margin effect).
- **Dark theme**: Header uses conditional background (`#1a1a1a` dark / `#001529` light) with `2px solid #ff7a45` accent border. Menu background must be `transparent` in dark mode to override Ant Design defaults.
- **Database**: `steps` table has `deleted_at` column (soft delete); `modules` table does NOT.
- **Auth middleware chain**: `ProxyAuthMiddleware` → `AuthMiddleware` → handler. Proxy middleware sets claims in context; auth middleware skips cookie check if claims already present.
- **Git repo auth_type values**: `none`, `http-basic`, `http-token`, `instance-ssh-key` (CHECK constraint on `git_repositories` table).

## Running

```bash
# Development
docker compose up --build -d

# E2E tests
make e2e                          # Dockerized, no local deps needed
make e2e GITHUB_TOKEN=ghp_xxx     # With content repo access

# Unit tests
cd internal && go test ./...

# Frontend dev
cd frontend && npm run dev
```

## CI/CD

- **CI workflow** (`ci.yml`): Frontend build + Go build + Go tests (with PostgreSQL service container)
- **E2E workflow** (`e2e.yml`): Full Dockerized Playwright tests. Uses `CONTENT_PAT` secret for content repo access.
- E2E sync flag: `e2e/storage-state/content-synced` file (written by global-setup, read by tests).
