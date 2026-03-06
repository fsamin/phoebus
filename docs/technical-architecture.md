# Phœbus — Technical Architecture

## 1. Overview

This document describes the technical architecture of Phœbus, an open-source e-learning platform for DevOps training in enterprise environments. It covers the system components, their interactions, data flows, and technology choices.

Phœbus is designed as a **single-tenant**, **self-hosted** application composed of a Go backend, a React frontend, and a PostgreSQL database. All exercises (quizzes, terminal simulations, code challenges) are **interactive browser-based experiences** — there is no server-side code execution, no VM provisioning, and no infrastructure management.

---

## 2. System Context

```
                    ┌──────────────┐
                    │   Learner    │
                    │   Browser    │
                    └──────┬───────┘
                           │ HTTPS
                           ▼
┌──────────────────────────────────────────────────────────┐
│                     Phœbus Platform                      │
│                                                          │
│  ┌─────────────────┐      ┌─────────────────┐           │
│  │    Frontend      │      │    Backend       │           │
│  │    (React SPA)   │◄────▶│    (Go)          │           │
│  │                  │      │                  │           │
│  │ • Learning UI    │      │ • REST API       │           │
│  │ • Terminal       │      │ • Webhook API    │           │
│  │   simulator      │      │ • Auth (OIDC)    │           │
│  │ • Code viewer    │      │ • Content Syncer │           │
│  │   (Monaco)       │      │ • Progress       │           │
│  │ • Admin UI       │      │ • Analytics      │           │
│  └─────────────────┘      └────────┬─────────┘           │
│                                    │                     │
│                    ┌───────────────┴───────────────┐     │
│                    ▼                               ▼     │
│           ┌──────────────┐                ┌────────────┐ │
│           │  PostgreSQL  │                │ Git Repos  │ │
│           │              │                │ (ephemeral)│ │
│           └──────────────┘                └────────────┘ │
└──────────────────────────────────────────────────────────┘
                           ▲
                           │ Webhook (HTTPS POST)
                    ┌──────┴───────┐
                    │ Git Hosting  │
                    │ (any)        │
                    └──────────────┘
```

The architecture is intentionally simple. All exercise interactivity happens **client-side**: the browser renders terminal simulations, code viewers, and quizzes using content stored in the database. The backend serves content, validates exercise answers, and tracks progress — it never executes learner code, provisions infrastructure, or manages SSH connections.

---

## 3. Component Architecture

### 3.1 Frontend (React SPA)

The frontend is a single-page application built with **Vite** and **React Router**, producing pure static files with no Node runtime required in production. The compiled assets are **embedded in the Go binary** via `go:embed`, resulting in a single deployment artifact with no CORS configuration needed.

Markdown rendering happens **client-side**: the backend sends raw Markdown to the browser, where **remark/rehype** render it with syntax highlighting and **Mermaid** renders diagrams natively. This keeps the Content Syncer simple and provides an evolvable rendering pipeline.

| Subcomponent | Technology | Responsibility |
|---|---|---|
| Learning UI | React + Ant Design + React Router | Learning path catalog, module/step navigation, progress display |
| Terminal Simulator | React (custom component) | Terminal-like UI for command-selection exercises (prompt, output, command proposals) |
| Code Viewer | Monaco Editor (read-only) | VS Code-like IDE layout: file tree, syntax highlighting, line selection, resizable panels |
| Quiz UI | React + Ant Design | Multiple-choice and short-answer question rendering with feedback |
| Admin UI | React + Ant Design | Git repo management, user management, analytics dashboards |
| Markdown Renderer | React + remark/rehype | Render lesson content with syntax highlighting, Mermaid diagrams, embedded media |
| Theme System | React Context + CSS Variables | Light/dark mode with system detection, user toggle, and localStorage persistence |

