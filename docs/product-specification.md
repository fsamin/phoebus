# Phœbus — Product Specification

## 1. Vision

Phœbus is an open-source e-learning platform purpose-built for **DevOps and software engineering training** in enterprise environments. It enables organizations to upskill and reskill their engineering teams through hands-on, practical learning experiences.

Phœbus promotes **self-assessment**: learners progress at their own pace, in their own style. There is no grading, no ranking, no instructor gatekeeping. Programming exercises and terminal sessions are **practical applications** of the pedagogical content — they exist so learners can build muscle memory and validate their own understanding, not to be evaluated by others. Automated feedback (test results, validation scripts) serves the learner directly, helping them iterate until they master the skill.

What sets Phœbus apart is its **content-as-code** philosophy: all training content lives in Git repositories as Markdown files, making content creation and updates as agile as software development itself. There is no CMS, no WYSIWYG editor, no publishing pipeline — **what is in Git is what learners see**.

Phœbus provides **interactive, guided exercises** — terminal command sequencing, code review challenges, and quizzes — all rendered in the browser with no infrastructure to provision. Combined with rich Markdown lessons, Phœbus bridges the gap between theoretical knowledge and real-world DevOps practice.

### Core Principles

| Principle | Description |
|---|---|
| **Content-as-Code** | Training content is Markdown in Git repos. One repo per learning path. No CMS, no build step. |
| **Self-Assessment** | Learners evaluate themselves. No grading, no ranking. Automated feedback serves the learner, not the instructor. |
| **Hands-on First** | Exercises are practical applications of lessons. Learners practice through interactive terminal simulations, code review challenges, and quizzes — not just read slides. |
| **Observable Practice** | All exercise attempts are recorded, enabling instructors to understand how learners approach problems. |
| **Agile Content Lifecycle** | Content follows the same workflow as code: branch, edit, review, merge, deploy. |
| **Open Source** | The platform is free and open, encouraging community contributions and transparency. |
| **Enterprise-Ready** | Designed for single-tenant, on-premise deployment within corporate environments. |

---

## 2. Personas

### 2.1 Learner (Developer / Ops Engineer)

**Profile:** A software developer, system administrator, or operations engineer within an enterprise. They need to acquire or sharpen skills in DevOps practices, tooling, and workflows.

**Goals:**
- Follow structured learning paths to develop new competencies
- Practice through interactive exercises: terminal command sequences, code review challenges, quizzes
- Self-assess understanding through hands-on exercises with immediate automated feedback
- Track personal progress across learning paths and competencies
- Learn at their own pace, in their own style, revisiting and resetting exercises as needed

**Pain Points:**
- Existing training platforms feel too academic or disconnected from real tooling
- Slides and videos don't build muscle memory — they need hands-on practice
- Hard to find training content that stays up-to-date with fast-moving DevOps ecosystem

---

### 2.2 Instructor (Tech Lead / Staff Engineer / Training Lead)

**Profile:** An experienced engineer or training lead responsible for creating and maintaining training content. They are comfortable with Git, Markdown, and DevOps tooling.

**Goals:**
- Author and update training content using familiar tools (Git, Markdown, their IDE)
- Define learning paths composed of ordered modules and exercises
- Design exercises as practical applications of lessons — not as evaluation tools
- Monitor learner progress and identify knowledge gaps across teams
- Collaborate with other instructors via Git workflows (branches, pull requests, reviews)

**Pain Points:**
- Traditional LMS platforms require tedious UI-based content editing
- Content goes stale quickly and updating it is a heavyweight process
- Exercises are either too simplistic (basic QCM) or too heavy (full lab environments)

---

### 2.3 Platform Administrator

**Profile:** An infrastructure or platform engineer responsible for deploying, configuring, and maintaining the Phœbus instance within the enterprise.

**Goals:**
- Deploy and operate a single-tenant Phœbus instance on-premise or in the company cloud
- Integrate with corporate identity providers (SSO / LDAP / OIDC)
- Monitor platform health and resource usage

**Pain Points:**
- SaaS training platforms don't meet security/compliance requirements
- Need a platform that fits into existing infrastructure and tooling

---

## 3. Key Concepts

### 3.1 Learning Path

A **Learning Path** is the top-level organizational unit. It represents a complete training journey on a specific topic (e.g., "Kubernetes Fundamentals", "CI/CD with GitLab", "Linux Administration").

