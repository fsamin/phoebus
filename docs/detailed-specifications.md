# Phœbus — Detailed Specifications

## 1. Overview

This document provides detailed functional specifications for each feature area of Phœbus. It expands on the use cases and feature breakdown defined in the [Product Specification](./product-specification.md) and references the [Technical Architecture](./technical-architecture.md) for implementation context.

Each section describes the detailed behavior, rules, edge cases, and UI expectations for a feature area.

---

## 2. Content Management

### 2.1 Git Repository Registration

**Description:** Administrators register Git repositories that contain learning path content.

**Detailed Behavior:**

1. Administrator provides:
   - Clone URL (SSH or HTTPS)
   - Authentication type: `none`, `instance-ssh-key`, `http-basic`, `http-token`
   - Credentials (private key, username/password, or token) — stored encrypted
   - Branch to track (default: `main`)

> **Note:** When `instance-ssh-key` is selected, no credentials are needed — Phœbus uses the instance's auto-generated Ed25519 SSH keypair (see section 10.11). The administrator must add the instance public key as a deploy key on the Git repository.

2. Upon registration, Phœbus:
   - Generates a unique webhook UUID
   - Returns the full webhook URL: `https://{phoebus-host}/api/webhooks/{uuid}`
   - Performs an initial clone and sync of the repository
   - Validates the repository structure (presence of `phoebus.yaml`)

3. If the initial clone fails (bad URL, invalid credentials, missing `phoebus.yaml`):
   - The repository is stored with `sync_status: error`
   - An error message is surfaced in the admin UI
   - The repository can be edited and re-synced

**Admin UI — Repository List:**

| Column | Description |
|---|---|
| Name | Learning path title (from `phoebus.yaml`) |
| Clone URL | Git repository URL |
| Branch | Tracked branch |
| Auth | Authentication type |
| Owners | Assigned instructor/admin owners |
| Last Synced | Timestamp of last successful sync |
| Status | `synced`, `syncing`, `error`, `never_synced` |
| Actions | Edit, Sync Now, Delete, Copy Webhook URL |

**Edge Cases:**
- Duplicate clone URLs are allowed (different branches)
- Deleting a repository: content is removed from the database, learner progress is preserved (orphaned but not deleted)
- Credentials can be updated without deleting/re-creating the repository

### 2.2 Webhook-Triggered Content Sync

**Description:** Any POST to `/api/webhooks/{uuid}` triggers a content sync for the associated repository.

**Detailed Behavior:**

1. Webhook endpoint receives a POST request
2. Request body is **ignored** (provider-agnostic — works with GitHub, GitLab, Gitea, Bitbucket, or a simple `curl`)
3. The UUID is looked up in `git_repositories`
4. If UUID is unknown: return `404 Not Found`
5. If the repository is already syncing: return `200 OK` (debounce — ignore duplicate triggers)
6. Otherwise: enqueue a sync job and return `200 OK` immediately

**Sync Process (detailed):**

1. Set `sync_status = syncing`
2. If the clone URL uses the `file://` protocol, use the local directory path directly (no `git clone`). Otherwise, execute `git clone --depth 1` on the tracked branch.
3. Parse directory structure:
   - Read `phoebus.yaml` → update `learning_paths` table
   - For each module directory (ordered by numeric prefix):
     - Read `index.md` → parse front matter → update `modules` table
   - For each step file/directory within each module (ordered by numeric prefix):
     - Single `.md` file → parse front matter + body → update `steps` table
     - Directory with `instructions.md` → parse front matter + body → update `steps` table; read `codebase/` → update `codebase_files` table
4. **Content parsing** (see section 2.3 for per-type details):
   - Extract exercise data from structured Markdown body
   - Store parsed structure in `steps.exercise_data` (JSONB)
   - Store raw Markdown body in `steps.content_md`
5. All database writes are wrapped in a **single transaction** — if any parsing error occurs, the entire sync is rolled back
6. On success: set `sync_status = synced`, update `last_synced_at`
7. On failure: set `sync_status = error`, store error message in `sync_error` column

**Ordering Rules:**
- Module directories are ordered by their numeric prefix: `01-intro/`, `02-basics/`, etc.
- Step files within a module follow the same convention: `01-lesson.md`, `02-quiz.md`, etc.
- If a file/directory has no numeric prefix, it is sorted alphabetically after numbered items
- The numeric prefix is stripped from the display title (the title from front matter is used)

**Hash-Based Content Sync:**

Each level of the content hierarchy carries a **SHA-256 content hash** to avoid unnecessary database writes:

- **Step hash** = `SHA256(title + type + duration + content_md + exercise_data)`
- **Module hash** = `SHA256(module metadata + concatenated step hashes)`
- **Path hash** = `SHA256(path metadata + concatenated module hashes)`

On re-sync, hashes are compared at each level. If a hash matches, the entire subtree is **skipped** (zero DB writes for unchanged content). Only modified elements are upserted. Result: re-syncing unchanged content produces **0 database writes**.

Hash values are stored in a `content_hash TEXT` column on `learning_paths`, `modules`, and `steps`.

**Soft-Delete Rules:**

- Steps, modules, and learning paths removed from the repo are **soft-deleted** (`deleted_at = NOW()`), never physically deleted
- If a previously deleted item reappears at the same path, it is automatically restored (`deleted_at = NULL`)
- `progress.step_id` and `exercise_attempts.step_id` use `ON DELETE SET NULL` (step_id is nullable) — learner progress is **never** lost, even if a step is eventually purged
- Migration: `007_content_hash_sync.up.sql`

**Content Update Rules:**
- Steps are matched by their file path (relative to repo root) — renaming a file creates a new step
- Updated steps overwrite content and exercise data, but **never delete learner progress**
- Deleted steps (files removed from repo): the `steps` record is soft-deleted (flagged, not removed), preserving associated progress
- Deleted modules and learning paths are likewise soft-deleted (`deleted_at` column)
- New steps are inserted at the position determined by their numeric prefix
- Module positions are **not** enforced by a unique database constraint — the syncer manages ordering based on the numeric prefix convention

**Slug Columns:**

- `slug TEXT NOT NULL` column on `learning_paths`, `modules`, and `steps`
- Auto-generated from titles during sync: lowercase, spaces replaced with hyphens, special characters removed. Duplicate slugs within the same scope are deduplicated with `-2`, `-3`, etc. suffixes
- Unique indexes: `idx_learning_paths_slug` (globally unique), `idx_modules_slug` (unique per `learning_path_id`), `idx_steps_slug` (unique per `module_id`)

**Edge Cases:**
- Webhook storms (multiple pushes in quick succession): debounced by checking `sync_status = syncing`
- Concurrent syncs on different repos: allowed (independent operations)
- Git clone failure (network, auth): `sync_status = error`, admin notified
- Invalid YAML front matter: sync rolls back, error message includes file path and parse error
- Empty module directory (no step files): module is synced with zero steps (allowed)
- Binary files in `codebase/`: silently skipped (only text files are stored)

### 2.3 Content Parsing Rules

The Content Syncer must parse structured Markdown conventions to extract exercise data. This section defines the exact parsing rules for each step type.

#### 2.3.1 Front Matter Parsing

All step files begin with YAML front matter delimited by `---`:

```
---
title: "Step Title"
type: lesson | quiz | terminal-exercise | code-exercise
estimated_duration: "15m"
# Additional fields depending on type
---
```

**Required fields:** `title`, `type`
**Optional fields:** `estimated_duration`, plus type-specific fields (see below)

If `type` is missing or invalid, the step is rejected and the sync fails.

#### 2.3.2 Lesson Parsing

No exercise data to extract. The Markdown body is stored in `content_md` as-is.

`exercise_data`: `null`

#### 2.3.3 Quiz Parsing

**Input:** Markdown body with `## [multiple-choice]` and `## [short-answer]` headings.

**Parsing rules:**

1. Split the body on `## ` headings
2. For each heading:
   - Extract the type tag from brackets: `[multiple-choice]` or `[short-answer]`
   - The rest of the heading is the **question text**
3. For `[multiple-choice]`:
   - Find all lines matching `- [x] ` or `- [ ] `
   - `[x]` = correct answer, `[ ]` = incorrect answer
   - If multiple `[x]`, the question is multi-select; otherwise single-select
   - Lines not matching checkbox syntax are ignored (treated as context text)
4. For `[short-answer]`:
   - Find the first indented code block (4 spaces or 1 tab) — this is the **expected answer pattern** (regex)
   - If no indented code block is found, the step is rejected
5. For both types:
   - Find blockquote lines (`> `) after the choices — this is the **explanation**
   - The explanation is shown to the learner after submission

**Output (`exercise_data` JSONB):**

```json
{
  "questions": [
    {
      "type": "multiple-choice",
      "text": "What is the smallest deployable unit in Kubernetes?",
      "choices": [
        { "text": "Container", "correct": false },
        { "text": "Pod", "correct": true },
        { "text": "Deployment", "correct": false }
      ],
      "multi_select": false,
      "explanation": "A Pod is the smallest deployable unit in Kubernetes."
    },
    {
      "type": "short-answer",
      "text": "Which command lists all running pods?",
      "pattern": "kubectl get pods",
      "explanation": "`kubectl get pods` lists all pods in the current namespace."
    }
  ]
}
```

#### 2.3.4 Terminal Exercise Parsing

**Input:** Markdown body with `## Step N` headings.

**Parsing rules:**

1. Split the body on `## Step ` headings (case-insensitive number matching)
2. Content before the first `## Step` heading is the **exercise introduction** (context text)
3. For each step:
   - Text before the first code block or checkbox is the **context** for that step
   - Find the `` ```console `` code block — this is the **prompt** (decorative, displayed in the terminal UI)
   - Find all lines matching `- [x] ` or `- [ ] ` — these are command proposals:
     - The command is the text inside the **first backtick pair** on the line
     - Text after ` — ` (em dash) is the **explanation**
     - `[x]` = correct command, `[ ]` = incorrect command
   - Find the `` ```output `` code block — this is the **simulated output** displayed after the correct choice
   - Exactly **one** `[x]` command is required per step (error if zero or more than one)

**Output (`exercise_data` JSONB):**

```json
{
  "introduction": "You are logged into a fresh Ubuntu 22.04 server...",
  "steps": [
    {
      "context": "You need to install the container runtime first.",
      "prompt": "$ ▌",
      "proposals": [
        { "command": "apt install docker.io", "correct": false, "explanation": "Docker works but kubeadm recommends containerd..." },
        { "command": "apt install containerd", "correct": true, "explanation": "Containerd is the recommended container runtime..." },
        { "command": "snap install microk8s", "correct": false, "explanation": "MicroK8s is a different Kubernetes distribution..." }
      ],
      "output": "Reading package lists... Done\nSetting up containerd (1.6.20-0ubuntu1) ..."
    }
  ]
}
```

#### 2.3.5 Code Exercise Parsing

**Additional front matter fields:**
- `mode` (required): `identify-and-fix`, `choose-the-fix`, or `identify-then-fix`
- `target` (required for modes A/C): `{ file: "path/to/file.go", lines: [12, 13] }`

**Input:** Markdown body with a `## Patches` section containing `### [x]` or `### [ ]` headings.

**Parsing rules:**

1. Content before `## Patches` is the **problem description** (free Markdown)
2. Split the `## Patches` section on `### ` headings
3. For each patch heading:
   - Extract correctness from brackets: `[x]` = correct, `[ ]` = incorrect
   - The rest of the heading is the **patch label**
   - Text between the heading and the `diff` code block is the **explanation**
   - Find the `` ```diff `` code block — this is the **unified diff** for the patch
   - Exactly **one** `[x]` patch is required per exercise (error if zero or more than one)

**Output (`exercise_data` JSONB):**

```json
{
  "mode": "identify-and-fix",
  "target": { "file": "pkg/handler.go", "lines": [12, 13] },
  "description": "The deployment is failing its liveness probe...",
  "patches": [
    {
      "label": "Fix the status code and response body",
      "correct": true,
      "explanation": "The health endpoint should return 200 OK, not 500.",
      "diff": "--- a/pkg/handler.go\n+++ b/pkg/handler.go\n@@ -11,3 +11,3 @@..."
    },
    {
      "label": "Return 404 Not Found",
      "correct": false,
      "explanation": "404 Not Found is not appropriate for a health check endpoint.",
      "diff": "--- a/pkg/handler.go\n+++ b/pkg/handler.go\n..."
    }
  ]
}
```

**Codebase files:**
- The `codebase/` directory alongside `instructions.md` is read recursively
- Each file's relative path and content are stored in `codebase_files`
- Binary files are silently skipped
- The file tree structure is preserved for Monaco Editor rendering

### 2.4 Markdown Rendering

**Description:** Lesson content and exercise context text is rendered as rich HTML.

**Supported Markdown features:**
- Standard CommonMark (headings, paragraphs, lists, links, images, tables)
- Fenced code blocks with syntax highlighting (language-specific)
- Mermaid diagrams (rendered in the browser)
- Admonition blocks via `remark-directive` plugin: `:::tip`, `:::warning`, `:::danger`, `:::info`, `:::note` (Docusaurus/MkDocs convention). Rendered as styled callout components (a React component maps each directive to a visual block with icon and color)
- Embedded images, videos, and audio (relative paths referencing the `assets/` directory)
- HTML inline elements (limited — no scripts)

**Rendering rules:**
- Images with relative paths are automatically rewritten to `/api/assets/{hash}` during sync
- Video files (`.mp4`, `.webm`, `.ogg`, `.mov`) referenced with `![alt](./assets/file.mp4)` are rendered as `<video>` players
- Audio files (`.mp3`, `.wav`, `.ogg`, `.flac`, `.aac`) referenced with `![alt](./assets/file.mp3)` are rendered as `<audio>` players
- Code blocks with a language identifier receive syntax highlighting
- Mermaid code blocks (`` ```mermaid ``) are rendered as SVG diagrams
- Raw HTML is sanitized (script tags, iframes, event handlers are stripped)