**Key design notes:**
- The **Terminal Simulator** is not a real terminal emulator (no xterm.js). It renders an immersive terminal window (dark background, macOS-style title bar, monospace font) where command history, active prompt, command suggestions, and feedback all live inside the terminal. Suggestions are clickable items styled as shell commands; selecting one fills the prompt line, and pressing Enter submits. All content comes from the database.
- The **Code Viewer** uses Monaco Editor in **read-only mode** with a full-bleed VS Code-like layout (file explorer left, editor center, resizable bottom panel for controls). Monaco theme switches between `vs` (light) and `vs-dark` (dark) based on the active theme. Learners cannot edit code — they can browse files, click on lines (for identification modes), and review diffs. No language server, no IntelliSense.
- The **Quiz UI** renders checkbox-style questions parsed from Markdown. Validation is **server-side**: the frontend sends the learner's answers to the backend, which validates them against `exercise_data` and returns a verdict with explanations. Correct answers are never exposed to the client, ensuring analytics integrity.
- The **Theme System** uses CSS custom properties (variables) defined on `:root` (light) and `[data-theme="dark"]` (dark). All components consume `var(--color-xxx)` tokens — no hardcoded color values. Ant Design's `ConfigProvider` switches between `defaultAlgorithm` and `darkAlgorithm`. The header maintains a fixed dark branding identity regardless of theme. A `ThemeContext` provides `{ mode, toggle, isDark }` to all components.

### 3.2 Backend (Go)

The backend is a **single Go binary (monolith)** that exposes a REST API and manages all server-side logic. Internal isolation is achieved via Go packages — the target scale (200 concurrent users) does not justify microservices. The HTTP layer uses the **Chi** router, which is stdlib-compatible (`http.Handler`), supports composable middleware and route groups for RBAC, and has zero transitive dependencies.

Content sync is driven by a **PostgreSQL-based job queue**: when a webhook arrives, the backend inserts a job into the `sync_jobs` table. A worker goroutine consumes jobs via `SELECT FOR UPDATE SKIP LOCKED`, providing retry semantics, sync history for the admin UI, and multi-replica safety with no external dependency beyond PostgreSQL.

The backend has no code execution, no infrastructure orchestration, and no WebSocket endpoints.

#### 3.2.1 API Layer

| Endpoint Group | Description |
|---|---|
| `GET /api/learning-paths` | List learning paths with metadata, aggregated competencies (`competencies_provided`), and `prerequisites_met` status for the authenticated learner. Supports `?sort=competency-path` for topological ordering |
| `GET /api/learning-paths/{id}` | Retrieve learning path detail with modules (including competencies) and step summaries |
| `GET /api/learning-paths/{id}/steps/{id}` | Retrieve step content including exercise data (proposals, patches, codebase files) |
| `GET /api/competencies` | List all distinct competencies across all modules (for catalog filter). Returns `[{ name, learning_path_ids }]` |
| `POST /api/auth/login` | Local/LDAP authentication (sets httpOnly JWT cookie) |
| `POST /api/auth/register` | Self-registration for local auth (creates learner, sets JWT cookie) |
| `POST /api/webhooks/{uuid}` | Git-provider-agnostic webhook endpoint to trigger content sync |
| `GET/POST /api/progress` | Learner progress tracking (read/write) |
| `POST /api/exercises/{id}/attempt` | Submit exercise answers for server-side validation; returns verdict + explanations |
| `POST /api/exercises/{id}/reset` | Reset exercise progress for a learner |
| `GET /api/admin/repos` | Manage registered Git repositories (CRUD, sync status) |
| `POST /api/admin/repos/{id}/sync` | Manually trigger content sync for a repository |
| `GET /api/admin/instructor-users` | List users with instructor/admin role (for owner selection) |
| `POST /api/admin/users` | Create a local user (admin only, local auth must be enabled) |
| `GET /api/analytics/*` | Learner and instructor analytics |
| `GET /api/instructor/repos` | List repositories owned by the authenticated instructor |
| `GET /api/instructor/repos/{id}` | View owned repo details (ownership verified) |
| `POST /api/instructor/repos/{id}/sync` | Trigger sync on owned repo (ownership verified) |
| `GET /api/instructor/repos/{id}/sync-logs` | View sync logs on owned repo (ownership verified) |
| `GET /api/users/*` | User management (admin only) |
| `GET /api/assets/{hash}` | Serve binary assets (images, videos) by content hash. Public, immutable caching |
| `GET /api/me/onboarding` | Returns onboarding tour status (`{ tour_name: boolean }`) for the authenticated user |
| `PATCH /api/me/onboarding` | Marks a tour as seen (body: `{"tour":"<name>"}`) |
| `DELETE /api/me/onboarding` | Resets all onboarding tours for the authenticated user |

#### 3.2.2 Internal Services

