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
| Code Viewer | Monaco Editor (read-only) | Multi-file read-only code viewer with file tree, syntax highlighting, line selection, diff display |
| Quiz UI | React + Ant Design | Multiple-choice and short-answer question rendering with feedback |
| Admin UI | React + Ant Design | Git repo management, user management, analytics dashboards |
| Markdown Renderer | React + remark/rehype | Render lesson content with syntax highlighting, Mermaid diagrams, embedded media |

**Key design notes:**
- The **Terminal Simulator** is not a real terminal emulator (no xterm.js). It renders a styled UI that presents a prompt, context text, command proposals as selectable options, and simulated output. All content comes from the database.
- The **Code Viewer** uses Monaco Editor in **read-only mode**. Learners cannot edit code — they can browse files, click on lines (for identification modes), and review diffs. No language server, no IntelliSense.
- The **Quiz UI** renders checkbox-style questions parsed from Markdown. Validation is **server-side**: the frontend sends the learner's answers to the backend, which validates them against `exercise_data` and returns a verdict with explanations. Correct answers are never exposed to the client, ensuring analytics integrity.

### 3.2 Backend (Go)

The backend is a **single Go binary (monolith)** that exposes a REST API and manages all server-side logic. Internal isolation is achieved via Go packages — the target scale (200 concurrent users) does not justify microservices. The HTTP layer uses the **Chi** router, which is stdlib-compatible (`http.Handler`), supports composable middleware and route groups for RBAC, and has zero transitive dependencies.

Content sync is driven by a **PostgreSQL-based job queue**: when a webhook arrives, the backend inserts a job into the `sync_jobs` table. A worker goroutine consumes jobs via `SELECT FOR UPDATE SKIP LOCKED`, providing retry semantics, sync history for the admin UI, and multi-replica safety with no external dependency beyond PostgreSQL.

The backend has no code execution, no infrastructure orchestration, and no WebSocket endpoints.

#### 3.2.1 API Layer

| Endpoint Group | Description |
|---|---|
| `GET /api/learning-paths` | List, search, and retrieve learning paths, modules, steps |
| `GET /api/learning-paths/{id}/steps/{id}` | Retrieve step content including exercise data (proposals, patches, codebase files) |
| `POST /api/auth/login` | Local/LDAP authentication (sets httpOnly JWT cookie) |
| `POST /api/auth/register` | Self-registration for local auth (creates learner, sets JWT cookie) |
| `POST /api/webhooks/{uuid}` | Git-provider-agnostic webhook endpoint to trigger content sync |
| `GET/POST /api/progress` | Learner progress tracking (read/write) |
| `POST /api/exercises/{id}/attempt` | Submit exercise answers for server-side validation; returns verdict + explanations |
| `POST /api/exercises/{id}/reset` | Reset exercise progress for a learner |
| `GET /api/admin/repos` | Manage registered Git repositories (CRUD, sync status) |
| `POST /api/admin/repos/{id}/sync` | Manually trigger content sync for a repository |
| `POST /api/admin/users` | Create a local user (admin only, local auth must be enabled) |
| `GET /api/analytics/*` | Learner and instructor analytics |
| `GET /api/users/*` | User management (admin only) |

#### 3.2.2 Internal Services