> **Not in v1:** LaTeX math rendering (KaTeX/MathJax). DevOps content rarely needs math formulas. Adding KaTeX later is trivial (one remark plugin, no content format change).

### 2.5 Asset Pipeline

**Description:** Binary assets (images, videos, PDFs) from content repositories are uploaded to the asset store and served via the API.

**Storage backends:**
- **Filesystem** (default): Assets stored in `{data_dir}/{hash[0:2]}/{hash}` with companion `.meta` JSON file
- **S3** (production): Assets stored in `{bucket}/{prefix}/{hash}` using any S3-compatible service (AWS S3, MinIO, etc.)

**Database tables:**

```sql
-- Deduplicated asset storage (one row per unique file)
content_assets (
    id UUID PK,
    content_hash TEXT UNIQUE NOT NULL,  -- SHA-256 hex
    content_type TEXT NOT NULL,         -- MIME type
    file_name TEXT NOT NULL,            -- original filename
    size_bytes BIGINT NOT NULL,
    storage_backend TEXT NOT NULL,      -- 'filesystem' or 's3'
    created_at TIMESTAMPTZ
)

-- N:N relationship between steps and assets
step_assets (
    step_id UUID FK → steps(id) ON DELETE CASCADE,
    asset_id UUID FK → content_assets(id),
    original_path TEXT NOT NULL,        -- e.g. 'assets/diagram.png'
    PK (step_id, asset_id)
)
```

**Sync process:**
1. For each step, scan the `assets/` directory relative to the step's parent directory
2. Compute SHA-256 hash for each file
3. If the hash doesn't exist in `content_assets` → upload to the asset store
4. Insert/update `step_assets` relationship
5. Rewrite relative URLs in `content_md`: `./assets/img.png` → `/api/assets/{hash}`
6. Files exceeding `max_file_size` (default 50 MB) are skipped with a warning

**Deduplication:** The same file used across multiple steps is stored only once (keyed by content hash).

**Slug-based API routing:** API endpoints that reference path or step IDs accept both UUIDs and slugs. When a UUID is provided, the API returns a 301 redirect to the slug-based URL for backward compatibility.

**API endpoint:**

```
GET /api/assets/{hash}
```

- **Public** (no authentication required)
- Returns the asset with its original `Content-Type`
- Cache headers: `Cache-Control: public, max-age=31536000, immutable`
- Hash validation: must match `^[a-f0-9]{64}$`
- Returns 404 for unknown hashes

---

## 3. Learning Experience

### 3.1 Learning Path Catalog

**Description:** Learners browse and discover available learning paths.

**Behavior:**

1. The catalog page lists all available learning paths
2. Each learning path card displays:
   - Title, description, icon
   - Tags (filterable)
   - Estimated duration
   - Prerequisites (other learning paths)
   - Competencies provided (aggregated from modules)
   - Learner's progress (if enrolled): percentage bar, current step
3. Filtering: by tag, by enrollment status (enrolled, not enrolled, completed), **by competency** (multi-select)
4. Sorting: alphabetical, by progress, by most recently accessed, **by competency path** (topological order — **default**)
5. Search: full-text search on title, description, tags
6. **View mode toggle:** A `Segmented` control (Ant Design) lets the user switch between:
   - **Grid view** (AppstoreOutlined icon) — the default card layout described below
   - **DAG view** (ApartmentOutlined icon) — a directed acyclic graph visualization of learning paths and their dependencies
   - The selected view mode is persisted in `localStorage` (`catalog-view` key)
7. All filters (search, tags, competencies, status, sort) apply to both grid and DAG views

**DAG view behavior:**
- Uses `@xyflow/react` (React Flow) with `dagre` layout (horizontal left-to-right)
- **Toggle** is a `Segmented` control placed next to the page title (not in the filter bar), with labels "Grid" and "Graph"
- **Nodes** represent learning paths with: title, icon, progress border color (green = completed, orange = in_progress, gray = not_started), and tag badges
- **Edges** use `smoothstep` type (orthogonal routing with rounded corners, `borderRadius: 12`). Three dependency types are combined:
  - `auto`: computed from `path.prerequisites` matching other paths' `competencies_provided` — rendered in neutral gray (`#8c8c8c`)
  - `manual`: administrator-created via the Admin UI (see §8.4) — rendered in purple (`#b37feb`), animated
  - `yaml`: defined in `phoebus.yaml` via the `depends_on` field (list of path slugs) — rendered in purple (`#b37feb`), animated
- Edge labels show the matching competency names (deduplicated)
- **Dark mode**: all DAG elements (node backgrounds, edge labels, canvas, grid, MiniMap) adapt to the current theme via `useTheme()` context
- **Popover on click**: clicking a node shows a popover with description, competencies provided, prerequisites, and a "View Path" button navigating to `/paths/:pathSlug`
- **MiniMap**: color-coded by progress status (green/orange/gray)
- **Controls**: zoom in/out and pan provided by React Flow's built-in Controls component

**Competency path sorting (topological order):**
Learning paths are ordered based on their competency dependencies:
1. Paths with no prerequisites appear first
2. Then paths whose prerequisites are provided by earlier paths in the list
3. Cycles (if any) are broken arbitrarily — paths involved in cycles are grouped together
4. Within the same dependency level, paths are sorted alphabetically

**Competency filter:**
- A multi-select dropdown lists all competencies found across all modules
- Selecting a competency filters to show only paths whose modules provide that competency
- Multiple competencies selected = OR (show paths providing any of the selected competencies)
- Competency filter can be pre-set via URL query parameter: `/catalog?competencies=docker-basics,k8s-pods` — used by the prerequisite enforcement popup to redirect learners

**Edge Cases:**
- Learning path with unmet prerequisites: displayed with a warning badge; clicking "Start Learning" triggers a prerequisite confirmation popup (see §3.1.1)
- Empty learning path (no modules after sync): displayed with a "coming soon" indicator
- Competency filter with no results: display "No learning paths match the selected competencies"

### 3.1.1 Prerequisite Enforcement Popup

**Description:** When a learner starts a learning path with unmet prerequisites, a confirmation modal warns them.

**Trigger:** The popup is shown when:
- The learner clicks "Start Learning" or "Continue Learning" on a path overview page
- OR the learner navigates directly to a step of a path
- AND the path has prerequisites that the learner has not yet acquired

**Prerequisite resolution:** A prerequisite competency is considered "met" when the learner has completed (100% progress) **any** learning path whose modules declare that competency.

**Popup content:**
```
⚠ Prerequisites Not Met

This learning path requires knowledge of:
  • docker-basics (provided by "Docker Fundamentals") — ✅ Completed
  • k8s-pods (provided by "Kubernetes Basics") — ❌ Not completed
  • linux-cli (provided by "Linux Fundamentals") — ❌ Not completed

You may continue, but the content assumes familiarity with these topics.

[Browse Prerequisite Paths]    [Continue Anyway]
```

**Behavior:**
- **"Continue Anyway"** — dismisses the popup; the learner proceeds normally. The dismissal is stored in the browser session (sessionStorage) so the popup is not shown again for this path during the same session.
- **"Browse Prerequisite Paths"** — navigates to `/catalog?competencies=k8s-pods,linux-cli` (only unmet competencies), where the catalog is pre-filtered to show paths that provide the missing competencies.
- Each prerequisite line shows which learning path provides it and whether it's already completed.
- If all prerequisites are met, the popup is never shown.

### 3.2 Learning Path Navigation

**Description:** Learners navigate through the sequential structure of a learning path.

**Behavior:**

1. Sidebar displays the learning path structure:
   - Modules (expandable)
   - Steps within each module (with type icon and completion indicator)
2. Learner clicks on a step to navigate to it
3. Steps are displayed sequentially; navigation is **free** (no enforced order)
   - Rationale: self-assessment philosophy — learners choose their own path
4. Current step is highlighted in the sidebar
5. "Next" and "Previous" buttons navigate to adjacent steps (for exercises)
6. Lesson steps: a unified "Complete & Continue" button is shown at the bottom (replaces the separate "Mark as Completed" and "Next Step" buttons). The button is disabled until the learner has scrolled through at least 75% of the lesson content. When the 75% scroll threshold is reached, the step is automatically marked as `in_progress`. Clicking the button marks the step as `completed` and navigates to the next step. On the last step of the path, clicking the button shows a congratulations modal before redirecting to the path overview
7. Exercise steps: completion is automatic upon successful exercise completion. Exercises are marked as `in_progress` when the learner first opens them (unchanged behavior)