| Service | Responsibility |
|---|---|
| **Content Syncer** | Consumes sync jobs, clones Git repos (ephemeral), parses content structure (front matter + Markdown), syncs binary assets (images, videos) to the asset store, stores codebase files, updates database |
| **Auth Service** | Handles OIDC/LDAP authentication (with local auth fallback), session management, RBAC |
| **Progress Service** | Tracks learner progress, exercise attempts, competency acquisition |
| **Analytics Service** | Aggregates progress data for instructor dashboards (completion rates, failure points, attempt distributions) |

### 3.3 Database (PostgreSQL)

PostgreSQL is the sole data store for all persistent data. All content lives in the database — Git clones are ephemeral and used only during sync. This enables simple backups (`pg_dump`), transparent multi-replica operation, and eliminates shared filesystem dependencies.

Git credentials are protected with **application-level AES-256-GCM encryption**. The encryption key is provided via `configstore` (environment variable or file tree) and never sent to PostgreSQL. A database dump is unexploitable without the application key.

The `content_md` column stores **raw Markdown**, consistent with client-side rendering. The `exercise_data` column stores **parsed JSONB** produced by the Content Syncer at sync time — this is required by server-side validation, enables SQL queries, and ensures format errors are caught at import rather than at read time. The `codebase_files` table stores file contents **inline in PostgreSQL** (pedagogical codebases are KB-scale), keeping the database fully self-contained.

#### 3.3.1 Schema Overview

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│ learning_paths   │────▶│ modules          │────▶│ steps            │
│                  │     │                  │     │                  │
│ id (PK)          │     │ id (PK)          │     │ id (PK)          │
│ repo_id (FK)     │     │ learning_path_id │     │ module_id (FK)   │
│ title            │     │ title            │     │ title            │
│ description      │     │ description      │     │ type (enum)      │
│ metadata (JSONB) │     │ position         │     │ position         │
│ content_hash     │     │ competencies     │     │ content_md       │
│ deleted_at       │     │   (JSONB)        │     │ exercise_data    │
│ synced_at        │     │ content_hash     │     │   (JSONB)        │
└─────────────────┘     │ deleted_at       │     │ metadata (JSONB) │
                         └─────────────────┘     │ content_hash     │
                                                  │ deleted_at       │
                                                  └────────┬─────────┘
                                                           │
┌─────────────────┐     ┌─────────────────┐               │
│ users            │     │ progress         │◄──────────────┘
│                  │     │                  │
│ id (PK)          │     │ id (PK)          │
│ external_id      │     │ user_id (FK)     │
│ email            │     │ step_id (FK)     │
│ display_name     │     │ status (enum)    │
│ role (enum)      │     │ started_at       │
│ onboarding_tours │     │ completed_at     │
│ _seen (JSONB)    │     │                  │
│ created_at       │     │                  │
└─────────────────┘     └─────────────────┘

┌─────────────────┐     ┌─────────────────────┐
│ git_repositories │     │ exercise_attempts    │
│                  │     │                      │
│ id (PK)          │     │ id (PK)              │
│ webhook_uuid     │     │ user_id (FK)         │
│ clone_url        │     │ step_id (FK)         │
│ branch           │     │ attempt_number       │
│ auth_type (enum) │     │ answers (JSONB)      │
│ credentials      │     │ is_correct (BOOLEAN) │
│   (encrypted)    │     │ created_at           │
│ last_synced_at   │     └─────────────────────┘
│ sync_status      │
└─────────────────┘

┌─────────────────┐
│ instance_settings│
│                  │
│ key (PK)         │
│ value (TEXT)     │
│ encrypted (BOOL) │
│ created_at       │
│ updated_at       │
└─────────────────┘

┌─────────────────┐     ┌─────────────────────┐
│ codebase_files   │     │ audit_log            │
│                  │     │                      │
│ id (PK)          │     │ id (PK)              │
│ step_id (FK)     │     │ actor_id (FK)        │
│ file_path        │     │ action (enum)        │
│ content          │     │ resource_type        │
│ language         │     │ resource_id          │
│ position         │     │ metadata (JSONB)     │
└─────────────────┘     │ created_at           │
                         └─────────────────────┘