| Service | Responsibility |
|---|---|
| **Content Syncer** | Consumes sync jobs, clones Git repos (ephemeral), parses content structure (front matter + Markdown), stores codebase files, updates database |
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
│ synced_at        │     │ competencies     │     │ content_md       │
└─────────────────┘     │   (JSONB)        │     │ exercise_data    │
                         └─────────────────┘     │   (JSONB)        │
                                                  │ metadata (JSONB) │
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
│ created_at       │     │ completed_at     │
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
- **`steps.deleted_at`** — nullable timestamp for soft-delete. Steps removed from the repo during sync are flagged (`deleted_at = NOW()`), not physically deleted. If a step file reappears at the same path, it is auto-restored (`deleted_at = NULL`). Learner APIs filter `WHERE deleted_at IS NULL`; analytics include deleted steps with a visual indicator
- **`exercise_attempts`** — records every attempt a learner makes on an exercise. The `answers` JSONB stores the learner's selections (which command, which patch, which quiz answers). This enables instructors to analyze how learners approach problems.
- **`codebase_files`** — stores the file contents from code exercise `codebase/` directories inline in PostgreSQL, keyed by step and file path. Served to the frontend for Monaco Editor rendering.
- **`progress`** — tracks step-level completion status. `status` enum: `not_started`, `in_progress`, `completed`
- **`sync_jobs`** — PostgreSQL-based job queue for content sync. Webhook inserts a row; a worker goroutine consumes via `SELECT FOR UPDATE SKIP LOCKED`. Tracks status (`pending`, `running`, `completed`, `failed`), attempt count, and error messages. Provides retry semantics, sync history for the admin UI, and multi-replica coordination.

### 3.4 Content Syncer

The Content Syncer is responsible for keeping the database in sync with Git repositories. It runs as a worker goroutine that consumes jobs from the `sync_jobs` table.

Git repos are cloned into **`/tmp` (ephemeral)**, parsed, their content stored in PostgreSQL, and then the clone is deleted. There is no persistent disk state, no shared filesystem, and no PVC needed. A full clone of a pedagogical content repo (few MB) takes 5–15 seconds — acceptable for occasional syncs.

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
                     Parse directory structure
                              │
                              ▼
                     Parse phoebus.yaml (learning path metadata)
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
                     Upsert learning_paths, modules,
                     steps, codebase_files in PostgreSQL
                     (within a transaction)
                     Soft-delete steps no longer in repo
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
- The syncer parses the directory tree, reading `phoebus.yaml`, `index.md` (modules), and step Markdown files
- Front matter is parsed using a YAML parser; body is stored as raw Markdown
- Exercise logic is **extracted from the Markdown body** during sync: checkbox syntax is parsed into structured JSONB (`exercise_data`) so the backend can validate answers at runtime without re-parsing Markdown
- Ordering is determined by the directory/file naming convention (numeric prefix)
- Code exercise `codebase/` directories are read file-by-file and stored in `codebase_files` with their relative paths
- Sync is **all-or-nothing per repository**: if any file fails to parse, the entire sync is rolled back and an error is surfaced to administrators
- Existing learner progress is **never deleted** during content sync — steps may be updated or soft-deleted but `progress` and `exercise_attempts` records are preserved
- Steps removed from the repo are **soft-deleted** (`deleted_at` timestamp), not physically deleted. Re-publishing the same file path auto-restores the step

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
Soft-delete steps no longer present in repo
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
Browser ──▶ Backend ──▶ OIDC Provider / LDAP
                │              (or local auth fallback)
                ▼
         JWT session token
         (httpOnly cookie, SameSite=Lax)
                │
                ▼
         RBAC middleware
         ┌────────────────┐
         │ admin          │ Full access (repos, users, all analytics)
         │ instructor     │ Content management + analytics
         │ learner        │ Learning paths + own progress
         └────────────────┘
```

Authentication supports OIDC and LDAP as primary providers, with a **local auth fallback** for environments without an identity provider. JWT session tokens are stored in **httpOnly cookies** with `SameSite=Lax`, preventing XSS-based token theft and CSRF from cross-origin requests.

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

### 6.3 Git Credential Management

Git repositories may be private, requiring authentication for clone/pull operations:

| Auth Type | Storage | Usage |
|---|---|---|
| **Instance SSH Key** | Ed25519 keypair in `instance_settings` (private key encrypted) | Default for SSH repos — uses the instance's auto-generated keypair via `GIT_SSH_COMMAND` |
| **SSH Key** | Private key encrypted in `git_repositories.credentials` | Per-repo custom SSH key, used with `git clone git@...` |
| **HTTPS Token** | Token encrypted in `git_repositories.credentials` | Used with `git clone https://token@...` |
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