- Each Learning Path is identified by a **directory containing a `phoebus.yaml` file** inside a Git repository
- A single Git repository can contain **one or many Learning Paths** (see [Repository Layouts](#41-repository-structure))
- A Learning Path contains an ordered sequence of **Modules**
- Learners enroll in Learning Paths and progress through them

### 3.2 Module

A **Module** is a self-contained unit of learning within a Learning Path. It covers a coherent topic or skill area (e.g., "Pod Basics", "Deployments and ReplicaSets").

- A Module contains an ordered sequence of **Steps**
- A Module may define **competencies** that will be acquired upon completion

### 3.3 Step

A **Step** is the atomic unit of content. It can be one of the following types:

| Step Type | Description |
|---|---|
| **Lesson** | A Markdown document with text, diagrams, embedded images, videos, and audio. Rich instructional content with media assets served from the platform's asset store. |
| **Quiz** | A set of questions (multiple choice, short answer) to validate understanding of concepts. |
| **Terminal Exercise** | An interactive, simulated terminal scenario. The learner is presented with a context (system state, command output) and must choose the correct command at each step from a set of proposals. The exercise progresses as a guided sequence — no real VM is involved. |
| **Code Exercise** | A code review / debugging challenge presented in a read-only code viewer (with syntax highlighting and file tree). The learner must identify problematic code sections, choose the correct fix from proposed patches (diffs), or both. No free-form code editing — the exercise is about understanding and analysis. |

#### Terminal Exercise — Detailed Concept

A terminal exercise simulates a multi-step command-line scenario:

1. The instructor defines a **sequence of steps**, each consisting of:
   - A **context**: the current state of the terminal (prompt, previous output, system description)
   - A set of **command proposals** (e.g., 3-5 commands to choose from)
   - The **correct command** and an explanation of why it's correct
   - The **simulated output** that would result from executing the correct command
   - Optional explanations for why incorrect choices are wrong

2. The learner experience:
   - Sees a terminal-like UI showing the current context
   - Chooses a command from the proposals
   - If correct: the simulated output is displayed, and the exercise advances to the next step
   - If incorrect: feedback is shown explaining why the choice is wrong, the learner retries
   - The exercise completes when all steps are successfully answered

3. This approach tests the learner's **understanding of which commands to use and in what order**, without requiring any infrastructure.

#### Code Exercise — Detailed Concept

A code exercise presents a code review or debugging challenge. It supports three modes, configurable per exercise:

**Mode A — Identify & Fix:** The learner must first identify the problematic lines in the code (by clicking/selecting them), then choose the correct patch from a set of proposed diffs.

**Mode B — Choose the Fix:** The learner is shown the code and directly presented with multiple proposed patches (unified diffs). They must choose the correct one.

**Mode C — Identify, then Fix:** Same as Mode A, but the identification and fix selection are presented as two distinct phases.

In all modes:
1. The instructor provides:
   - A **codebase** (multi-file, multi-directory) displayed in a read-only code viewer with syntax highlighting and a file tree
   - A **problem description** (what's wrong or what needs to change)
   - For modes A/C: the **target lines** the learner should identify
   - A set of **proposed patches** (unified diff format), one of which is correct
   - Explanations for each patch (why it's correct or why it's wrong)

2. The learner experience:
   - Sees the code in a viewer that looks like an IDE (file tree, syntax highlighting, line numbers)
   - Reads the problem description
   - Depending on the mode: clicks on suspicious lines, reviews diff proposals, selects the correct fix
   - Receives immediate feedback with explanations

### 3.4 Competency

A **Competency** is a skill or knowledge area that can be tracked across modules and learning paths. Competencies enable instructors to define what a learner should be able to do after completing specific content.

**Key definitions:**

- **Competencies provided by a Learning Path** = the union of all `competencies` declared across its modules.
- **Acquired competency** = a learner has acquired a competency when they have **completed** (100% progress) a Learning Path whose modules cover that competency.
- **Prerequisites met** = all competencies listed in a Learning Path's `prerequisites` field have been acquired by the learner.

Competencies serve two purposes:
1. **Prerequisite enforcement** — When a learner starts a path with unmet prerequisites, a confirmation popup warns them and offers navigation to paths that provide the missing competencies.
2. **Catalog discovery** — Learners can filter the catalog by competency and sort learning paths by competency dependency order (topological sort: paths with no prerequisites first, then paths whose prerequisites are covered by earlier paths).

---

## 4. Content Model (Content-as-Code)

### 4.1 Repository Structure

A Git repository registered in Phœbus can be **any standard Git repository** — it may contain application code, documentation, CI pipelines, etc. Phœbus simply looks for directories containing a `phoebus.yaml` file and treats each one as a Learning Path.

Two layouts are supported:

#### Single-path layout

If `phoebus.yaml` is at the **root** of the repository, the entire repo is one Learning Path:

```
learning-path-kubernetes/
├── phoebus.yaml              # Learning path metadata
├── 01-introduction/
│   ├── index.md               # Module description (front matter + content)
│   ├── 01-what-is-k8s.md      # Lesson (front matter + content)
│   ├── 02-setup-cluster.md    # Terminal exercise (front matter + scenario)
│   ├── 03-debug-deployment/
│   │   ├── instructions.md    # Code exercise (front matter + instructions)
│   │   └── codebase/          # Read-only code files for the exercise
│   │       ├── main.go
│   │       ├── go.mod
│   │       └── pkg/
│   │           └── handler.go
│   └── 04-quiz.md             # Quiz (front matter + content)
├── 02-workloads/
│   ├── index.md
│   └── ...
└── assets/
    └── diagrams/              # Shared images, videos, and media assets
```

#### Multi-path layout

If there is **no** `phoebus.yaml` at the root, Phœbus scans **immediate subdirectories** for `phoebus.yaml` files. Each matching subdirectory becomes a separate Learning Path. The repository can contain any other files or directories alongside them — they are simply ignored.

```
my-training-repo/                  # A normal Git repo
├── README.md                      # Ignored by Phœbus
├── .github/                       # CI/CD, ignored by Phœbus
├── src/                           # Application code, ignored
├── networking/                    # ← Learning Path 1
│   ├── phoebus.yaml
│   ├── 01-fundamentals/
│   │   ├── index.md
│   │   └── 01-what-is-a-network.md
│   └── 02-tcpip/
│       └── ...
├── ssh/                           # ← Learning Path 2
│   ├── phoebus.yaml
│   ├── 01-what-is-ssh/
│   │   ├── index.md
│   │   └── 01-understanding-ssh.md
│   └── ...
└── kubernetes/                    # ← Learning Path 3
    ├── phoebus.yaml
    └── ...
```

> **Note:** The two layouts are mutually exclusive. If a `phoebus.yaml` exists at the root, subdirectories are not scanned. This prevents ambiguity when a single-path repo also has nested `phoebus.yaml` files for other purposes.

**Asset management:** Binary assets (images, videos, PDFs) placed in `assets/` directories are automatically uploaded to the platform's asset store during synchronization. Relative paths in Markdown (`![](./assets/diagram.png)`) are transparently rewritten to API URLs. Assets are deduplicated by content hash and served with immutable HTTP caching. The asset store supports two backends: **filesystem** (default, for development) and **S3** (for production, compatible with any S3 service including MinIO). Maximum file size is configurable (default: 50 MB).

All metadata is embedded as **YAML front matter** in Markdown files (à la Docusaurus / Hugo), eliminating the need for separate metadata files. Each module directory contains an `index.md` with module metadata in its front matter. For simple steps (lessons, quizzes, terminal exercises), the step is a single `.md` file. For code exercises that reference a codebase, the step is a directory containing `instructions.md` and a `codebase/` directory.

### 4.2 Metadata Files

**`phoebus.yaml`** — Learning Path metadata:
```yaml
title: "Kubernetes Fundamentals"
description: "Learn Kubernetes from zero to deploying production workloads."
icon: "kubernetes"
tags: ["kubernetes", "containers", "orchestration"]
estimated_duration: "20h"
prerequisites:
  - "linux-fundamentals"
  - "containers-basics"
```

### 4.3 Module Front Matter

Module metadata is defined in the `index.md` file at the root of each module directory:

**Module** (`01-introduction/index.md`):
```markdown
---
title: "Introduction to Kubernetes"
description: "Understand what Kubernetes is and set up your first cluster."
competencies:
  - "k8s-cluster-setup"
  - "k8s-pod-basics"
---

# Introduction to Kubernetes

This module covers the fundamentals of Kubernetes...
```

### 4.4 Step Authoring Format

Step metadata is defined as YAML front matter (delimited by `---`) at the top of each Markdown file. The exercise logic (questions, scenarios, patches) is authored in the **Markdown body** using structured conventions — keeping the front matter light and the content readable.

#### Lessons

A lesson is pure Markdown content with minimal front matter:

**`01-what-is-k8s.md`**
```markdown
---
title: "What is Kubernetes?"
type: lesson
estimated_duration: "15m"
---

# What is Kubernetes?

Kubernetes is an open-source container orchestration platform...
```

#### Quizzes

Each question is a `##` heading with the question type in brackets. Choices use Markdown checkbox syntax (`- [x]` for correct, `- [ ]` for incorrect). Explanations are blockquotes after the choices. Short-answer questions use an indented code block for the expected pattern.

**`04-quiz.md`**
```markdown
---
title: "Kubernetes Basics"
type: quiz
estimated_duration: "10m"
---

# Kubernetes Basics

Test your understanding of the core concepts.

## [multiple-choice] What is the smallest deployable unit in Kubernetes?

- [ ] Container
- [x] Pod
- [ ] Deployment

> **Explanation:** A Pod is the smallest deployable unit in Kubernetes.
> It can contain one or more containers.

## [short-answer] Which command lists all running pods?

    kubectl get pods

> **Explanation:** `kubectl get pods` lists all pods in the current namespace.
```

**Authoring rules for quizzes:**
- `## [multiple-choice]` — Single or multi-select (determined by number of `[x]` answers)
- `## [short-answer]` — Free-text input validated against the indented code block (regex pattern)
- Explanation blockquotes (`>`) are shown to the learner after they submit their answer
- Questions are rendered in document order

#### Terminal Exercises

A terminal exercise is a guided, step-by-step scenario. Each step is a `##` heading. A `console` code block shows the prompt. Choices use checkbox syntax with the command in backticks, followed by a dash and the explanation. An `output` code block shows the simulated result after the correct choice.

**`02-setup-cluster.md`**
````markdown
---
title: "Set Up a Local Cluster"
type: terminal-exercise
estimated_duration: "15m"
---

# Set Up a Local Cluster

You are logged into a fresh Ubuntu 22.04 server.
You need to set up a local Kubernetes cluster using kubeadm.

## Step 1: Install the container runtime

You need to install the container runtime first.

```console
$ ▌
```

- [ ] `apt install docker.io` — Docker works but kubeadm recommends containerd as a standalone runtime.
- [x] `apt install containerd` — Containerd is the recommended container runtime for kubeadm clusters.
- [ ] `snap install microk8s` — MicroK8s is a different Kubernetes distribution, not kubeadm.

```output
Reading package lists... Done
Setting up containerd (1.6.20-0ubuntu1) ...
```

## Step 2: Initialize the cluster

Container runtime is installed. Now initialize the cluster.

```console
$ ▌
```

- [x] `kubeadm init --pod-network-cidr=10.244.0.0/16` — Initializes the cluster with a pod network CIDR compatible with Flannel.
- [ ] `kubeadm init` — Without --pod-network-cidr, the CNI plugin may not work correctly.
- [ ] `kubectl cluster-info` — The cluster doesn't exist yet, you need to initialize it first.

```output
[init] Using Kubernetes version: v1.28.0
...
Your Kubernetes control-plane has initialized successfully!
```
````

**Authoring rules for terminal exercises:**
- Each `## Step N` defines one step in the sequence
- Text before the choices is the context/instructions for that step
- The `console` code block is decorative (shows the prompt in the rendered UI)
- Choices: `- [x]` for the correct command, `- [ ]` for incorrect. Command in backticks, then ` — ` and the explanation
- The `output` code block defines the simulated terminal output displayed after the correct choice
- Steps are rendered in document order; the learner must complete each step before seeing the next

#### Code Exercises

A code exercise references a `codebase/` directory (real files displayed in a read-only viewer) and defines patches in the Markdown body. The front matter specifies the exercise mode and (for modes A/C) the target lines to identify.

**`03-debug-deployment/instructions.md`**
````markdown
---
title: "Fix the Health Check Handler"
type: code-exercise
mode: identify-and-fix
estimated_duration: "10m"
target:
  file: "pkg/handler.go"
  lines: [12, 13]
---

# Fix the Health Check Handler

The deployment is failing its liveness probe. The `HandleHealth` function
in `pkg/handler.go` is returning the wrong status code.

Find the problematic lines and choose the correct fix.

## Patches

### [x] Fix the status code and response body

The health endpoint should return 200 OK, not 500.

```diff
--- a/pkg/handler.go
+++ b/pkg/handler.go
@@ -11,3 +11,3 @@
 func HandleHealth(w http.ResponseWriter, r *http.Request) {
-    w.WriteHeader(http.StatusInternalServerError)
-    w.Write([]byte("error"))
+    w.WriteHeader(http.StatusOK)
+    w.Write([]byte("ok"))
 }
```

### [ ] Return 404 Not Found

404 Not Found is not appropriate for a health check endpoint.

```diff
--- a/pkg/handler.go
+++ b/pkg/handler.go
@@ -11,3 +11,3 @@
 func HandleHealth(w http.ResponseWriter, r *http.Request) {
-    w.WriteHeader(http.StatusInternalServerError)
-    w.Write([]byte("error"))
+    w.WriteHeader(http.StatusNotFound)
+    w.Write([]byte("not found"))
 }
```

### [ ] Panic instead

Never use panic for control flow — it will crash the server.

```diff
--- a/pkg/handler.go
+++ b/pkg/handler.go
@@ -11,3 +11,2 @@
 func HandleHealth(w http.ResponseWriter, r *http.Request) {
-    w.WriteHeader(http.StatusInternalServerError)
-    w.Write([]byte("error"))
+    panic("health check not implemented")
 }
```
````

**Authoring rules for code exercises:**
- `mode` in front matter: `identify-and-fix` (A), `choose-the-fix` (B), or `identify-then-fix` (C)
- `target` in front matter (modes A/C only): the file and line numbers the learner must identify
- The body starts with the problem description (free Markdown)
- `## Patches` section contains the proposed fixes
- Each patch is a `### [x]` (correct) or `### [ ]` (incorrect) heading with a label
- Text under the heading is the explanation (shown after the learner's choice)
- The `diff` code block contains the unified diff for the patch
- The `codebase/` directory alongside `instructions.md` contains the actual project files rendered in the code viewer

### 4.5 Content Synchronization

- Administrators register Git repositories in Phœbus and configure how they are cloned (SSH or HTTPS, with authentication if needed)
- Each registered repository is assigned a **unique webhook URL** containing a UUID (e.g., `https://phoebus.example.com/webhooks/{uuid}`). This URL is Git-provider-agnostic: any POST to it triggers a sync for the associated repository
- When the webhook is called, Phœbus pulls the latest changes from the configured branch (e.g., `main`) and updates the content live — no build or publish step required
- Content versioning is inherently handled by Git history
- Instructors can use branches and pull requests for content review before merging to `main`
- Phœbus supports **any Git hosting platform** (GitHub, GitLab, Gitea, Bitbucket, self-hosted, etc.) since it only relies on standard Git clone (SSH/HTTPS) and a generic webhook endpoint
- **Instance SSH Key:** At first startup, Phœbus generates a unique **Ed25519 SSH keypair** for the instance. The private key is stored encrypted (AES-256-GCM) in the database, and the public key is displayed on the repository management page so administrators can add it as a **deploy key** (read-only) on their Git repositories. This enables SSH-based clone without managing per-repo credentials
- **Synchronisation par hash de contenu (SHA-256) :** chaque niveau de la hiérarchie (learning path, module, step) possède un hash calculé à partir de son contenu et de ses métadonnées. Lors d'une re-synchronisation, Phœbus compare les hashs à chaque niveau et **ignore entièrement** les éléments inchangés (zéro écriture en base). Seuls les éléments modifiés sont mis à jour — résultat : une re-sync de contenu identique ne produit aucune écriture
- **Soft-delete généralisé :** les modules et learning paths supprimés du dépôt reçoivent un `deleted_at = NOW()` au lieu d'être physiquement supprimés (même comportement que les steps). Un contenu réapparu est automatiquement restauré (`deleted_at = NULL`). La progression des apprenants n'est **jamais** perdue

---

## 5. Use Cases

### UC-1: Learner Follows a Learning Path

1. Learner browses the catalog of available Learning Paths
2. Learner enrolls in a Learning Path
3. Learner progresses through Modules and Steps in order
4. For each Step:
   - **Lesson**: Learner reads the content; after scrolling through at least 75% of the lesson, a "Complete & Continue" button becomes enabled. Clicking it marks the step as completed and navigates to the next step
   - **Quiz**: Learner answers questions and gets immediate results
   - **Terminal Exercise**: Learner navigates a simulated terminal scenario, choosing the correct commands step by step
   - **Code Exercise**: Learner reviews code, identifies problems, and selects the correct patch
5. Learner tracks their progress on a personal dashboard

### UC-2: Instructor Creates a Learning Path

1. Instructor creates a new Git repository following the Phœbus content structure
2. Instructor writes `phoebus.yaml`, module `index.md` files, and step Markdown files using the structured authoring conventions
3. Instructor registers the repository in Phœbus
4. Content becomes immediately available to learners
5. Instructor iterates on content by committing to the repository

### UC-3: Instructor Updates Existing Content

1. Instructor edits Markdown files in the Git repository
2. Changes are pushed to `main` (directly or via PR after review)
3. Phœbus detects the changes and updates the content live
4. Learners see the updated content on their next page load
5. Learners who already completed the step are unaffected in their progress

### UC-4: Learner Completes a Terminal Exercise

1. Learner reaches a Terminal Exercise step
2. A terminal-like UI is displayed with the initial context (system state, prompt)
3. Learner reads the scenario and chooses a command from the proposals
4. If the choice is correct: simulated output is displayed, exercise advances to the next step
5. If the choice is incorrect: explanation is shown, learner retries
6. Exercise completes when all steps are successfully answered
7. Step is marked as completed

### UC-5: Learner Completes a Code Exercise

1. Learner reaches a Code Exercise step
2. A read-only code viewer is displayed with the project file tree, syntax highlighting, and line numbers
3. Learner reads the problem description
4. Depending on the exercise mode:
   - **Identify & Fix (A/C)**: Learner clicks on the lines they believe are problematic, then reviews proposed patches and selects the correct one
   - **Choose the Fix (B)**: Learner reviews proposed patches (unified diffs) and selects the correct one
5. Feedback is shown with explanations for correct and incorrect choices
6. Step is marked as completed

### UC-6: Administrator Deploys Phœbus

1. Administrator provisions infrastructure (servers, network)
2. Administrator deploys Phœbus using provided Docker Compose configuration or standalone binary
3. Administrator configures SSO integration (OIDC / LDAP)
4. Administrator registers Git repositories containing learning paths (SSH or HTTPS clone URL, credentials if needed)
5. Administrator configures the webhook URL (with UUID) on the Git hosting platform for each repository
6. Platform is ready for learners and instructors

### UC-7: Instructor Monitors Learner Progress

1. Instructor opens the analytics dashboard for a Learning Path
2. Instructor sees aggregated progress: how many learners completed each module/step
3. Instructor can drill down to individual learner progress
4. Instructor identifies common failure points (steps with low pass rates)
5. Instructor uses insights to improve content or provide targeted help

### UC-8: Learner Tracks Competencies

1. Learner views their competency dashboard
2. Dashboard shows acquired competencies mapped from completed learning paths (a competency is acquired when the learning path covering it is 100% completed)
3. Learner identifies skill gaps and discovers relevant Learning Paths
4. Progress is visible to the learner and optionally to their manager

### UC-9: Prerequisite Enforcement with Guided Navigation

1. Learner opens a Learning Path that has prerequisites (competencies the learner has not yet acquired)
2. When the learner clicks "Start Learning" or navigates to a step, a **confirmation popup** appears:
   - Lists the unmet prerequisite competencies
   - Warns that the content may assume prior knowledge
   - Offers two actions:
     - **"Continue anyway"** — dismisses the popup and lets the learner proceed (non-blocking)
     - **"Browse prerequisite paths"** — redirects to the Catalog page pre-filtered with the missing competencies, so the learner can find and complete the relevant paths first
3. The popup is shown only once per learning path per session (dismissed state stored client-side)
4. If all prerequisites are met, no popup is shown

---

## 6. Feature Breakdown

### 6.1 Content Management

| Feature | Description | Priority |
|---|---|---|
| Git-based content sync | Sync content from Git repos triggered by webhook | Must Have |
| Markdown rendering | Render Markdown with syntax highlighting, diagrams (Mermaid), embedded media | Must Have |
| Learning Path catalog | Browse and search available Learning Paths | Must Have |
| Content versioning | Track content changes via Git history | Must Have |
| Multi-repo support | Register and manage multiple Git repositories | Must Have |
| Content preview | Instructors preview content before merging to main | Should Have |

### 6.2 Learning Experience

| Feature | Description | Priority |
|---|---|---|
| Sequential progression | Learners progress through steps in order | Must Have |
| Unified lesson completion | Lessons use a single "Complete & Continue" button (enabled after 75% scroll) that marks the step as completed and navigates to the next step. On the last step, a congratulations modal is shown before redirecting to the path overview | Must Have |
| Progress tracking | Track completion of steps, modules, and learning paths. `in_progress` is a distinct third state shown in all analytics | Must Have |
| Personal dashboard | Learner sees their enrolled paths, progress, and competencies | Must Have |
| Competency-based catalog | Filter catalog by competency, sort by competency dependency order (topological) | Must Have |
| Prerequisite enforcement | Confirmation popup when starting a path with unmet prerequisites; link to prerequisite paths | Must Have |
| Light/Dark mode | Theme follows system preference by default; toggle in header to override. Preference persisted in `localStorage` | Must Have |
| Bookmarks & notes | Learners can bookmark steps and take personal notes | Should Have |
| Exercise reset | Learners can reset any exercise and start over, unlimited resets | Must Have |
| Onboarding tour | Interactive guided tour (React Joyride) on Dashboard and Catalog pages; auto-triggered on first visit, replayable via a "?" button in the header; tour completion state persisted in backend | Must Have |

### 6.3 Terminal Exercises (Simulated)

| Feature | Description | Priority |
|---|---|---|
| Terminal-like UI | Render a terminal-style interface with prompt, context, and command output | Must Have |
| Step-by-step scenario | Present a sequence of steps, each with command proposals | Must Have |
| Command selection | Learner chooses the correct command from a set of proposals | Must Have |
| Simulated output | Display the instructor-defined output after a correct choice | Must Have |
| Incorrect choice feedback | Show explanation when the learner selects a wrong command, allow retry | Must Have |
| Scenario defined in front matter | Terminal scenarios are authored as YAML in the step's front matter | Must Have |

### 6.4 Code Exercises

| Feature | Description | Priority |
|---|---|---|
| Read-only code viewer | Display code with syntax highlighting, line numbers, and file tree navigation | Must Have |
| Multi-file codebase | Support multi-file, multi-directory project structures in the `codebase/` directory | Must Have |
| Line identification (modes A/C) | Learner clicks on lines to identify problematic code sections | Must Have |
| Patch proposals | Display unified diffs as proposed fixes, learner selects the correct one | Must Have |
| Three exercise modes | Support identify-and-fix (A), choose-the-fix (B), and identify-then-fix (C) | Must Have |
| Feedback & explanations | Show per-patch explanations after the learner makes a choice | Must Have |
| Exercise defined in front matter | Code exercise metadata, patches, and target lines are authored in front matter | Must Have |

### 6.5 Quizzes

| Feature | Description | Priority |
|---|---|---|
| Multiple choice questions | Single and multi-select questions | Must Have |
| Short answer questions | Free-text answers with pattern matching validation | Should Have |
| Immediate feedback | Show correct answers and explanations after submission | Must Have |
| Quiz defined in YAML/Markdown | Quizzes are authored as code in the content repo | Must Have |

### 6.6 Analytics & Monitoring

| Feature | Description | Priority |
|---|---|---|
| Learner progress dashboard | Individual learner progress across all paths | Must Have |
| Instructor analytics | Aggregated stats per learning path, module, and step | Must Have |
| Competency tracking | Map completed modules to competencies | Should Have |
| Failure analysis | Identify steps with high failure rates | Should Have |
| Manager view | Managers see team-level progress (opt-in) | Could Have |

### 6.7 Administration

| Feature | Description | Priority |
|---|---|---|
| SSO integration | OIDC, LDAP, and reverse proxy header auth | Must Have |
| User management | Roles: learner, instructor, admin | Must Have |
| Git repo registration | Register repos (SSH/HTTPS clone URL, credentials) and generate webhook URLs | Must Have |
| Platform health monitoring | Dashboards for system health, resource usage | Should Have |

---

## 7. High-Level Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Phœbus Platform                      │
│                                                         │
│  ┌──────────┐  ┌──────────────┐  ┌───────────────────┐  │
│  │ Frontend  │  │   Backend    │  │  Content Syncer   │  │
│  │ (React)   │◄─┤   (Go)       │◄─┤  (Git watcher)    │  │
│  │           │  │              │  │                   │  │
│  │• Dashboard│  │• REST API    │  │• Webhook receiver │  │
│  │• Terminal │  │• Auth (SSO)  │  │  (UUID-based)     │  │
│  │  simulator│  │• Progress    │  │• Git clone        │  │
│  │• Code     │  │• Analytics   │  │  (SSH / HTTPS)    │  │
│  │  viewer   │  │              │  │• Markdown parser  │  │
│  └──────────┘  └──────┬───────┘  └─────────┬─────────┘  │
│                       │                   │             │
│                       ▼                   ▼             │
│                ┌──────────────┐  ┌──────────────┐       │
│                │  Database    │  │ Git Repos     │       │
│                │ (PostgreSQL) │  │ (Content)     │       │
│                └──────────────┘  └──────────────┘       │
└─────────────────────────────────────────────────────────┘
```

### Technology Choices

| Component | Technology | Rationale |
|---|---|---|
| Backend | Go | Performance, simplicity, strong DevOps ecosystem |
| Frontend | React + Ant Design | Rich ecosystem, enterprise UI components, wide adoption |
| Database | PostgreSQL | Robust, open source, excellent JSON support |
| Code Viewer | Monaco Editor (read-only mode) | VS Code engine, syntax highlighting, file tree |
| Content Format | Markdown + YAML | Developer-friendly, Git-native |
| Authentication | OIDC / LDAP | Enterprise SSO integration |

---

## 8. Non-Functional Requirements

### 8.1 Performance

- Content sync from Git should complete within seconds of a push
- Exercise interactions (command selection, patch selection) should feel instant (< 100ms)
- Platform should support at least 200 concurrent learners per instance

### 8.2 Security

- SSO/OIDC/LDAP for authentication; reverse proxy header auth for environments behind OAuth2 Proxy, Authelia, or Traefik Forward Auth; role-based access control for authorization
- Content repos can be private (SSH key or token-based Git access)
- Git credentials are encrypted at rest
- **Prévention XSS dans le Markdown :** le pipeline de rendu utilise `rehype-sanitize` (après `rehypeRaw`) avec un schéma strict — autorise les classes hljs/admonition/mermaid, bloque `<script>`, `<style>`, `<iframe>`, `<object>`, `<embed>`, `<form>`, `<textarea>`. Liste blanche de protocoles : `href` accepte http/https/mailto uniquement ; `src` accepte http/https uniquement (bloque `file://`, `javascript:`, `data:` dans les liens)
- **Assainissement SVG Mermaid :** DOMPurify avec `USE_PROFILES: { svg: true }` avant injection dans le DOM ; `securityLevel: 'strict'` sur Mermaid
- **En-têtes CSP (Content Security Policy) :** middleware backend configurant `default-src 'self'`, `script-src 'self' blob:` (blob pour les workers Monaco), `worker-src 'self' blob:`, `style-src 'self' 'unsafe-inline'` (styles inline Ant Design), `img-src 'self' data:`, `font-src 'self' data:`, `connect-src 'self'`, `frame-ancestors 'none'`
- **Autres en-têtes de sécurité :** `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: strict-origin-when-cross-origin`, `X-XSS-Protection: 0`

### 8.3 Reliability

- Stateless backend design for horizontal scalability
- Content sync failures should be surfaced to administrators

### 8.4 Operability

- Single-tenant deployment (one instance per organization)
- Deployable via Docker Compose or standalone binary
- Configuration via `configstore` (`github.com/ovh/configstore`) — supports environment variables and file tree backends
- Structured logging (JSON) for observability
- Prometheus metrics endpoint

---

## 9. Out of Scope (v1)

The following are explicitly **not** in the initial scope:

- Real VM/container lab environments (Terraform, bastion host, ttyrec)
- Free-form code editing with automated test execution
- Formal exam mode with proctoring
- Plagiarism detection
- Video content hosting / streaming
- Multi-tenant / SaaS deployment
- Mobile native applications
- Real-time collaborative editing
- LTI integration
- AI-powered tutoring assistant (future consideration)
- Certificate generation (future consideration)
- Gamification (badges, leaderboards)

---

## 10. Success Metrics

| Metric | Target |
|---|---|
| Content update lead time | < 5 minutes from Git push to learner visibility |
| Exercise completion rate | > 70% of enrolled learners complete at least one path |
| Instructor content velocity | An instructor can create a new module (3-5 steps) in < 1 day |
| Platform uptime | 99.5% during business hours |

---

## 11. Open Questions

| # | Question | Context |
|---|---|---|
| *(No open questions at this time)* | | |

---

## 12. Glossary

| Term | Definition |
|---|---|
| **Learning Path** | A complete training journey on a topic, identified by a directory containing a `phoebus.yaml` file inside a Git repository |
| **Module** | A section within a Learning Path covering a coherent topic |
| **Step** | The atomic unit of content: lesson, quiz, terminal exercise, or code exercise |
| **Terminal Exercise** | An interactive simulated terminal scenario where the learner chooses commands from proposals at each step |
| **Code Exercise** | A code review/debugging challenge where the learner identifies problematic code and/or selects the correct patch from proposed diffs |
| **Content-as-Code** | The practice of storing and managing training content using software development tools and workflows |
| **Competency** | A trackable skill or knowledge area acquired through completing modules |