┌─────────────────────┐
│ sync_jobs            │
│                      │
│ id (PK)              │
│ repo_id (FK)         │
│ status (enum)        │
│ attempts             │
│ last_error           │
│ created_at           │
│ started_at           │
│ completed_at         │
└─────────────────────┘
```

**Key schema notes:**

- **`steps.type`** — enum: `lesson`, `quiz`, `terminal-exercise`, `code-exercise`
- **`steps.exercise_data`** — JSONB column containing the parsed exercise structure (produced by the Content Syncer at sync time):
  - For **quizzes**: array of questions with choices and correct answers
  - For **terminal exercises**: array of steps with prompt, proposals, correct command, and simulated output
  - For **code exercises**: exercise mode, target lines, array of patches with diffs and correctness flags
- **`steps.content_md`** — raw Markdown body (the content after front matter), rendered client-side by remark/rehype
- **`content_hash`** (on `learning_paths`, `modules`, `steps`) — `TEXT` column storing the SHA-256 content hash. Used by the Content Syncer to skip unchanged elements during re-sync (zero DB writes for unchanged content). Step hash = `SHA256(title + type + duration + content_md + exercise_data)`; module hash = `SHA256(metadata + concatenated step hashes)`; path hash = `SHA256(metadata + concatenated module hashes)`
- **`deleted_at`** (on `learning_paths`, `modules`, `steps`) — nullable `TIMESTAMPTZ` for soft-delete. Content removed from the repo during sync is flagged (`deleted_at = NOW()`), never physically deleted. If an item reappears at the same path, it is auto-restored (`deleted_at = NULL`). Learner APIs filter `WHERE deleted_at IS NULL`; analytics include deleted items with a visual indicator
- **`progress.step_id`** and **`exercise_attempts.step_id`** — FK changed from `ON DELETE CASCADE` to `ON DELETE SET NULL` (step_id is nullable). Learner progress is **never** lost, even if a step is eventually purged. Migration: `007_content_hash_sync.up.sql`
- **`exercise_attempts`** — records every attempt a learner makes on an exercise. The `answers` JSONB stores the learner's selections (which command, which patch, which quiz answers). This enables instructors to analyze how learners approach problems.
- **`codebase_files`** — stores the file contents from code exercise `codebase/` directories inline in PostgreSQL, keyed by step and file path. Served to the frontend for Monaco Editor rendering.
- **`content_assets`** — deduplicated binary asset storage. Each unique file is identified by its SHA-256 hash. Stores metadata (MIME type, filename, size) while the actual binary data lives in the asset store (filesystem or S3). Assets are served via `GET /api/assets/{hash}` with immutable caching.
- **`step_assets`** — N:N relationship between steps and content_assets. Tracks which assets are used by each step, with the original relative path for reference.
- **`progress`** — tracks step-level completion status. `status` enum: `not_started`, `in_progress`, `completed`
- **`sync_jobs`** — PostgreSQL-based job queue for content sync. Webhook inserts a row; a worker goroutine consumes via `SELECT FOR UPDATE SKIP LOCKED`. Tracks status (`pending`, `running`, `completed`, `failed`), attempt count, and error messages. Provides retry semantics, sync history for the admin UI, and multi-replica coordination.
- **`users.onboarding_tours_seen`** — `JSONB NOT NULL DEFAULT '{}'` column tracking which onboarding tours the user has completed (e.g., `{"dashboard": true, "catalog": true}`). Added in migration `011`. Used by `GET/PATCH/DELETE /api/me/onboarding` endpoints.
- **`repository_owners`** — N:N join table between `git_repositories` and `users`. Composite primary key `(repo_id, user_id)` with cascading deletes. Tracks which instructors own which repositories, enabling ownership-based access control on instructor routes. Added in migration `012`.

### 3.4 Content Syncer

The Content Syncer is responsible for keeping the database in sync with Git repositories. It runs as a worker goroutine that consumes jobs from the `sync_jobs` table.

Git repos are cloned into **`/tmp` (ephemeral)**, parsed, their content stored in PostgreSQL, and then the clone is deleted. There is no persistent disk state for content text, no shared filesystem, and no PVC needed. A full clone of a pedagogical content repo (few MB) takes 5–15 seconds — acceptable for occasional syncs.

Binary assets (images, videos, PDFs) discovered in `assets/` directories are uploaded to the **Asset Store** — a pluggable storage backend implementing the `assets.Store` interface:

| Backend | Use case | Storage path |
|---------|----------|--------------|
| **Filesystem** (default) | Development, small deployments | `{data_dir}/{hash[0:2]}/{hash}` |
| **S3** | Production, cloud deployments | `{bucket}/{prefix}/{hash}` (any S3-compatible: AWS, MinIO, etc.) |

Assets are deduplicated by SHA-256 hash: the same image used in 10 steps is stored once. Relative URLs in markdown (`./assets/img.png`) are rewritten to `/api/assets/{hash}` during sync, enabling immutable HTTP caching (`Cache-Control: public, max-age=31536000, immutable`).

Steps removed from the repo are **soft-deleted** with a `deleted_at` timestamp rather than physically deleted, preserving learner progress and exercise attempts. If a step file reappears at the same path, it is automatically restored (`deleted_at = NULL`).

The content parser is an **internal Go package** used by the Content Syncer. The same binary also exposes a **`phoebus validate`** CLI subcommand for local and CI validation — instructors run `phoebus validate .` in their content repo to catch format errors before pushing. Same parsing code, zero divergence.

**Flow:**
```
Webhook POST ──▶ Insert job into sync_jobs table
                              │
                              ▼
                     Worker picks job (SELECT FOR UPDATE SKIP LOCKED)
                              │
                              ▼
                     Lookup repo by UUID ──▶ git clone to /tmp
                              │
                              ▼
                     Discover learning paths:
                       1. Check root for phoebus.yaml → single-path
                       2. Else scan subdirectories → multi-path
                              │
                              ▼
                     For each learning path:
                       Parse phoebus.yaml (metadata)
                              │
                              ▼
                       For each module directory:
                         Parse index.md (module metadata)
                              │
                              ▼
                       For each step file/directory:
                         Parse front matter + Markdown body
                         Extract exercise_data from body
                         (quiz questions, terminal steps, patches)
                              │
                              ▼
                       For code exercises:
                         Read codebase/ directory files
                         Store in codebase_files table
                              │
                              ▼
                     Compute SHA-256 content hashes
                     (step → module → path)
                              │
                              ▼
                     Compare hashes with stored values;
                     skip unchanged subtrees (0 DB writes)
                              │
                              ▼
                     Upsert learning_paths, modules,
                     steps, codebase_files in PostgreSQL
                     (within a transaction)
                     Soft-delete removed content
                     (steps, modules, learning_paths)
                              │
                              ▼
                     Update sync_jobs status,
                     git_repositories.last_synced_at
                              │
                              ▼
                     Delete ephemeral clone from /tmp