**Completion Logic:**
- A step is completed when:
  - **Lesson**: learner clicks the "Complete & Continue" button (enabled after 75% scroll)
  - **Quiz**: learner submits all answers (regardless of correctness — it's self-assessment)
  - **Terminal Exercise**: learner correctly answers all steps in sequence
  - **Code Exercise**: learner correctly identifies lines (modes A/C) and selects the correct patch
- A module is completed when all its steps are completed
- A learning path is completed when all its modules are completed

### 3.3 Progress Tracking

**Description:** Track and display individual learner progress.

**Behavior:**

1. Each step has a progress status: `not_started`, `in_progress`, `completed`. `in_progress` is displayed as a distinct third state in all analytics (icon: ⏳ ClockCircleOutlined)
2. `in_progress` is set when the learner first opens a quiz or exercise step. For lessons, `in_progress` is set only after the learner scrolls through at least 75% of the content (not on page open)
3. `completed` is set according to the completion logic above
4. Progress is stored per learner per step in the `progress` table
5. Module progress = (completed steps / total steps) × 100%
6. Learning path progress = (completed modules / total modules) × 100%

**Personal Dashboard:**
- Lists enrolled learning paths with progress bars
- Shows recently accessed steps (quick resume)
- Shows competencies acquired (linked from modules)
- Shows total steps completed, total time spent (estimated from step durations)

### 3.4 Exercise Reset

**Description:** Learners can reset any exercise to start over.

**Detailed Behavior:**

1. A "Reset" button is available on every exercise step
2. Clicking "Reset" triggers a confirmation dialog
3. Upon confirmation:
   - `progress.status` is set back to `in_progress`
   - `progress.completed_at` is cleared
   - **Previous `exercise_attempts` are preserved** (historical data is never deleted)
   - The exercise UI is reset to its initial state:
     - Quiz: all answers cleared
     - Terminal Exercise: back to Step 1
     - Code Exercise: line selection cleared, patch selection cleared
4. Reset is unlimited (no cap on number of resets)
5. Reset does not affect module or learning path completion status (they are recalculated from step statuses)

---

## 4. Terminal Exercises

### 4.1 Rendering

**Description:** Terminal exercises are displayed in a terminal-like UI component.

**UI Layout:**

```
  You are logged into a fresh Ubuntu 22.04 server.
  You need to install the container runtime first.

  [Step 1/3]

  ┌─── ● ● ● ──────────── Terminal Exercise ───┐
  │                                             │
  │  ~ › apt update                             │
  │  Hit:1 http://archive.ubuntu.com/ubuntu ... │
  │                                             │
  │  ~ › ▌                                      │
  │                                             │
  │  ─── Select a command: ─────────────────    │
  │                                             │
  │    $ apt install docker.io                  │
  │    $ apt install containerd                 │
  │    $ snap install microk8s                  │
  │                                             │
  └─────────────────────────────────────────────┘
```

**UI Elements:**
- Context area: introduction text + per-step context text (Markdown-rendered), displayed **above** the terminal
- Step counter: badge showing current step (Step N/M), displayed above the terminal
- **Immersive terminal design** (no Card wrapper): full dark theme with a fire orange palette (`#ff9a6c`, `#ffb899`) on a neutral gray background (`#1c1c1c`), monospace font
  - Numbered badges `[1]`, `[2]` displayed before each command choice
  - **Arrow key navigation** (↑↓) to move between commands + Enter to validate the selection
  - Auto-focus on mount, no visible focus outline
  - Auto-select first available (non-eliminated) command
  - Command history: previous steps' commands and outputs scroll upward
  - Active prompt line: `~ ›` prefix with blinking cursor `▌`; when a command is selected, it appears on the prompt line
  - Feedback: success (green ✓) and error messages (red ✗ with explanation) appear inline in the terminal
  - Incorrect commands are struck-through and grayed out
- Validation: selecting a suggestion (click or arrow keys) fills the prompt; pressing Enter submits the command (no separate Submit button)

### 4.2 Step-by-Step Flow

**Behavior:**

1. Exercise loads with Step 1 displayed; context text appears above the terminal
2. Learner clicks a command suggestion inside the terminal; the command appears on the active prompt line
3. Learner presses Enter (keyboard) or the suggestion is submitted on click:
   - **Correct**: 
     - A green "Correct!" message appears inline in the terminal
     - After a short delay, the command and its simulated output are added to the terminal history
     - The exercise advances to the next step; the terminal scrolls down
     - Previous commands and outputs remain visible (scrollable history)
   - **Incorrect**:
     - The explanation for the incorrect choice is displayed inline in the terminal (red)
     - The incorrect suggestion is struck-through, grayed out, and disabled
     - The prompt is cleared; the learner selects another command
     - Number of retries is unlimited
4. When the last step is answered correctly:
   - The exercise is marked as completed
   - A summary is shown (all steps with the correct commands and outputs)
   - The `TerminalExercise` component emits an `onComplete` callback when all steps are answered correctly

**Attempt Recording:**

Each attempt (correct or incorrect) is recorded as an `exercise_attempt`:

```json
{
  "step_number": 1,
  "selected_command": "apt install docker.io",
  "is_correct": false
}
```

A new attempt record is created for every submission. The `answers` JSONB stores which command was selected and which step it applies to. This allows instructors to analyze:
- Which incorrect commands are most commonly selected
- How many attempts each step takes on average
- Which steps are the most difficult

### 4.3 Edge Cases

- Exercise with a single step: behaves the same (just no step progression)
- All choices eliminated (all incorrect selected): theoretically impossible since the correct one is always available
- Content update during an exercise: the learner continues with the version they started; on next load, the updated version is shown
- Browser refresh mid-exercise: on load, the frontend fetches existing `exercise_attempts` and resumes after the last correctly-answered step. No new DB field needed — reuses data already recorded by server-side validation. Progress survives page refresh. If the exercise was already completed, the `progress` record is already `completed`.

---

## 5. Code Exercises

### 5.1 Rendering

**Description:** Code exercises display a read-only code viewer with a problem description and patch proposals.

**UI Layout (Mode A — Identify & Fix):**

```
┌──────────────────────────────────────────────────────────────────┐
│ ● Fix the Health Check Handler                      Phase 1/2   │
├──────────────────────┬───────────────────────────────────────────┤
│ File Tree            │ Editor (read-only)                        │
│                      │                                           │
│ ▾ codebase/          │  1  package handler                       │
│   ├── main.go        │  2                                        │
│   ├── go.mod         │  3  import (                               │
│   └── pkg/           │  4      "net/http"                         │
│       └── handler.go │  5  )                                      │
│            ★         │  6                                        │
│                      │  7  // HandleHealth responds to            │
│                      │  8  // health check requests               │
│                      │  9  func HandleHealth(w http.ResponseWr    │
│                      │ 10      r *http.Request) {                 │
│                      │ 11                                        │
│                      │ 12▸     w.WriteHeader(http.StatusInternalE │
│                      │ 13▸     w.Write([]byte("error"))           │
│                      │ 14  }                                      │
│                      │                                           │
├──────────────────────┴───────────────────────────────────────────┤
│                                                                  │
│  The deployment is failing its liveness probe. The               │
│  HandleHealth function is returning the wrong status code.       │
│                                                                  │
│  Click on the lines you think are problematic.                   │
│                                                                  │
│  Selected lines: 12, 13                    [Validate Selection]  │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

**UI Elements:**
- **File tree** (left panel): navigable file tree from the `codebase/` directory. The target file is highlighted (★)
- **Code viewer** (right panel): Monaco Editor in read-only mode. Line numbers, syntax highlighting. Clickable lines (for modes A/C). Uses `monaco-editor` local bundle (no CDN) via `loader.config({ monaco })` with Vite worker imports for all Monaco workers (editor, json, css, html, ts) and `self.MonacoEnvironment.getWorker()` configuration for CSP compliance. `manualChunks` in `vite.config.ts` splits Monaco into a separate ~3.7 MB chunk
- **Problem description / Feedback panel**: Markdown-rendered text below the code viewer. When feedback is available (after an attempt), the feedback **replaces** the description panel and auto-expands to 220 px
- **Phase indicator**: shows current phase (Phase 1: Identify, Phase 2: Fix) for modes A/C
- **Selected lines display**: shows the lines the learner has clicked on
- **Patch proposals** (Phase 2): displayed as radio buttons. Each patch shows its label and a **DiffEditor preview** (Monaco side-by-side diff view) when selected. The DiffEditor remains visible after exercise completion. Incorrect patches are displayed with disabled radio buttons and strikethrough text
- **Navigation buttons**: Previous / Reset / Next buttons displayed in the completed state

### 5.2 Mode A — Identify & Fix

**Flow:**

1. **Phase 1 — Identify:**
   - Code viewer is displayed with all files from `codebase/`
   - The target file is pre-selected in the file tree (but the learner can browse other files)
   - Learner clicks on lines they believe are problematic
   - Clicking a line toggles its selection (click again to deselect)
   - A "Validate Selection" button submits the selected lines
   - **Validation**: the selected lines are compared against `target.lines` from `exercise_data`
     - **Exact match required**: learner must select exactly the target lines (no more, no fewer)
     - Feedback is **progressive**: partial matches receive guiding hints (e.g., "1/2 lines found", "right area, refine your selection") rather than a binary correct/incorrect
     - **Correct**: visual confirmation (green highlight on target lines), advance to Phase 2
     - **Incorrect**: progressive feedback helps the learner converge on the right lines, allow retry
   
2. **Phase 2 — Fix:**
   - The target lines remain highlighted during Phase 2 as a visual reminder, helping the learner verify that the selected patch applies to the correct location
   - Patch proposals are displayed below the code viewer
   - Each patch shows: label, diff block (syntax-highlighted unified diff)
   - Learner selects one patch via radio buttons
   - On submit:
     - **Correct**: explanation is shown (green), exercise is marked as completed
     - **Incorrect**: explanation for the selected patch is shown (red), learner can retry

### 5.3 Mode B — Choose the Fix

**Flow:**

1. Single phase — no line identification
2. Code viewer is displayed (same as Mode A)
3. Patch proposals are displayed immediately below the code viewer
4. Learner reviews each diff and selects the correct one
5. On submit:
   - **Correct**: explanation shown, exercise completed
   - **Incorrect**: explanation for selected patch shown, allow retry

**Note:** In Mode B, `target` is not required in front matter (no line identification step).

### 5.4 Mode C — Identify, then Fix

Identical to Mode A in behavior, but the two phases are more explicitly separated in the UI:
- Phase 1 only shows the code viewer and line selection (no patches visible)
- Phase 2 only shows the patches (the code viewer is still visible for reference but line selection is locked)

The distinction is primarily in the UI presentation, not in the underlying logic.

### 5.5 Attempt Recording

Each attempt records the learner's action:

**Phase 1 (line identification) attempt:**
```json
{
  "phase": "identify",
  "selected_lines": [10, 11, 12],
  "is_correct": false
}
```

**Phase 2 (patch selection) attempt:**
```json
{
  "phase": "fix",
  "selected_patch": "Return 404 Not Found",
  "is_correct": false
}
```

### 5.6 Edge Cases

- Large codebase (many files): file tree is scrollable; no limit on number of files
- Target lines span multiple files: **not supported in v1** — target must reference a single file
- Empty `codebase/` directory: exercise is rejected during content sync
- Binary files in `codebase/`: skipped (not shown in the file tree)
- Diff references lines that don't exist: the diff is displayed as-is (the content syncer does not validate diff applicability)

The `CodeExercise` component emits an `onComplete` callback when the learner successfully completes both phases (line identification and patch selection).

---

## 6. Quizzes

### 6.1 Rendering

**Description:** Quizzes display questions one at a time with immediate feedback. A "Question N/M" counter is shown in the header to indicate progress (as shown in the UI layout below).

**UI Layout:**

```
┌──────────────────────────────────────────────────────┐
│ ● Kubernetes Basics                        Q 1/3     │
├──────────────────────────────────────────────────────┤
│                                                      │
│  What is the smallest deployable unit in Kubernetes? │
│                                                      │
│  ☐  Container                                        │
│  ☐  Pod                                              │
│  ☐  Deployment                                       │
│                                                      │
│                          [Submit]                     │
└──────────────────────────────────────────────────────┘
```

### 6.2 Question Rendering

**Multiple-choice (single-select):**
- Questions with exactly one `[x]` answer are rendered as radio buttons
- Learner selects one answer and clicks "Submit"

**Multiple-choice (multi-select):**
- Questions with more than one `[x]` answer are rendered as checkboxes
- A hint is shown: "Select all that apply"
- Learner selects one or more answers and clicks "Submit"

**Short-answer:**
- Rendered as a text input field
- On submit, the learner's answer is validated against the expected pattern (regex match)
- Matching is case-insensitive by default
- Multiple valid answers are supported natively via regex alternatives (e.g., `kubectl get po(ds)?`, `^(kubectl|k) get pods$`). Instructors should document their regex patterns clearly

### 6.3 Submission & Feedback

**Behavior:**

1. Learner answers a question and clicks "Submit"
2. Feedback is shown immediately:
   - **Multiple-choice**: correct answers are highlighted in green, incorrect selections in red
   - **Short-answer**: "Correct" or "Incorrect" with the expected answer shown
3. The explanation blockquote is displayed below the feedback
4. Learner clicks "Next Question" to proceed (the learner cannot go back to a previous question — immediate feedback reveals the correct answer, making re-answering meaningless. Linear progression, consistent with terminal and code exercises)
5. After the last question, a summary is shown:
   - Number of correct answers out of total
   - List of questions with correct/incorrect status
   - Option to review explanations

**Completion logic:**
- A quiz is marked as completed once all questions have been submitted (regardless of correctness)
- Rationale: self-assessment — the learner sees their results and decides if they need to review
- The `Quiz` component emits an `onComplete` callback when all questions have been submitted

### 6.4 Attempt Recording

Each question submission is recorded:

```json
{
  "question_index": 0,
  "type": "multiple-choice",
  "selected": ["Pod"],
  "is_correct": true
}
```

For short-answer:
```json
{
  "question_index": 1,
  "type": "short-answer",
  "answer": "kubectl get pods",
  "is_correct": true
}
```

### 6.5 Edge Cases

- Quiz with a single question: no "Next Question" button, summary is shown immediately
- Short-answer with regex pattern: the syncer validates the regex during content parsing (reject invalid regex)
- Empty quiz (no questions parsed): rejected during content sync
- Multiple-choice with all answers correct: allowed (trivial question, but valid)

---

## 7. Analytics & Progress Tracking

### 7.1 Learner Dashboard

**Description:** Each learner has a personal dashboard showing their progress.

**Content:**

| Section | Data |
|---|---|
| Enrolled Paths | List of learning paths with progress bars (% completed) |
| Current Step | The last step the learner was working on (quick resume link) |
| Competencies | Acquired competencies from completed modules |
| Activity | Recent activity: steps completed, exercises attempted |
| Statistics | Total steps completed, total exercises attempted, total time (estimated) |

### 7.2 Instructor Analytics

**Description:** Instructors see aggregated analytics for all learning paths.

**Visibility:** All instructors can see analytics for all learning paths (no scoping). In a single-tenant deployment (same company), compartmentalization between instructors adds no value. If scoping is needed later, OIDC groups can be leveraged.

**Content:**

| Section | Data |
|---|---|
| Enrollment | Number of learners enrolled per learning path |
| Progress Distribution | Histogram of learner progress (how many at 0-25%, 25-50%, etc.) |
| Completion Rate | Percentage of enrolled learners who completed the full path |
| Step-Level Stats | Per step: completion rate, average attempts (for exercises), most common wrong answers |
| Failure Points | Steps with the lowest completion rate (potential content improvement targets) |
| Common Wrong Answers | For terminal/code/quiz exercises: which incorrect choices are selected most often |

**Computation:** Analytics are computed in real-time via SQL queries on demand. No batch jobs, no cache tables, no stale data. At target scale (200 users, ~30K rows), PostgreSQL aggregates in <10ms. Materialized views can be added later without architecture changes if scale grows.

**Drill-down:**
- Click on a step to see detailed attempt distribution
- Click on a learner to see their individual progress timeline

### 7.3 Manager View (Optional, Could Have)

Managers can view aggregated progress for their team members. Teams are derived from OIDC group claims (mapped via a configurable `group_claim` setting). No manual team management in Phœbus — the enterprise directory (AD/LDAP → OIDC) is the source of truth. Groups are refreshed on each login.

---

## 8. Administration

### 8.1 User Management

**Description:** Administrators manage users and roles.

**Behavior:**

1. Users are created automatically upon first SSO login (OIDC/LDAP)
2. When local auth is enabled, users can self-register via a signup form on the login page (created with role `learner`)
3. Administrators can create local users manually from the Admin > Users view (with a chosen role and temporary password)
4. Default role for new users: `learner`
5. Administrators can change a user's role: `learner`, `instructor`, `admin`
6. Administrators can deactivate users (soft-delete — they cannot log in but their data is preserved)
7. User list displays: name, email, role, last login, number of completed paths
8. **Forced admin users**: Specific usernames can be configured in the `admin.forced_admins` list. These users are always assigned the `admin` role upon account creation or login (OIDC, proxy, or local registration). Their role is locked: it cannot be changed from the admin UI, and the API rejects role modification requests with a 403 error. The role selector is replaced by a locked badge in the user list.

**RBAC Matrix:**

| Action | Learner | Instructor | Admin |
|---|---|---|---|
| Browse catalog | ✅ | ✅ | ✅ |
| Enroll in learning path | ✅ | ✅ | ✅ |
| Complete exercises | ✅ | ✅ | ✅ |
| View own progress | ✅ | ✅ | ✅ |
| View analytics (all learners) | ❌ | ✅ | ✅ |
| View owned repositories | ❌ | ✅ (own) | ✅ |
| Trigger sync on owned repositories | ❌ | ✅ (own) | ✅ |
| View sync logs on owned repositories | ❌ | ✅ (own) | ✅ |
| Register Git repository | ❌ | ❌ | ✅ |
| Manage repository owners | ❌ | ❌ | ✅ |
| Trigger manual sync (any repo) | ❌ | ❌ | ✅ |
| Manage users | ❌ | ❌ | ✅ |
| View platform health | ❌ | ❌ | ✅ |

### 8.2 Repository Management

See section 2.1 (Git Repository Registration). The admin UI provides:
- List of registered repositories with sync status
- Ability to add, edit, delete repositories
- Manual sync trigger
- Webhook URL display and copy
- **Learning path management per repository**: expandable section showing all learning paths with enable/disable toggle

### 8.2.1 Learning Path Enable/Disable

**Description:** Administrators can enable or disable individual learning paths within a repository.

**Behavior:**
- By default, all learning paths are **enabled** when a new repository is added
- Disabled paths disappear completely from the learner catalog and API (`/api/learning-paths`)
- Disabling is **reversible**: re-enabling restores the path with all learner progression preserved
- Sync operations do **not** modify the enabled/disabled status (independent of `deleted_at` lifecycle)

**API:**
- `GET /api/admin/repos/{repoId}/paths` — returns all paths for a repo with `enabled` status, module count, step count
- `PATCH /api/admin/repos/{repoId}/paths/{pathId}` — set `{"enabled": true/false}`

**UI:** Expandable row in the repository table showing a sub-table of learning paths with:
- Title, description, module/step counts
- Toggle switch + status tag (Active / Disabled)

### 8.2.2 Repository Owners

**Description:** Administrators can assign instructors (or admins) as owners of a repository. Owners have read access to their repositories and can trigger syncs and view sync logs, but cannot edit or delete the repository.

**Behavior:**
- Repository ownership is a N:N relationship (multiple instructors per repository, one instructor can own multiple repositories)
- Administrators assign owners from the repository add/edit form via a multi-select dropdown listing users with `instructor` or `admin` roles
- The catalog displays deduplicated owner display names on each learning path card (via the repository association); owners are fetched with `DISTINCT` to avoid duplication when a repository contains multiple learning paths
- The instructor dashboard shows a "My Repositories" section listing owned repositories with clone URL (copyable), sync status, and action buttons (Sync / Sync Logs)
- Instructors can trigger a sync and view sync logs only on repositories they own
- Ownership verification is enforced server-side via middleware on `/api/instructor/repos/{repoId}/*` routes

**API:**
- `GET /api/admin/instructor-users` — list users with instructor/admin role (for owner selection)
- `POST /api/admin/repos` and `PUT /api/admin/repos/{repoId}` — accept `owner_ids` field (array of user IDs)
- `GET /api/admin/repos` and `GET /api/admin/repos/{repoId}` — return `owners` field (array of `{id, username, display_name}`)
- `GET /api/instructor/repos` — list repos owned by the authenticated instructor
- `GET /api/instructor/repos/{repoId}` — repo details (ownership verified)
- `POST /api/instructor/repos/{repoId}/sync` — trigger sync (ownership verified)
- `GET /api/instructor/repos/{repoId}/sync-logs` — sync logs (ownership verified)
- `GET /api/instructor/repos/{repoId}/sync-logs/{jobId}` — sync job details (ownership verified)

**Database:**
- `repository_owners` table: `repo_id UUID FK`, `user_id UUID FK`, `created_at`, composite primary key `(repo_id, user_id)`
- Cascading delete: removing a repository or user automatically removes the ownership associations

### 8.4 Dependency Management

**Description:** Administrators manage manual dependencies between learning paths, complementing auto-detected (competency-based) and YAML-declared dependencies.

**Route:** `/admin/dependencies`

**Menu entry:** "Dependencies" with ApartmentOutlined icon in the admin sidebar.

**Behavior:**

1. The page lists all manual and YAML dependencies with source/target path titles and dependency type
2. Administrators can create a new manual dependency by selecting a source and target learning path
3. Administrators can delete manual dependencies (YAML dependencies are read-only — managed via `phoebus.yaml`)
4. Duplicate dependencies (same source + target) are rejected by UNIQUE constraint

**API:**

- `GET /api/admin/dependencies` — list manual and YAML dependencies with path titles. Returns `[{ id, source_path_id, target_path_id, source_title, target_title, dep_type }]`
- `POST /api/admin/dependencies` — create a manual dependency. Body: `{ "source_path_id": "<uuid>", "target_path_id": "<uuid>" }`. Returns 201 with the created dependency. Returns 409 if duplicate.
- `DELETE /api/admin/dependencies/{depId}` — delete a manual dependency. Returns 404 if not found or if `dep_type` is `yaml`.

### 8.3 Platform Health

**Description:** Administrators monitor platform health.

**Metrics:**
- Application uptime
- Database connection status
- Git sync status per repository (last sync time, errors)
- Active user count (users logged in within the last 24h)
- API response times (p50, p95, p99)
- Disk usage (cloned Git repos)

**Delivery:** Prometheus metrics endpoint (`/metrics`) + optional health dashboard in the admin UI.

---

## 9. Authentication & Authorization

### 9.1 OIDC Authentication

**Flow:**

1. User accesses Phœbus → redirected to OIDC provider login page
2. User authenticates with their corporate credentials
3. OIDC provider redirects back to Phœbus with an authorization code
4. Phœbus exchanges the code for an ID token and access token
5. Phœbus creates or updates the user record (matched by `external_id` or email)
6. A session token (JWT) is issued as an httpOnly cookie (see section 9.3)
7. Subsequent API requests carry the JWT automatically via the cookie — no `Authorization` header needed

**Configuration:**
- OIDC issuer URL
- Client ID and client secret
- Scopes (default: `openid email profile`)
- Claim mapping: which OIDC claim maps to display name, email, external ID

### 9.2 LDAP Authentication

**Flow:**

1. User submits username/password on the Phœbus login page
2. Phœbus performs an LDAP bind with the provided credentials
3. If bind succeeds, Phœbus fetches user attributes (name, email, groups). LDAP group membership is synced on every login (no caching). Login happens once per session (~8h), so one extra LDAP query is negligible. Consistent with OIDC behavior (claims refreshed on auth)
4. User record is created or updated
5. A session token (JWT) is issued

**Configuration:**
- LDAP server URL
- Base DN for user search
- User search filter (e.g., `(uid={username})`)
- Attribute mapping: which LDAP attributes map to display name, email
- Optional: group-to-role mapping (e.g., LDAP group "trainers" → role `instructor`)

### 9.3 Session Management

- JWT tokens with configurable expiration (default: 8 hours)
- Refresh token support for seamless re-authentication
- JWT is stored in an httpOnly cookie with `SameSite=Lax`. Immune to XSS (critical for a SPA rendering user-provided Markdown as HTML). The frontend never touches the token — it is sent automatically by the browser
- CSRF protection via `SameSite=Lax` + JSON-only API (no cross-origin form submission). OWASP recommended
- Logout invalidates the JWT (client-side cookie deletion; server-side blacklist is optional)

### 9.4 Local Authentication (Fallback)

**Description:** A minimal local authentication mechanism for bootstrap and development scenarios.

**Flow:**

1. User submits username/password on the Phœbus login page (same form as LDAP)
2. Phœbus verifies the password against a bcrypt hash stored in the `users` table
3. If valid, a session token (JWT) is issued as an httpOnly cookie (same as OIDC/LDAP)

**Self-Registration (Signup):**

When local auth is enabled, a "Create account" link is displayed below the login form. Clicking it reveals a registration form:

1. User fills in: username, display name, email (optional), password, confirm password
2. `POST /api/auth/register` creates the user with role `learner` and `auth_provider: local`
3. On success, the user is automatically logged in (JWT cookie set) and redirected to `/`

**Constraints:**
- Signup is only available when local auth is enabled (`local_auth.enabled: true`)
- Username must be unique (4–32 chars, alphanumeric + hyphens)
- Password minimum length: 8 characters
- If username already exists → error "Username already taken"

**Admin User Creation:**

Administrators can create local users from the Admin > Users view:

1. Admin clicks "Add User" button
2. Modal form: username, display name, email, role (learner/instructor/admin), password
3. `POST /api/admin/users` creates the user with `auth_provider: local`
4. The created user can log in immediately with the provided credentials

**Scope:**
- Bcrypt-hashed passwords
- `/api/auth/login` endpoint
- `/api/auth/register` endpoint (self-registration)
- `/api/admin/users` endpoint (admin creation with `POST`)
- No password reset functionality
- No password complexity policy (beyond minimum 8 chars)
- Essential for bootstrap (first admin account), getting started (`docker compose up` → immediate login), and development

**Configuration:**
- Enabled/disabled via `local_auth.enabled` (default: `true`)
- Disabled in production via `local_auth.enabled: false`
- When disabled, `/api/auth/register` returns `403 Forbidden` and signup UI is hidden

### 9.5 Reverse Proxy Authentication

**Description:** Transparent authentication via HTTP headers injected by a reverse proxy (OAuth2 Proxy, Authelia, Traefik Forward Auth, etc.). Phœbus trusts the headers and provisions users automatically.

**Flow:**

1. User accesses Phœbus through a reverse proxy that handles SSO authentication
2. The reverse proxy injects identity headers (e.g., `X-Remote-User`, `X-Remote-Groups`)
3. Phœbus middleware detects the configured user header and provisions the user:
   - If no user exists with `auth_provider='proxy'` and matching username → create one
   - If user already exists → update display name, email, and role from groups
4. A session token (JWT) is issued as an httpOnly cookie (same as other providers)
5. Subsequent requests with a valid JWT cookie skip the upsert (performance optimization)

**Configuration:**

| Key | Default | Description |
|---|---|---|
| `proxy_auth.enabled` | `false` | Enable reverse proxy authentication |
| `proxy_auth.header_user` | `X-Remote-User` | Header containing the username (required) |
| `proxy_auth.header_groups` | `X-Remote-Groups` | Header containing comma-separated group names |
| `proxy_auth.header_email` | *(empty)* | Header containing the user's email |
| `proxy_auth.header_display_name` | *(empty)* | Header containing the user's display name |
| `proxy_auth.default_role` | `learner` | Default role when no group matches |
| `proxy_auth.group_to_role` | *(empty)* | Map of group name → role (`admin`, `instructor`, `learner`) |

**Group-to-role resolution:** When multiple groups match, the highest-privilege role wins (admin > instructor > learner). This is consistent with LDAP group resolution (§9.2).

**Coexistence:** Proxy auth works alongside other authentication methods. When both proxy headers and other providers are enabled, the proxy middleware runs first. If no proxy header is present, the request falls through to standard JWT cookie authentication.

**Frontend behavior:** When proxy auth is enabled and is the only provider, the login page displays an informational message ("SSO Authentication") instead of a login form. If the user hits `/login`, the frontend tries `GET /api/me` to detect an existing session and auto-redirects to `/` if authenticated.

### 9.6 Content Security

**Markdown XSS Prevention:**

The Markdown rendering pipeline includes `rehype-sanitize` (inserted after `rehypeRaw`) with a strict schema:

- **Allowed classes:** hljs highlight classes, admonition classes, Mermaid classes
- **Blocked elements:** `<script>`, `<style>`, `<iframe>`, `<object>`, `<embed>`, `<form>`, `<textarea>`
- **Protocol whitelist:** `href` allows `http`, `https`, `mailto` only; `src` allows `http`, `https` only — blocks `file://`, `javascript:`, and `data:` URIs in links

**Mermaid SVG Sanitization:**

- Mermaid output is sanitized with **DOMPurify** using `USE_PROFILES: { svg: true }` before DOM injection
- Mermaid is configured with `securityLevel: 'strict'`

**Content Security Policy (CSP) Headers:**

A backend middleware sets the following CSP headers on all responses:

| Directive | Value | Rationale |
|---|---|---|
| `default-src` | `'self'` | Only same-origin resources by default |
| `script-src` | `'self' blob:` | `blob:` required for Monaco Editor web workers |
| `worker-src` | `'self' blob:` | Same — Monaco workers loaded as blobs |
| `style-src` | `'self' 'unsafe-inline'` | Ant Design injects inline styles |
| `img-src` | `'self' data:` | Data URIs for small inline images |
| `font-src` | `'self' data:` | Data URIs for embedded fonts |
| `connect-src` | `'self'` | API calls to same origin only |
| `frame-ancestors` | `'none'` | Prevent framing (clickjacking) |

**Additional Security Headers:**

- `X-Frame-Options: DENY`
- `X-Content-Type-Options: nosniff`
- `Referrer-Policy: strict-origin-when-cross-origin`
- `X-XSS-Protection: 0` (disabled — CSP supersedes it; the legacy header can cause issues)

---

## 10. SPA Views

This section inventories every view (page) of the Phœbus single-page application. For each view it defines the route, required role, layout, data loaded, API calls, navigation targets, and detailed interactions.

### 10.1 Global Layout

All authenticated views share a common shell layout:

```
┌─────────────────────────────────────────────────────────────────────┐
│  🔥 Phœbus           Catalog   Dashboard     [🌙] [User ▾] [Logout]│
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│                        <Page Content>                               │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

**Header (global, always visible):**

| Element | Behavior |
|---|---|
| Logo + name | Click → navigates to Dashboard (`/`). **Fixed branding:** dark background (`#001529`), fire icon (`#ff7a45`), white text — does not change with theme |
| Catalog link | Navigates to `/catalog` |
| Dashboard link | Navigates to `/` |
| Analytics link | Visible only for `instructor` and `admin` roles. Navigates to `/analytics` |
| Admin link | Visible only for `admin` role. Opens a dropdown: Repositories, Users, Health |
| Theme toggle 🌙/☀️ | Switches between light and dark mode. Icon: moon (→ dark) / sun (→ light). White icon, same branding as header |
| User menu | Dropdown: display name, role badge, Logout. White icon, same branding as header |

**Theme system:**

| Aspect | Behavior |
|---|---|
| Default | Follows system `prefers-color-scheme` media query |
| Toggle | User clicks moon/sun icon in header to switch mode |
| Persistence | User preference stored in `localStorage` (`phoebus-theme`). Overrides system detection once set |
| System follow | If no explicit preference stored, theme tracks system changes in real-time |
| Implementation | CSS variables on `:root` (light) and `[data-theme="dark"]` (dark). Ant Design `ConfigProvider` with `darkAlgorithm`/`defaultAlgorithm`. All UI components consume CSS variables — no hardcoded colors |
| Header exception | The header bar keeps its dark branding identity (`#001529` background) regardless of the active theme |

**Unauthenticated views** (Login) render without the header — they use a centered, minimal layout.

**Learning Path views** (`/paths/:pathSlug/steps/:stepSlug`) replace the global header with a learning-specific header that includes a sidebar (see section 10.7).

### 10.2 Route Table

| Route | View | Min. Role | Layout | Section |
|---|---|---|---|---|
| `/login` | Login | — (public) | Centered | 10.3 |
| `/` | Dashboard | `learner` | Global shell | 10.4 |
| `/catalog` | Catalog | `learner` | Global shell | 10.5 |
| `/paths/:pathSlug` | Learning Path Overview | `learner` | Global shell | 10.6 |
| `/paths/:pathSlug/steps/:stepSlug` | Step View | `learner` | Learning layout | 10.7 |
| `/analytics` | Analytics Dashboard | `instructor` | Global shell | 10.8 |
| `/analytics/paths/:pathSlug` | Learning Path Analytics | `instructor` | Global shell | 10.9 |
| `/analytics/learners/:learnerId` | Learner Detail | `instructor` | Global shell | 10.10 |
| `/admin/repositories` | Repository Management | `admin` | Global shell | 10.11 |
| `/admin/repositories/new` | Add Repository | `admin` | Global shell | 10.12 |
| `/admin/repositories/:repoId/edit` | Edit Repository | `admin` | Global shell | 10.12 |
| `/admin/repositories/:repoId/sync-logs` | Sync Logs | `admin` | Global shell | 10.13 |
| `/admin/users` | User Management | `admin` | Global shell | 10.14 |
| `/admin/health` | Platform Health | `admin` | Global shell | 10.15 |

**Access control:** The frontend checks the user's role (from the JWT payload decoded client-side — the httpOnly cookie contains the JWT but a non-sensitive role claim is also available via a `/api/me` call on page load) and hides inaccessible navigation links. The backend enforces RBAC on every API call — a direct URL access to an unauthorized route returns `403 Forbidden`.

**Redirect rules:**
- Unauthenticated user accessing any route → redirect to `/login?redirect={originalUrl}`
- Authenticated user accessing `/login` → redirect to `/`
- User accessing a route above their role → redirect to `/` with a toast notification ("Access denied")

### 10.3 Login (`/login`)

**Purpose:** Authenticate the user via OIDC, LDAP, or local credentials. Optionally allow self-registration.

**Layout:** Centered card on a neutral background, no global header.

```
┌─────────────────────────────────────┐
│           🔥 Phœbus                │
│                                     │
│  ┌─────────────────────────────┐    │
│  │  Sign in with SSO           │    │  ← OIDC button (if configured)
│  └─────────────────────────────┘    │
│                                     │
│  ──────── or sign in below ──────── │  ← Divider (if LDAP or local auth)
│                                     │
│  Username  [___________________]    │
│  Password  [___________________]    │
│                                     │
│        [Sign In]                    │
│                                     │
│  Don't have an account? Create one  │  ← Link (if local auth enabled)
│                                     │
│  ⚠ Invalid credentials             │  ← Error (conditional)
└─────────────────────────────────────┘
```

**Registration form** (toggled via "Create one" link, replaces login form):

```
┌─────────────────────────────────────┐
│           🔥 Phœbus                │
│                                     │
│  Create your account                │
│                                     │
│  Username      [___________________]│
│  Display Name  [___________________]│
│  Email         [___________________]│  ← Optional
│  Password      [___________________]│
│  Confirm       [___________________]│
│                                     │
│        [Create Account]             │
│                                     │
│  Already have an account? Sign in   │  ← Link back to login form
│                                     │
│  ⚠ Username already taken           │  ← Error (conditional)
└─────────────────────────────────────┘
```

**Elements:**

| Element | Condition | Behavior |
|---|---|---|
| SSO button | OIDC is configured | Redirects to OIDC provider (external redirect via `GET /api/auth/oidc/redirect`) |
| Username/password form | LDAP or local auth enabled | `POST /api/auth/login` with credentials |
| "Create account" link | Local auth enabled | Toggles to registration form |
| Registration form | Local auth enabled + user clicked link | `POST /api/auth/register` with `{ username, display_name, email, password }` |
| Error message | On failed login or registration | "Invalid credentials" or "Username already taken" (no distinction between wrong user/wrong password for security) |

**API Calls:**
- `POST /api/auth/login` — LDAP/local authentication (sets httpOnly cookie on success)
- `POST /api/auth/register` — Local self-registration (creates user with role `learner`, sets httpOnly cookie on success)
- `GET /api/auth/oidc/redirect` — returns OIDC provider URL for browser redirect

**Navigation:**
- On successful login → redirect to `/` (or to the URL stored in `?redirect=` query param)

**Conditional behavior:**
- If OIDC is the only configured provider (LDAP + local auth disabled) → auto-redirect to OIDC provider (skip login page entirely)
- If only local auth is enabled → show only username/password form (no SSO button, no divider)

### 10.4 Dashboard (`/`)

**Purpose:** Personal landing page showing the learner's progress and activity.

**Required role:** `learner` (all authenticated users)

**Layout:** Global shell. Single-column responsive content.

```
┌──────────────────────────────────────────────────────────────────────┐
│ Header                                                               │
├──────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Welcome back, François                                              │
│                                                                      │
│  ┌─ Continue Learning ────────────────────────────────────────────┐  │
│  │ ● Kubernetes Fundamentals — Step: ConfigMap & Secrets   [→]   │  │
│  └────────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  ┌─ My Learning Paths ───────────────────────────────────────────┐  │
│  │                                                                │  │
│  │  Kubernetes Fundamentals          ████████████░░ 72%    [→]   │  │
│  │  GitOps with ArgoCD               ███░░░░░░░░░░░ 20%    [→]   │  │
│  │  Terraform Advanced               ░░░░░░░░░░░░░░  0%    [→]   │  │
│  │                                                                │  │
│  └────────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  ┌─ Competencies ─────────┐  ┌─ Statistics ──────────────────────┐  │
│  │ ✅ Container Basics    │  │  Steps completed:    42            │  │
│  │ ✅ kubectl CLI         │  │  Exercises attempted: 28            │  │
│  │ ⬜ Helm Charts         │  │  Est. time spent:    12h30         │  │
│  └────────────────────────┘  └────────────────────────────────────┘  │
│                                                                      │
│  ┌─ Recent Activity ─────────────────────────────────────────────┐  │
│  │ Today     Completed "Pod Lifecycle" quiz (3/3 correct)        │  │
│  │ Yesterday Completed terminal exercise "Deploy with kubectl"   │  │
│  │ 2 days    Started module "Services & Networking"              │  │
│  └────────────────────────────────────────────────────────────────┘  │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

**Sections:**

| Section | Data Source | Behavior |
|---|---|---|
| Continue Learning | Last accessed step (`progress` table, most recent `updated_at` where status = `in_progress`) | Click → `/paths/:pathSlug/steps/:stepSlug`. Hidden if no in-progress step |
| My Learning Paths | `progress` aggregated by learning path | Progress bars. Click → `/paths/:pathSlug`. Shows only enrolled paths (at least one step accessed) |
| Competencies | `modules.competencies` for completed modules | ✅ for acquired, ⬜ for pending |
| Statistics | Aggregated from `progress` + `exercise_attempts` | Total steps completed, total exercises attempted, estimated time (sum of `estimated_duration` for completed steps) |
| Recent Activity | `progress` + `exercise_attempts` ordered by timestamp | Last 10 activities. Each links to the relevant step |

**API Calls:**
- `GET /api/me/dashboard` — returns all dashboard data in a single call (enrolled paths with progress, last step, competencies, stats, recent activity)

**Navigation targets:**
- Continue Learning card → `/paths/:pathSlug/steps/:stepSlug`
- Learning path row → `/paths/:pathSlug`
- Activity item → `/paths/:pathSlug/steps/:stepSlug`
- If no enrolled paths → show a call-to-action: "Start learning → Browse catalog" → `/catalog`

### 10.5 Catalog (`/catalog`)

**Purpose:** Browse and discover all available learning paths.

**Required role:** `learner` (all authenticated users)

**Layout:** Global shell. Segmented view toggle (Grid / DAG) above the filter bar. Grid of learning path cards (default) or DAG visualization, with filter/search controls.

```
┌──────────────────────────────────────────────────────────────────────┐
│ Header                                                               │
├──────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Learning Path Catalog                                               │
│                                                                      │
│  [🔍 Search learning paths...                               ]       │
│                                                                      │
│  Filters:  [All ▾]  [Tag: kubernetes ✕]  [Sort: Competency Path ▾]  │
│            [Competency: docker-basics ✕] [linux-cli ✕]              │
│                                                                      │
│  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐   │
│  │ 🐧 Linux         │  │ 🐳 Docker        │  │ 🎯 Kubernetes    │   │
│  │ Fundamentals     │  │ Fundamentals     │  │ Fundamentals     │   │
│  │                  │  │                  │  │                  │   │
│  │ 3 modules · 2h   │  │ 4 modules · 3h   │  │ 5 modules · 3h   │   │
│  │ #linux #cli      │  │ #docker          │  │ #kubernetes      │   │
│  │                  │  │ #containers      │  │ #containers      │   │
│  │ Provides:        │  │ Provides:        │  │ Provides:        │   │
│  │  linux-cli       │  │  docker-basics   │  │  k8s-pods        │   │
│  │                  │  │ Prereqs: ✅      │  │ Prereqs: ⚠ 1/2  │   │
│  │ ██████████ 100%  │  │ ████████░░ 72%   │  │ Not started      │   │
│  └──────────────────┘  └──────────────────┘  └──────────────────┘   │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

**Card content:**

| Element | Source |
|---|---|
| Icon | `phoebus.yaml` → `icon` |
| Title | `phoebus.yaml` → `title` |
| Description | `phoebus.yaml` → `description` (truncated to ~100 chars) |
| Module count + duration | Computed from modules and `estimated_duration` |
| Tags | `phoebus.yaml` → `tags` (clickable — toggles filter) |
| Competencies provided | Aggregated from modules' `competencies` fields. Displayed as colored badges on each card |
| Prerequisites status | ✅ all met, ⚠ N/M met (with count), or hidden if no prerequisites. Shown as warning badge if unmet |
| Progress | From `progress` table. "Not started" if no progress |

**Filters & Search:**

| Control | Behavior |
|---|---|
| Search input | Client-side full-text filter on title, description, tags. Debounced (300ms) |
| Tag filter | Click a tag on a card or in the filter bar to toggle. Multiple tags = AND. Shown as removable chips |
| Competency filter | Multi-select dropdown listing all competencies. Multiple = OR. Pre-settable via URL `?competencies=a,b`. Shown as removable chips |
| Status filter | Dropdown: All, Not Started, In Progress, Completed |
| Sort | Dropdown: Alphabetical (A-Z), Alphabetical (Z-A), Progress (desc), **Competency Path** (topological order based on competency dependencies — **default**) |

**API Calls:**
- `GET /api/learning-paths` — returns all learning paths with metadata, `competencies_provided` (aggregated), and `prerequisites_met` (boolean)
- `GET /api/competencies` — returns the list of all competencies across all modules (for the filter dropdown)
- `GET /api/learning-paths/dependencies` — returns all dependency edges (auto + manual + yaml). Each edge: `{ source_path_id, target_path_id, dep_type }`. Used by the DAG view to render edges.

**Navigation targets:**
- Click a card → `/paths/:pathSlug`
- "Browse Prerequisite Paths" (from popup) → `/catalog?competencies=...`
- Click "View Path" in DAG popover → `/paths/:pathSlug`

### 10.6 Learning Path Overview (`/paths/:pathSlug`)

**Purpose:** Display learning path details, module structure, and allow the learner to start or continue.

**Required role:** `learner` (all authenticated users)

**Layout:** Global shell. Single-column content.

```
┌──────────────────────────────────────────────────────────────────────┐
│ Header                                                               │
├──────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ← Back to Catalog                                                   │
│                                                                      │
│  🎯 Kubernetes Fundamentals                                          │
│                                                                      │
│  Master the fundamentals of Kubernetes: from pods to deployments,    │
│  services, and configuration management.                             │
│                                                                      │
│  Tags: #kubernetes #containers #orchestration                        │
│  Duration: ~3 hours · 5 modules · 18 steps                          │
│  Prerequisites: Docker Basics (✅ completed)                         │
│                                                                      │
│  Progress: ████████████░░░░ 72%               [Continue Learning]    │
│                                                                      │
│  ┌─ Modules ─────────────────────────────────────────────────────┐   │
│  │                                                                │   │
│  │  ✅ 1. Introduction to Kubernetes         4 steps   ✓ done    │   │
│  │     ├── ✅ 📖 What is Kubernetes?                             │   │
│  │     ├── ✅ 📖 Architecture Overview                           │   │
│  │     ├── ✅ ❓ Key Concepts Quiz                               │   │
│  │     └── ✅ 💻 Install minikube                                │   │
│  │                                                                │   │
│  │  🔵 2. Pods & Containers                  5 steps   3/5       │   │
│  │     ├── ✅ 📖 Pod Basics                                      │   │
│  │     ├── ✅ 💻 Create a Pod                                    │   │
│  │     ├── ✅ ❓ Pod Lifecycle                                    │   │
│  │     ├── ○  🔧 Fix Pod CrashLoop           ← current          │   │
│  │     └── ○  📖 Multi-container Pods                            │   │
│  │                                                                │   │
│  │  ▸ 3. Services & Networking               4 steps   0/4       │   │
│  │  ▸ 4. Configuration                       3 steps   0/3       │   │
│  │  ▸ 5. Deployments & Rollouts              2 steps   0/2       │   │
│  │                                                                │   │
│  └────────────────────────────────────────────────────────────────┘   │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

**Elements:**

| Element | Behavior |
|---|---|
| ← Back to Catalog | Navigates to `/catalog` |
| Path metadata | Title, description, tags, duration, prerequisites, module/step count |
| Prerequisite status | ✅ if completed, ⚠ if not (with link to the prerequisite path). Not blocking — advisory only. Clicking "Start Learning" with unmet prerequisites triggers a confirmation popup (see §3.1.1) |
| Progress bar + percentage | Overall learning path completion |
| Continue Learning button | Navigates to the next incomplete step. Becomes "Start Learning" if not started. Hidden if completed. If prerequisites are unmet, shows the prerequisite enforcement popup first (§3.1.1) |
| Module list | Expandable/collapsible (Ant Design Collapse). Shows step list with type icons and completion |
| Current step indicator | "← current" on the last in-progress step |

**Step type icons:** 📖 Lesson · ❓ Quiz · 💻 Terminal Exercise · 🔧 Code Exercise

**API Calls:**
- `GET /api/learning-paths/:pathId` — returns path metadata, modules, steps (titles, types, order)
- `GET /api/me/progress?path_id=:pathId` — returns learner's progress for all steps in this path

**Navigation targets:**
- Click any step → `/paths/:pathSlug/steps/:stepSlug`
- Continue Learning → `/paths/:pathSlug/steps/:stepSlug` (next incomplete step)
- Prerequisite link → `/paths/:prerequisitePathId`
- ← Back to Catalog → `/catalog`

### 10.7 Step View (`/paths/:pathSlug/steps/:stepSlug`)

**Purpose:** Display and interact with a single learning step (lesson, quiz, terminal exercise, or code exercise).

**Required role:** `learner` (all authenticated users)

**Layout:** Replaces the global shell with a learning-specific layout: collapsible sidebar + full-width content area.

```
┌──────────────────────────────────────────────────────────────────────┐
│  ← Kubernetes Fundamentals                              [☰] [✕]    │
├──────────────┬───────────────────────────────────────────────────────┤
│ Sidebar      │  Content Area                                        │
│              │                                                       │
│ ▾ Module 1   │  (varies by step type — see 10.7.1 to 10.7.4)       │
│   ✅ Step 1  │                                                       │
│   ✅ Step 2  │                                                       │
│   ✅ Step 3  │                                                       │
│ ▾ Module 2   │                                                       │
│   ✅ Step 1  │                                                       │
│   🔵 Step 2  │                                                       │
│   ○  Step 3  │                                                       │
│ ▸ Module 3   │                                                       │
│ ▸ Module 4   │                                                       │
│              │                                                       │
│              ├───────────────────────────────────────────────────────┤
│              │  [← Previous]                          [Next →]      │
└──────────────┴───────────────────────────────────────────────────────┘
```

**Header elements:**

| Element | Behavior |
|---|---|
| ← Path title | Click → back to `/paths/:pathSlug` (learning path overview) |
| ☰ (hamburger) | Toggle sidebar visibility. Default: expanded on desktop (≥1024px), collapsed on mobile |
| ✕ (close) | Exit learning mode → `/paths/:pathSlug` |

**Sidebar:**

| Element | Behavior |
|---|---|
| Module headers | Expandable/collapsible (Ant Design Menu in inline mode). Title + completion count (e.g., "3/5") |
| Step entries | Type icon + title + completion indicator (✅ completed, ⏳ in-progress, ○ not started). Click → navigate to that step |
| Current step | Active state (bold, accent background color) |
| Scroll | Auto-scrolls to keep the current step visible on load |
| Resize | Sidebar width is adjustable via a drag handle on the right edge (180px–500px range). Handle highlights blue on hover |
| Collapse | When collapsed (48px), only the unfold button is visible — the menu is hidden entirely to avoid popup overlap |

**Footer navigation:**

| Element | Behavior |
|---|---|
| ← Previous | Navigate to the previous step (across module boundaries). Hidden on first step |
| Next → | Navigate to the next step (for exercises). Hidden on last step. Label changes to "Complete Path" on the last step of the last module if all steps are completed |

**Content area — varies by step type:**

#### 10.7.1 Lesson Step

Renders the Markdown body as rich HTML (see section 2.4 for rendering rules).

| Element | Behavior |
|---|---|
| Rendered content | Full Markdown rendering: headings, code blocks, Mermaid diagrams, admonitions, images |
| "Complete & Continue" button | Unified button at the bottom of content (replaces separate "Mark as Completed" and "Next Step" buttons). **Disabled** until the learner has scrolled through ≥75% of the lesson content. When the scroll threshold is reached, `POST /api/progress` sets status to `in_progress`. Clicking the button calls `POST /api/progress/:stepId/complete` and navigates to the next step. On the last step of the path, a congratulations modal is shown before redirecting to the path overview. Becomes "✅ Completed" (disabled) if already completed |

**Scroll-based progress tracking:** The 75% scroll detection is entirely **client-side** (IntersectionObserver or scroll event). No new API endpoints are needed — it reuses the existing `POST /api/progress` with status `in_progress`.

**API Calls:**
- `GET /api/learning-paths/:pathId/steps/:stepId` — returns `content_md` + step metadata
- `POST /api/progress` — updates progress to `in_progress` (triggered by 75% scroll)
- `POST /api/progress/:stepId/complete` — marks step as completed

#### 10.7.2 Quiz Step

Renders questions one at a time with immediate feedback (see section 6 for detailed behavior).

| Element | Behavior |
|---|---|
| Question counter | "Question N/M" in the content header |
| Question text | Markdown-rendered |
| Answer input | Radio buttons (single-select), checkboxes (multi-select), or text input (short-answer) |
| Submit button | `POST /api/exercises/:stepId/attempt` with answer payload. Server returns `{ is_correct, explanation, correct_answer }` |
| Feedback | Green/red highlight + explanation blockquote. Shown immediately after submit |
| Next Question | Advances to next question. No going back (see section 6.3) |
| Summary panel | After last question: correct count, per-question status, "Review Explanations" toggle |
| Reset button | Top-right corner. Confirmation dialog → `POST /api/exercises/:stepId/reset` |

**API Calls:**
- `GET /api/learning-paths/:pathId/steps/:stepId` — step metadata + sanitized `exercise_data` (questions without `correct` flags)
- `POST /api/exercises/:stepId/attempt` — submit one question answer
- `GET /api/exercises/:stepId/attempts` — resume from last answered question
- `POST /api/exercises/:stepId/reset` — reset quiz

#### 10.7.3 Terminal Exercise Step

Renders the terminal simulator (see section 4 for detailed behavior).

| Element | Behavior |
|---|---|
| Step counter | "Step N/M" in the content header |
| Introduction | Markdown-rendered exercise context (always visible above the terminal) |
| Per-step context | Additional context text for the current step |
| Terminal area | Dark background, monospace font. Renders instructor-defined `console` block as-is. Accumulates commands + outputs from previous steps (scrollable) |
| Command proposals | Ant Design `Radio.Group` — single selection |
| Submit button | `POST /api/exercises/:stepId/attempt` with `{ step_number, selected_command }`. Returns `{ is_correct, explanation, output }` |
| Correct feedback | Command appears typed in terminal, output displayed below, Ant Design `Alert` (success) |
| Incorrect feedback | Ant Design `Alert` (error) with explanation. Wrong choice gets strike-through and disabled |
| Summary | After last step: full terminal history with all commands and outputs |
| Reset button | Top-right. Confirmation → `POST /api/exercises/:stepId/reset` |

**API Calls:**
- `GET /api/learning-paths/:pathId/steps/:stepId` — step metadata + sanitized `exercise_data` (proposals without `correct` flags, prompts, contexts)
- `POST /api/exercises/:stepId/attempt` — submit command selection
- `GET /api/exercises/:stepId/attempts` — resume after last correctly answered step
- `POST /api/exercises/:stepId/reset` — reset terminal exercise

#### 10.7.4 Code Exercise Step

Renders a VS Code-like IDE layout with full-bleed display (no padding, no max-width). The layout adapts to the active theme (light/dark).

```
┌────────────────────────────────────────────────────────────────────┐
│  📄 main.go                     [Phase 1: Find the bug]           │  ← Top bar
├────────┬───────────────────────────────────────────────────────────┤
│Explorer│  Monaco Editor (read-only)                    ┃ minimap  │
│        │  1  package main                              ┃          │
│ 📁 cmd │  2  import "fmt"                              ┃          │
│ 📄 main│  3  func main() {                             ┃          │
│ 📄 go. │  4    fmt.Println("Hello")                    ┃          │
│        │  5  }                                         ┃          │
├────────┴───────────────────────────────────────────────────────────┤
│  ═══════════════════════════  (drag handle)                        │
├────────────────────────────────────────────────────────────────────┤
│  Description / Controls (bottom panel, resizable 100–500px)       │
│  Click line numbers to select problematic lines.  [Validate]      │
└────────────────────────────────────────────────────────────────────┘
```

| Element | Behavior |
|---|---|
| Top bar | File name + phase indicator tag. Background follows IDE secondary color (`--color-bg-ide-secondary`) |
| File tree | Left panel (Ant Design Tree, 200px). All `codebase/` files shown. Target file pre-selected. Background transparent over IDE secondary |
| Code viewer | Center panel. Monaco Editor (read-only). Theme: `vs` (light mode) or `vs-dark` (dark mode), switches automatically. Syntax highlighting, minimap (scale 2, proportional), line numbers, folding |
| Bottom panel | Resizable via drag handle (100–500px). Contains problem description (Markdown), controls, and feedback |
| Phase indicator | "Phase 1/2" (modes A/C) or single phase (mode B). Displayed as colored Tag in top bar |
| Selected lines | Click on gutter (line number / glyph margin) to toggle line selection. Blue highlight decoration. "Selected lines: 12, 13" display |
| Validate Selection | Phase 1 button. `POST /api/exercises/:stepId/attempt` with `{ phase: "identify", selected_lines }`. Returns `{ is_correct, feedback }` with progressive hints |
| Patch proposals | Phase 2 (or sole phase for mode B). Radio group in bottom panel. Tooltip shows unified diff on hover |
| Submit Patch | `POST /api/exercises/:stepId/attempt` with `{ phase: "fix", selected_patch }`. Returns `{ is_correct, explanation }` |
| Target highlight | Green decoration on target lines in Phase 2 as visual reference |
| Disabled button | When Validate/Submit is disabled, uses theme-aware contrast (`--color-ide-disabled-bg/text/border`) |

**API Calls:**
- `GET /api/learning-paths/:pathId/steps/:stepId` — step metadata + sanitized `exercise_data` (patches without `correct` flags, codebase files, target info) + codebase files content
- `POST /api/exercises/:stepId/attempt` — submit line selection or patch selection
- `GET /api/exercises/:stepId/attempts` — resume at correct phase
- `POST /api/exercises/:stepId/reset` — reset code exercise

### 10.8 Analytics Dashboard (`/analytics`)

**Purpose:** High-level overview of platform learning metrics for instructors.

**Required role:** `instructor` or `admin`

**Layout:** Global shell. Dashboard grid with summary cards and table.

```
┌──────────────────────────────────────────────────────────────────────┐
│ Header                                                               │
├──────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Analytics                                                           │
│                                                                      │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────┐│
│  │ 3 paths      │  │ 42 learners  │  │ 68% avg.     │  │ 1,247    ││
│  │ published    │  │ enrolled     │  │ completion   │  │ attempts ││
│  └──────────────┘  └──────────────┘  └──────────────┘  └──────────┘│
│                                                                      │
│  ┌─ Learning Paths ──────────────────────────────────────────────┐  │
│  │                                                                │  │
│  │  Path                    Enrolled  Completion  Avg. Score  [→] │  │
│  │  Kubernetes Fundamentals    28        72%         85%      [→] │  │
│  │  GitOps with ArgoCD         15        45%         78%      [→] │  │
│  │  Terraform Advanced          8        20%         —        [→] │  │
│  │                                                                │  │
│  └────────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  ┌─ Recent Activity ─────────────────────────────────────────────┐  │
│  │  François D. enrolled in Terraform Advanced           2h ago  │  │
│  │  Marie L. completed Kubernetes Fundamentals          1d ago  │  │
│  │  Ahmed B. enrolled in GitOps with ArgoCD             2d ago  │  │
│  └────────────────────────────────────────────────────────────────┘  │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

**Sections:**

| Section | Data |
|---|---|
| Summary cards (Ant Design Statistic) | Total published paths, total enrolled learners, average completion rate, total exercise attempts |
| Learning paths table (Ant Design Table) | Per-path: title, enrolled count, completion rate, average exercise score. Sortable columns. Click row → `/analytics/paths/:pathSlug` |
| Recent activity | Last 10 enrollment/completion events with timestamps. Click learner name → `/analytics/learners/:learnerId` |

**API Calls:**
- `GET /api/analytics/overview` — returns summary cards + per-path aggregates (real-time SQL)
- `GET /api/analytics/activity?limit=10` — returns recent enrollment/completion events

**Navigation targets:**
- Click learning path row → `/analytics/paths/:pathSlug`
- Click learner name → `/analytics/learners/:learnerId`

### 10.9 Learning Path Analytics (`/analytics/paths/:pathSlug`)

**Purpose:** Detailed analytics for a specific learning path: step-level performance, failure points, common wrong answers.

**Required role:** `instructor` or `admin`

**Layout:** Global shell. Breadcrumb + tabbed content.

```
┌──────────────────────────────────────────────────────────────────────┐
│ Header                                                               │
├──────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ← Analytics  /  Kubernetes Fundamentals                             │
│                                                                      │
│  Enrolled: 28    Completed: 20 (72%)    Avg. time: 2h45              │
│                                                                      │
│  [Overview]  [Step Details]  [Learners]                              │
│                                                                      │
│  ── Overview Tab ──────────────────────────────────────────────────  │
│                                                                      │
│  Progress Distribution                                               │
│  ┌────────────────────────────────────────────┐                      │
│  │  0-25%  ██████ 6                           │                      │
│  │ 25-50%  ████ 4                             │                      │
│  │ 50-75%  ██ 2                               │                      │
│  │ 75-100% ████████████████ 16                │                      │
│  └────────────────────────────────────────────┘                      │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

**Tabs (Ant Design Tabs):**

| Tab | Content |
|---|---|
| Overview | Progress distribution histogram (Ant Design column chart), completion rate over time, summary stats |
| Step Details | Per-step table: type icon, title, completion rate, average attempts, failure indicator (⚠ for steps < 60% completion). Click a step row to expand with common wrong answers |
| Learners | Ant Design Table: learner name, progress %, last activity, time spent. Sortable. Click → `/analytics/learners/:learnerId` |

**Step drill-down (expanded row in Step Details tab):**

| Exercise Type | Drill-down content |
|---|---|
| Terminal exercise | Top 3 most-selected incorrect commands per step |
| Code exercise | Top 3 most-selected incorrect patches + common wrong line selections |
| Quiz | Per-question correct rate + most common wrong answers |

**API Calls:**
- `GET /api/analytics/paths/:pathId` — all path analytics (progress distribution, per-step stats, learner list)
- `GET /api/analytics/paths/:pathId/steps/:stepId` — step-level drill-down (common wrong answers, attempt distribution)

**Navigation:**
- ← Analytics breadcrumb → `/analytics`
- Learner row → `/analytics/learners/:learnerId`

### 10.10 Learner Detail (`/analytics/learners/:learnerId`)

**Purpose:** View an individual learner's complete progress timeline across all learning paths.

**Required role:** `instructor` or `admin`

**Layout:** Global shell. Breadcrumb + profile + timeline.

```
┌──────────────────────────────────────────────────────────────────────┐
│ Header                                                               │
├──────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ← Analytics  /  François Dupont                                     │
│                                                                      │
│  Role: learner    Last login: 2h ago    Joined: 2024-01-15          │
│                                                                      │
│  ┌─ Enrolled Paths ──────────────────────────────────────────────┐  │
│  │                                                                │  │
│  │  Kubernetes Fundamentals         ████████████░░ 72%           │  │
│  │  GitOps with ArgoCD              ███░░░░░░░░░░░ 20%           │  │
│  │                                                                │  │
│  └────────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  ┌─ Activity Timeline ───────────────────────────────────────────┐  │
│  │                                                                │  │
│  │  Today                                                         │  │
│  │  ● 10:30  Completed "ConfigMap & Secrets" quiz (2/3)          │  │
│  │  ● 10:15  Started "ConfigMap & Secrets" quiz                   │  │
│  │                                                                │  │
│  │  Yesterday                                                     │  │
│  │  ● 16:45  Completed "Fix Pod CrashLoop" code exercise         │  │
│  │  ● 16:20  Attempted "Fix Pod CrashLoop" (3 attempts)          │  │
│  │                                                                │  │
│  └────────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  ┌─ Exercise Performance ────────────────────────────────────────┐  │
│  │                                                                │  │
│  │  Exercise                   Attempts   Result   Time           │  │
│  │  Key Concepts Quiz              1      3/3       5m           │  │
│  │  Install minikube               2      ✅        8m           │  │
│  │  Fix Pod CrashLoop              3      ✅       25m           │  │
│  │                                                                │  │
│  └────────────────────────────────────────────────────────────────┘  │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

**Sections:**

| Section | Data |
|---|---|
| Profile | Name, role, last login, join date |
| Enrolled paths | Learning paths with progress bars |
| Activity timeline (Ant Design Timeline) | Chronological list of all progress and attempt events. Grouped by day |
| Exercise performance (Ant Design Table) | Per-exercise: name, number of attempts, result, estimated time spent. Sortable |

**API Calls:**
- `GET /api/analytics/learners/:learnerId` — returns profile, enrolled paths with progress, activity timeline, exercise performance

**Navigation:**
- ← Analytics breadcrumb → `/analytics`
- Click on enrolled path → `/analytics/paths/:pathSlug`

### 10.11 Repository Management (`/admin/repositories`)

**Purpose:** List and manage registered Git repositories. Display the instance SSH public key for deploy key setup.

**Required role:** `admin`

**Layout:** Global shell. Ant Design Table with action buttons. SSH public key banner above the table.

```
┌──────────────────────────────────────────────────────────────────────┐
│ Header                                                               │
├──────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Repository Management                          [+ Add Repository]   │
│                                                                      │
│  ┌────────────────────────────────────────────────────────────────┐  │
│  │ 🔑 Instance SSH Public Key                          [Copy]    │  │
│  │ ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI... phoebus-instance    │  │
│  │                                                                │  │
│  │ Add this key as a read-only deploy key on your Git             │  │
│  │ repositories to use SSH clone URLs (git@…).                    │  │
│  └────────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  ┌────────────────────────────────────────────────────────────────┐  │
│  │ Name             Clone URL              Branch  Status   Act. │  │
│  │ K8s Fundamentals git@github.com/...     main    ✅ synced  ⋮  │  │
│  │ GitOps ArgoCD    https://gitlab.com/... main    ✅ synced  ⋮  │  │
│  │ Terraform Adv.   git@github.com/...     main    ❌ error   ⋮  │  │
│  └────────────────────────────────────────────────────────────────┘  │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

**SSH Public Key Banner (Ant Design Alert, type: info):**

The instance SSH public key is always displayed above the repository table. It is generated once at first startup and persists across restarts.

| Element | Description |
|---|---|
| Key display | Full public key in `ssh-ed25519 AAAA...` format, monospace font |
| Copy button | Copies the full public key to clipboard. Toast: "SSH public key copied" |
| Help text | "Add this key as a read-only deploy key on your Git repositories to use SSH clone URLs (git@…)." |

**API Call:** `GET /api/admin/ssh-public-key` — returns `{ "public_key": "ssh-ed25519 AAAA..." }`

**Table columns (Ant Design Table):**

| Column | Description |
|---|---|
| Name | Learning path title (from `phoebus.yaml`) |
| Clone URL | Git repository URL (truncated, tooltip shows full URL) |
| Branch | Tracked branch |
| Last Synced | Relative timestamp ("2h ago"), tooltip shows absolute time |
| Status | Badge: `synced` (green), `syncing` (blue, spinning), `error` (red), `never_synced` (grey) |
| Actions | Dropdown menu (⋮): Edit, Sync Now, Copy Webhook URL, Delete |

**Actions behavior:**

| Action | Behavior |
|---|---|
| + Add Repository | Navigates to `/admin/repositories/new` |
| Edit | Navigates to `/admin/repositories/:repoId/edit` |
| Sync Now | `POST /api/admin/repos/:repoId/sync`. Status changes to `syncing`. Toast notification on completion/failure |
| Copy Webhook URL | Copies `https://{host}/api/webhooks/{uuid}` to clipboard. Toast: "Webhook URL copied" |
| Delete | Ant Design Modal confirmation ("This will remove all content from this repository. Learner progress will be preserved."). `DELETE /api/admin/repos/:repoId` |
| Sync Logs | Navigates to `/admin/repositories/:repoId/sync-logs`. Shows the complete sync history for this repository |

**Error display:** If status is `error`, clicking the status badge opens a popover showing the error message from `sync_error`.

**API Calls:**
- `GET /api/admin/repos` — returns all repositories with sync status
- `POST /api/admin/repos/:repoId/sync` — trigger manual sync
- `DELETE /api/admin/repos/:repoId` — delete repository
- `GET /api/admin/repos/:repoId/sync-logs` — returns sync job history for this repository

**Navigation targets:**
- + Add Repository → `/admin/repositories/new`
- Edit → `/admin/repositories/:repoId/edit`
- Sync Logs → `/admin/repositories/:repoId/sync-logs`

### 10.12 Add / Edit Repository (`/admin/repositories/new`, `/admin/repositories/:repoId/edit`)

**Purpose:** Form to register a new Git repository or edit an existing one.

**Required role:** `admin`

**Layout:** Global shell. Centered form (Ant Design Form).

```
┌──────────────────────────────────────────────────────────────────────┐
│ Header                                                               │
├──────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ← Repositories  /  Add Repository                                   │
│                                                                      │
│  Clone URL *        [_______________________________________]        │
│                                                                      │
│  Branch             [main_________________________________]          │
│                                                                      │
│  Authentication     [Instance SSH Key ▾]                              │
│                                                                      │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │ 🔑 Instance SSH Public Key                        [Copy]    │    │
│  │ ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI... phoebus-instance  │    │
│  │ Add this key as a read-only deploy key on your Git repo.    │    │
│  └──────────────────────────────────────────────────────────────┘    │
│                                                                      │
│  ── Credentials (shown when auth = http-basic or http-token) ─────  │
│                                                                      │
│  Token              [_______________________________________]        │
│                                                                      │
│                                                                      │
│  [Cancel]                                           [Save & Sync]    │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

**Form fields:**

| Field | Type | Validation | Required |
|---|---|---|---|
| Clone URL | Text input | Must be valid Git URL (SSH or HTTPS) | Yes |
| Branch | Text input | Default: `main` | No (defaults to `main`) |
| Authentication type | Select: `none`, `instance-ssh-key`, `http-basic`, `http-token` | — | Yes |
| | _(selecting `instance-ssh-key` displays the instance SSH public key in an info alert below, with a copy button — no credential fields shown)_ | | |
| Username | Text input (shown if `http-basic`) | — | Conditional |
| Password | Password input (shown if `http-basic`) | — | Conditional |
| Token | Password input (shown if `http-token`) | — | Conditional |

**Behavior:**

| Action | Behavior |
|---|---|
| Save & Sync | Validates form → `POST /api/admin/repos` (new) or `PUT /api/admin/repos/:repoId` (edit). On success, triggers initial sync and redirects to `/admin/repositories` with toast "Repository added, sync in progress" |
| Cancel | Navigates back to `/admin/repositories` (no save) |
| Edit mode | Pre-fills form with existing data. Credentials are shown as "••••••" (not retrievable). Leaving credential fields empty preserves existing credentials |

**API Calls:**
- `POST /api/admin/repos` — create new repository
- `PUT /api/admin/repos/:repoId` — update existing repository
- `GET /api/admin/repos/:repoId` — load existing data (edit mode)

**Navigation:**
- ← Repositories breadcrumb → `/admin/repositories`
- On save → `/admin/repositories`
- Cancel → `/admin/repositories`

### 10.13 Sync Logs (`/admin/repositories/:repoId/sync-logs`)

**Purpose:** View the complete synchronization history for a specific Git repository.

**Required role:** `admin`

**Layout:** Global shell. Header with repository info, followed by Ant Design Table listing all sync jobs.

```
┌──────────────────────────────────────────────────────────────────────┐
│ Header                                                               │
├──────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ← Back to Repositories                                             │
│                                                                      │
│  Sync Logs: git@github.com/company/k8s-content.git (main)           │
│                                                                      │
│  ┌────────────────────────────────────────────────────────────────┐  │
│  │ #   Status     Started              Duration   Error          │  │
│  │ 1   ✅ done    2025-01-15 14:32:05  3.2s       —              │  │
│  │ 2   ✅ done    2025-01-15 12:10:44  2.8s       —              │  │
│  │ 3   ❌ failed  2025-01-14 09:05:12  1.1s       git clone fa…  │  │
│  │ 4   ✅ done    2025-01-13 16:48:30  4.5s       —              │  │
│  └────────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  Showing 4 sync jobs                            [← 1  2  3  4  →]  │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

**Table columns (Ant Design Table):**

| Column | Description |
|---|---|
| # | Row number (most recent first) |
| Status | Badge: `done` (green), `failed` (red), `processing` (blue, spinning), `pending` (grey) |
| Started | Absolute timestamp of `started_at` (or `created_at` if not yet started). Relative time in tooltip |
| Duration | Computed from `started_at` → `completed_at` (e.g., "3.2s", "1m 12s"). Shows "—" if not completed |
| Error | Error message (truncated to 80 chars). Full error shown in expandable row or tooltip |

**Sorting:** Most recent jobs first (by `created_at DESC`).

**Error detail:** Clicking on a truncated error message or expanding the row reveals the full error text in a code block for easy reading.

**Expandable rows — Detailed sync logs:** Each sync job row is expandable. Clicking the expand arrow loads the detailed log entries for that job via `GET /api/admin/repos/:repoId/sync-logs/:jobId`. The expanded section shows a sub-table with columns:

| Column | Description |
|---|---|
| Time | Timestamp of the log entry (HH:MM:SS.mmm) |
| Level | Tag colored by severity: `debug` (grey), `info` (blue), `warn` (orange), `error` (red) |
| Message | Log message text |
| Details | Structured fields (key=value pairs), excluding repo_id and job_id |

Log entries are captured during sync by a dual-write collector: they are emitted to stdout in the configured format AND accumulated in memory. Additionally, stderr output from git operations (clone, fetch) is captured via `io.MultiWriter` and included in the sync logs, ensuring that git warnings and errors are visible to administrators. At the end of the sync job (success or failure), the accumulated entries are persisted as JSONB in the `sync_jobs.logs` column.

**API Calls:**
- `GET /api/admin/repos/:repoId/sync-logs` — returns array of sync jobs for this repository, ordered by `created_at DESC`
- `GET /api/admin/repos/:repoId/sync-logs/:jobId` — returns array of detailed log entries for a specific sync job

**Response format:**
```json
[
  {
    "id": "uuid",
    "repo_id": "uuid",
    "status": "done",
    "error": null,
    "attempts": 1,
    "started_at": "2025-01-15T14:32:05Z",
    "completed_at": "2025-01-15T14:32:08Z",
    "created_at": "2025-01-15T14:32:04Z"
  }
]
```

**Navigation:**
- ← Back to Repositories → `/admin/repositories`

### 10.14 User Management (`/admin/users`)

**Purpose:** List all users, change roles, deactivate accounts, and create local users.

**Required role:** `admin`

**Layout:** Global shell. Ant Design Table with inline actions.

```
┌──────────────────────────────────────────────────────────────────────┐
│ Header                                                               │
├──────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  User Management                                    [+ Add User]     │
│                                                                      │
│  [🔍 Search users...                    ]                            │
│                                                                      │
│  ┌────────────────────────────────────────────────────────────────┐  │
│  │ Name            Email              Role         Last Login  ⋮ │  │
│  │ François D.     francois@acme.com  admin        2h ago      ⋮ │  │
│  │ Marie L.        marie@acme.com     instructor   1d ago      ⋮ │  │
│  │ Ahmed B.        ahmed@acme.com     learner      3d ago      ⋮ │  │
│  │ Sophie R.       sophie@acme.com    learner      inactive    ⋮ │  │
│  └────────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  Showing 4 of 42 users                        [← 1  2  3  4  5 →]  │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

**"Add User" modal** (shown when clicking the [+ Add User] button, only when local auth is enabled):

```
┌─────────────────────────────────────┐
│  Create Local User                  │
│                                     │
│  Username      [___________________]│
│  Display Name  [___________________]│
│  Email         [___________________]│  ← Optional
│  Role          (●) learner          │
│                ( ) instructor       │
│                ( ) admin            │
│  Password      [___________________]│
│                                     │
│        [Cancel]  [Create User]      │
│                                     │
│  ⚠ Username already taken           │  ← Error (conditional)
└─────────────────────────────────────┘
```

**Table columns:**

| Column | Description |
|---|---|
| Name | Display name (from OIDC/LDAP or local) |
| Email | User email |
| Role | Badge: `learner` (blue), `instructor` (green), `admin` (red) |
| Last Login | Relative timestamp. "inactive" if deactivated (greyed row) |
| Completed Paths | Number of completed learning paths |
| Actions | Dropdown (⋮): Change Role, Deactivate / Reactivate |

**Actions behavior:**

| Action | Behavior |
|---|---|
| Add User | Ant Design Modal with form (username, display name, email, role, password). `POST /api/admin/users` with `{ username, display_name, email, role, password }`. Only shown when local auth is enabled. Toast confirmation "User created" |
| Change Role | Ant Design Modal with role selector (radio: learner, instructor, admin). `PATCH /api/users/:userId` with `{ role }`. Toast confirmation |
| Deactivate | Confirmation modal. `PATCH /api/users/:userId` with `{ active: false }`. User cannot log in. Row is greyed out |
| Reactivate | `PATCH /api/users/:userId` with `{ active: true }`. Row returns to normal |
| Search | Client-side filter on name and email. Debounced (300ms) |

**Pagination:** Server-side, 20 users per page (Ant Design Pagination).

**API Calls:**
- `GET /api/users?page=1&per_page=20` — paginated user list
- `POST /api/admin/users` — create a local user (admin only, local auth must be enabled)
- `PATCH /api/users/:userId` — update role or active status

### 10.15 Platform Health (`/admin/health`)

**Purpose:** Monitor platform health and operational metrics.

**Required role:** `admin`

**Layout:** Global shell. Dashboard with status cards and metrics.

```
┌──────────────────────────────────────────────────────────────────────┐
│ Header                                                               │
├──────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Platform Health                                                     │
│                                                                      │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐              │
│  │ 🟢 API       │  │ 🟢 Database  │  │ 🟢 3/3 repos │              │
│  │ up 14d 3h    │  │ connected    │  │ synced       │              │
│  └──────────────┘  └──────────────┘  └──────────────┘              │
│                                                                      │
│  ┌─ System Metrics ──────────────────────────────────────────────┐  │
│  │                                                                │  │
│  │  Active users (24h):     12                                    │  │
│  │  API response p50:       8ms                                   │  │
│  │  API response p95:       45ms                                  │  │
│  │  API response p99:       120ms                                 │  │
│  │  Disk usage (tmp):       0 B  (ephemeral clones, cleaned)      │  │
│  │                                                                │  │
│  └────────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  ┌─ Repository Sync Status ──────────────────────────────────────┐  │
│  │                                                                │  │
│  │  K8s Fundamentals      ✅ synced      Last: 2h ago            │  │
│  │  GitOps ArgoCD         ✅ synced      Last: 1d ago            │  │
│  │  Terraform Advanced    ❌ error       "Clone failed: auth"    │  │
│  │                                                                │  │
│  └────────────────────────────────────────────────────────────────┘  │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

**Sections:**

| Section | Data |
|---|---|
| Status cards | API health (up/down + uptime), database connection (connected/disconnected), repo sync summary (N/M synced) |
| System metrics | Active users (24h), API response times (p50/p95/p99), disk usage |
| Repository sync status | Per-repo: name, status badge, last sync time, error message if applicable |

**API Calls:**
- `GET /api/admin/health` — returns all health data (API status, DB status, repo sync statuses, system metrics)

**Auto-refresh:** The health page polls `GET /api/admin/health` every 30 seconds to keep metrics current.

**Note:** Detailed Prometheus metrics are available at `/metrics` for external monitoring tools (Grafana). This page is a lightweight summary for quick admin checks.

### 10.17 Structured Logging

**Purpose:** Provide comprehensive, structured logging for all backend components to facilitate debugging, monitoring, and auditing.

**Configuration (via `config/log`):**

| Setting | Description | Default |
|---|---|---|
| `format` | Output format: `json`, `text`, or `gelf` | `json` |
| `level` | Minimum log level: `debug`, `info`, `warn`, `error` | `info` |
| `request_id_header` | HTTP header for upstream request ID (e.g., `X-Request-Id`) | — (auto-generate UUID) |

**Log formats:**

- **json** — Standard Go `slog.JSONHandler` output. Fields: `time`, `level`, `msg`, plus structured attributes.
- **text** — Human-readable `slog.TextHandler` output. Suitable for local development.
- **gelf** — GELF 1.1 spec-compliant JSON. Fields: `version` (1.1), `host`, `short_message`, `timestamp` (epoch float), `level` (syslog severity). Additional fields prefixed with `_`. Compatible with Graylog, Loki, and other GELF consumers.

**HTTP request logging middleware:**

Every HTTP request is logged with the following fields:

| Field | Description |
|---|---|
| `method` | HTTP method (GET, POST, etc.) |
| `path` | Request URL path |
| `status` | Response status code |
| `duration_ms` | Request duration in milliseconds |
| `request_id` | Unique request ID (from upstream header or auto-generated UUID) |
| `user_agent` | Client User-Agent header |
| `remote_addr` | Client IP address |
| `user_id` | Authenticated user ID (added by auth middleware) |
| `role` | Authenticated user role (added by auth middleware) |
| `bytes_written` | Response body size in bytes |

Log level is determined by status code: `info` for 2xx/3xx, `warn` for 4xx, `error` for 5xx.

**Context propagation:** The logger is injected into `context.Context` by the HTTP middleware and enriched by the auth middleware with `user_id` and `role`. All downstream components access the logger via `logging.FromContext(ctx)`, ensuring structured fields are preserved across the call chain.

**Sync & parser logging:** The syncer and parser emit structured logs at each stage:
- `info`: sync started, repo details fetched, clone succeeded, paths discovered (count), path/module/step upserted, sync completed (with duration)
- `warn`: missing optional fields (description, tags, duration, explanation), empty modules/steps, assets exceeding max size
- `debug`: hash comparisons, unchanged steps skipped, individual parse results
- `error`: clone failures, parse errors (with file name and error detail), DB errors

**Sync log persistence:** A dual-write `SyncCollector` handler captures all log records during a sync job in memory while forwarding them to the standard output handler. At job completion (success or failure), the accumulated log entries are persisted as JSONB in the `sync_jobs.logs` column, accessible via the admin UI.

### 10.16 Navigation Graph

The following Mermaid diagram shows the navigation flow between all views:

```mermaid
graph TD
    LOGIN["/login<br>Login"] --> DASHBOARD

    DASHBOARD["/  <br>Dashboard"] --> CATALOG
    DASHBOARD --> STEP
    DASHBOARD --> PATH_OVERVIEW

    CATALOG["/catalog<br>Catalog"] --> PATH_OVERVIEW

    PATH_OVERVIEW["/paths/:id<br>Path Overview"] --> STEP
    PATH_OVERVIEW --> CATALOG

    STEP["/paths/:pathSlug/steps/:stepSlug<br>Step View"] --> PATH_OVERVIEW
    STEP -->|sidebar nav| STEP

    ANALYTICS["/analytics<br>Analytics Dashboard"] --> PATH_ANALYTICS
    ANALYTICS --> LEARNER_DETAIL

    PATH_ANALYTICS["/analytics/paths/:id<br>Path Analytics"] --> ANALYTICS
    PATH_ANALYTICS --> LEARNER_DETAIL
    PATH_ANALYTICS -->|step drill-down| PATH_ANALYTICS

    LEARNER_DETAIL["/analytics/learners/:id<br>Learner Detail"] --> ANALYTICS
    LEARNER_DETAIL --> PATH_ANALYTICS

    REPOS["/admin/repositories<br>Repositories"] --> REPO_FORM
    REPO_FORM["/admin/repositories/new|:id/edit<br>Repo Form"] --> REPOS

    USERS["/admin/users<br>Users"]
    HEALTH["/admin/health<br>Health"]

    %% Role-based access
    style LOGIN fill:#f5f5f5,stroke:#999
    style DASHBOARD fill:#e6f7ff,stroke:#1890ff
    style CATALOG fill:#e6f7ff,stroke:#1890ff
    style PATH_OVERVIEW fill:#e6f7ff,stroke:#1890ff
    style STEP fill:#e6f7ff,stroke:#1890ff
    style ANALYTICS fill:#f6ffed,stroke:#52c41a
    style PATH_ANALYTICS fill:#f6ffed,stroke:#52c41a
    style LEARNER_DETAIL fill:#f6ffed,stroke:#52c41a
    style REPOS fill:#fff1f0,stroke:#ff4d4f
    style REPO_FORM fill:#fff1f0,stroke:#ff4d4f
    style USERS fill:#fff1f0,stroke:#ff4d4f
    style HEALTH fill:#fff1f0,stroke:#ff4d4f
```

**Legend:** 🔵 Blue = all authenticated users · 🟢 Green = instructor+ · 🔴 Red = admin only

### 10.18 Onboarding Tour

**Purpose:** Guide new users through the UI with an interactive step-by-step tour on their first visit.

**Implementation:**

| Aspect | Detail |
|---|---|
| Library | React Joyride |
| Tours | Two separate tours: **Dashboard** tour and **Catalog** tour |
| Trigger | Each tour starts automatically the first time the user visits the corresponding page |
| Replay | A **"?"** button in the global header (`AppLayout`) replays the current page's tour on click |
| Target elements | Dashboard and Catalog components expose `data-tour` attributes on key UI elements to anchor tour steps |
| State management | `OnboardingContext` (React Context) + `useOnboarding` hook manage tour visibility and status client-side |
| Persistence | Tour completion state is persisted server-side via the `onboarding_tours_seen` JSONB column on the `users` table |
| API | `GET /api/me/onboarding` — returns `{ tour_name: boolean }` status; `PATCH /api/me/onboarding` — body `{"tour":"<name>"}` marks a tour as seen; `DELETE /api/me/onboarding` — resets all tours |
| E2E testing | Onboarding tours are covered by end-to-end tests |