```

**Key Design Decisions:**
- Git repos are cloned to `/tmp` (ephemeral) and deleted after sync — no persistent disk state
- The syncer first **discovers learning paths** (`discoverLearningPaths`): if `phoebus.yaml` exists at the repo root, the repo is treated as a single learning path; otherwise, immediate subdirectories containing `phoebus.yaml` are each treated as separate learning paths
- The syncer then parses `index.md` (modules) and step Markdown files within each learning path
- Front matter is parsed using a YAML parser; body is stored as raw Markdown
- Exercise logic is **extracted from the Markdown body** during sync: checkbox syntax is parsed into structured JSONB (`exercise_data`) so the backend can validate answers at runtime without re-parsing Markdown
- Ordering is determined by the directory/file naming convention (numeric prefix)
- Code exercise `codebase/` directories are read file-by-file and stored in `codebase_files` with their relative paths
- Sync is **all-or-nothing per repository**: if any file fails to parse, the entire sync is rolled back and an error is surfaced to administrators
- **Hash-based skip logic**: each level (learning path, module, step) carries a SHA-256 content hash. On re-sync, hashes are compared top-down; unchanged subtrees are skipped entirely (zero DB writes). This makes re-syncing unchanged content essentially free
- Existing learner progress is **never deleted** during content sync — steps may be updated or soft-deleted but `progress` and `exercise_attempts` records are preserved. FK constraints use `ON DELETE SET NULL` (not CASCADE)
- Steps, modules, and learning paths removed from the repo are **soft-deleted** (`deleted_at` timestamp), not physically deleted. Re-publishing the same file path auto-restores the item

---

## 4. Data Flows

### 4.1 Content Publication Flow

```
Instructor pushes to Git
         │
         ▼
Git hosting sends POST to /api/webhooks/{uuid}
         │
         ▼
Backend inserts job into sync_jobs table
         │
         ▼
Worker picks job, clones repo to /tmp
         │
         ▼
Parse phoebus.yaml, index.md files, step Markdown files
         │
         ▼
Extract exercise_data from Markdown body (quizzes, terminal steps, patches)
         │
         ▼
Read codebase/ files for code exercises
         │
         ▼
Upsert into PostgreSQL (learning_paths, modules, steps, codebase_files)
Soft-delete removed content (steps, modules, learning_paths)
Compare SHA-256 hashes; skip unchanged subtrees
         │
         ▼
Delete ephemeral clone from /tmp
         │
         ▼
Content available to learners on next page load
```

### 4.2 Exercise Completion Flow (all exercise types)

```
Learner opens a step
         │
         ▼
Frontend: fetches step content from API
  (content_md + exercise UI metadata — correct answers excluded)
         │
         ▼
Frontend: renders exercise UI
  • Quiz → checkbox questions
  • Terminal Exercise → terminal simulator with command proposals
  • Code Exercise → Monaco read-only viewer + patch proposals
         │
         ▼
Learner interacts (selects answer / command / patch)
         │
         ▼
Frontend: POST /api/exercises/{id}/attempt
  { answers: {...} }
         │
         ▼
Backend: validates answers against exercise_data
Backend: stores exercise_attempt record (answers + is_correct)
Backend: returns verdict + explanations to frontend
         │
         ▼
Frontend: displays result (correct/incorrect feedback + explanations)
         │
         ▼
On completion: Backend updates progress record,
  recalculates module/path completion
```

### 4.3 Terminal Exercise — Step-by-Step Interaction

```
Frontend loads terminal exercise (N steps)
         │
         ▼
Display Step 1: context + prompt + command proposals
         │
         ▼
Learner selects a command
         │
         ├── Incorrect → Show explanation, allow retry (same step)
         │
         └── Correct → Display simulated output
                  │
                  ▼
         Advance to Step 2 (repeat until all steps complete)
                  │
                  ▼
         All steps correct → Exercise completed
```

### 4.4 Code Exercise — Interaction (Identify & Fix mode)

```
Frontend loads code exercise
         │
         ▼
Display Monaco Editor (read-only) with file tree + problem description
         │
         ▼
Phase 1: Learner clicks on lines they believe are problematic
         │
         ▼
Frontend sends selected lines to backend for validation
         │
         ├── Incorrect → Feedback, allow retry
         │
         └── Correct → Advance to Phase 2
                  │
                  ▼
Phase 2: Display proposed patches (unified diffs)
         │
         ▼
Learner selects a patch
         │
         ├── Incorrect → Show explanation, allow retry
         │
         └── Correct → Show explanation, exercise completed
```

---

## 5. Deployment Architecture

### 5.1 Docker Compose Deployment

```
┌──────────────────────────────────────┐
│           Docker Compose             │
│                                      │
│  ┌─────────────┐  ┌──────────────┐  │
│  │  phoebus     │  │  postgres    │  │
│  │  (Go+React)  │  │  (5432)     │  │
│  │  :8080       │  │              │  │
│  └─────────────┘  └──────────────┘  │
│                                      │
│  Volume: pg-data/                    │
└──────────────────────────────────────┘
```

The deployment model is **Docker Compose only** — no Helm chart, no Kubernetes manifests. The setup consists of two containers and one volume. The `phoebus` container serves both the Go API and the React static assets (embedded via `go:embed`). Git clones are ephemeral (in `/tmp` inside the container, no volume needed). PostgreSQL is included by default in the `docker-compose.yml`; for production with an external database, the admin removes the `postgres` service and sets the `DATABASE_URL` environment variable.

No shared filesystem is needed across replicas. All content lives in PostgreSQL (codebase files stored inline, Git clones ephemeral), and content sync is coordinated via the PostgreSQL job queue (`SKIP LOCKED`). One replica takes the sync job, clones to `/tmp`, writes to PostgreSQL, and deletes the clone. All replicas read content from the database. Running multiple instances behind a load balancer is straightforward. If Kubernetes deployment is needed in the future, the single Docker image can be deployed via any orchestrator.

---

## 6. Security Architecture

### 6.1 Authentication & Authorization

```
Browser ──▶ [Reverse Proxy] ──▶ Backend ──▶ OIDC Provider / LDAP
                (optional)          │              (or local auth fallback)
                   │                ▼
            X-Remote-User    JWT session token
            X-Remote-Groups  (httpOnly cookie, SameSite=Lax)
                                    │
                                    ▼
                             RBAC middleware
                             ┌────────────────┐
                             │ admin          │ Full access (repos, users, all analytics)
                             │ instructor     │ Content management + analytics + owned repos (sync, logs)
                             │ learner        │ Learning paths + own progress
                             └────────────────┘
```

**Forced admin users:** The `admin.forced_admins` configuration list allows specific usernames to be permanently assigned the `admin` role. This override is applied at user creation and on every login (upsert) across all authentication providers (OIDC, proxy, local). The role is locked: the `PATCH /api/admin/users/{userId}` endpoint rejects role changes for these users (HTTP 403), and the `GET /api/admin/users` response includes a `role_locked: true` field so the frontend can disable the role selector.

**Repository ownership:** Administrators can assign instructors (or admins) as owners of git repositories via the `repository_owners` join table (N:N). Owners gain read-only access to their repositories through `/api/instructor/repos/*` routes: list owned repos, view details, trigger sync, and read sync logs. Ownership is verified server-side via a Chi middleware (`instructorOwnerMiddleware`) that checks the `repository_owners` table before allowing access (admins bypass the check). The catalog API enriches learning paths with deduplicated owner display names (using `SELECT DISTINCT` to avoid repeating the same instructor when a repository contains multiple learning paths), and the instructor dashboard includes a "My Repositories" section with repo URL, sync status, and action buttons.

Authentication supports four providers:

1. **OIDC** — Redirect-based SSO via OpenID Connect
2. **LDAP** — Username/password with LDAP bind verification
3. **Local** — Bcrypt-hashed passwords in the database (bootstrap/dev fallback)
4. **Reverse Proxy** — Transparent auth via HTTP headers injected by an upstream proxy (OAuth2 Proxy, Authelia, Traefik Forward Auth). Header names are configurable (`X-Remote-User`, `X-Remote-Groups`, etc.). When enabled, the `ProxyAuthMiddleware` runs before `AuthMiddleware`: if the user header is present and no valid JWT exists, the user is upserted and a JWT cookie is set. If a valid JWT already exists, the upsert is skipped.

JWT session tokens are stored in **httpOnly cookies** with `SameSite=Lax`, preventing XSS-based token theft and CSRF from cross-origin requests.

### 6.2 Network Security

```
Internet ──▶ [Reverse Proxy / LB] ──▶ Backend (:8080)
                                           │
                                           │ PostgreSQL (internal only)
                                           ▼
                                      PostgreSQL (:5432)
```

- The architecture has a **minimal attack surface**: no SSH, no code execution, no infrastructure provisioning
- The backend is the only externally-facing component (behind a reverse proxy)
- PostgreSQL is **never** exposed to the internet
- Git credentials (SSH keys or tokens) are encrypted at rest in PostgreSQL
- Webhook endpoints use **UUID only** (122-bit UUIDv4) — knowledge of the UUID is required to trigger a sync. The webhook body is ignored (not used for anything), so HMAC signature verification is unnecessary. This preserves Git-provider agnosticism (signature formats vary per provider). If a UUID is compromised, the admin regenerates it from the UI.

**Content Security Policy (CSP) Headers:**

A backend middleware sets the following headers on all responses:

| Directive | Value | Rationale |
|---|---|---|
| `default-src` | `'self'` | Same-origin only |
| `script-src` | `'self' blob:` | `blob:` for Monaco Editor web workers |
| `worker-src` | `'self' blob:` | Monaco workers loaded as blobs |
| `style-src` | `'self' 'unsafe-inline'` | Ant Design inline styles |
| `img-src` | `'self' data:` | Data URIs for inline images |
| `font-src` | `'self' data:` | Embedded fonts |
| `connect-src` | `'self'` | API calls to same origin |
| `frame-ancestors` | `'none'` | Prevent clickjacking |

Additional headers: `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: strict-origin-when-cross-origin`, `X-XSS-Protection: 0`.

### 6.3 Git Credential Management

Git repositories may be private, requiring authentication for clone/pull operations:

| Auth Type | Storage | Usage |
|---|---|---|
| **Instance SSH Key** | Ed25519 keypair in `instance_settings` (private key encrypted) | Default for SSH repos — uses the instance's auto-generated keypair via `GIT_SSH_COMMAND` |
| **HTTPS Token** | Token encrypted in `git_repositories.credentials` | Used with `git clone https://token@...` |
| **HTTPS Basic** | Username:password encrypted in `git_repositories.credentials` | Used with `git clone` + HTTP basic auth header |
| **None** | — | Public repositories, no credentials needed |

- Credentials are encrypted at application level before storage (AES-256-GCM with a key managed as a `configstore` item)
- Credentials are decrypted in-memory only during Git operations and never logged

### 6.4 Instance SSH Key Management

At first startup, Phœbus generates a unique **Ed25519 SSH keypair** for the instance:

1. **Startup check:** Query `instance_settings` for keys `ssh_private_key` and `ssh_public_key`
2. **If not found:** Generate a new Ed25519 keypair → encrypt the private key (AES-256-GCM) → store both in `instance_settings` (`ssh_private_key` encrypted, `ssh_public_key` in clear text)
3. **If found:** Decrypt the private key from the database and hold it in memory

The public key is exposed via `GET /api/admin/ssh-public-key` (admin-only) for display in the repository management UI. Administrators copy this key and add it as a **read-only deploy key** on their Git hosting platform.

When the Content Syncer clones a repo with `auth_type = instance-ssh-key`:
1. Write the decrypted private key to a temporary file (mode `0600`)
2. Run `ssh-keyscan {host}` to populate a temporary `known_hosts` file
3. Set `GIT_SSH_COMMAND="ssh -i /tmp/key -o UserKnownHostsFile=/tmp/known_hosts -o StrictHostKeyChecking=yes"`
4. Execute `git clone --depth 1`
5. Delete the temporary key and known_hosts files immediately after clone

### 6.4 Configuration & Secrets Management

All configuration — including the AES-256-GCM encryption key — is managed through **`github.com/ovh/configstore`**. The store is initialized from environment variables (`CONFIGURATION_FROM=env`) for development or from a file tree (`CONFIGURATION_FROM=filetree:/etc/phoebus/conf.d`) for production. The encryption key is a configstore item like any other configuration value, keeping secret management consistent and auditable.

### 6.5 Content Security

**Markdown XSS Prevention:**

The frontend Markdown rendering pipeline includes `rehype-sanitize` (after `rehypeRaw`) with a strict schema:

- **Allowed classes:** hljs highlight classes, admonition directive classes, Mermaid classes
- **Blocked elements:** `<script>`, `<style>`, `<iframe>`, `<object>`, `<embed>`, `<form>`, `<textarea>`
- **Protocol whitelist:** `href` allows `http`, `https`, `mailto` only; `src` allows `http`, `https` only — blocks `file://`, `javascript:`, and `data:` URIs in links

**Mermaid SVG Sanitization:**

- All Mermaid SVG output is sanitized through **DOMPurify** with `USE_PROFILES: { svg: true }` before injection into the DOM
- Mermaid itself is configured with `securityLevel: 'strict'`

**Monaco Editor CSP Compliance:**

- Monaco is loaded as a local bundle (no CDN) via `loader.config({ monaco })` with Vite worker imports
- `self.MonacoEnvironment.getWorker()` is configured to use blob workers, compatible with the `script-src 'self' blob:` CSP directive
- `manualChunks` in `vite.config.ts` splits Monaco into a separate ~3.7 MB chunk

---

## 7. Technology Stack Summary

| Layer | Technology | Version Policy |
|---|---|---|
| **Backend** | Go | Latest stable |
| **HTTP Router** | Chi | Latest stable |
| **Database Access** | sqlx (struct mapping over `database/sql`) | Latest stable |
| **Frontend** | React + Ant Design | Latest stable |
| **Frontend Build** | Vite + React Router | Latest stable |
| **Code Viewer** | Monaco Editor (read-only) | Latest stable |
| **Markdown Rendering** | remark / rehype / Mermaid | Latest stable |
| **Database** | PostgreSQL | 15+ |
| **Configuration** | `github.com/ovh/configstore` | Latest stable |
| **Authentication** | OIDC / LDAP (+ local fallback) | Standard protocols |
| **Content Format** | Markdown + YAML front matter | — |
| **Content Transport** | Git (SSH / HTTPS) | — |
| **Containerization** | Docker | For deployment |
| **Orchestration** | Docker Compose | Single deployment model |
| **Observability** | Structured logging (JSON) + Prometheus metrics | — |
