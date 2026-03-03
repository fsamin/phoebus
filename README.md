# 🔥 Phœbus

> **Phœbus** `/ˈfiːbəs/` — from Greek *Φοῖβος* (Phoîbos), meaning "bright, radiant". An epithet of Apollo, god of light, knowledge, music, and arts. A fitting name for a platform that illuminates the path to learning.
>
> Spelled with the **œ** ligature: **Phœbus**, not ~~Phoebus~~.

**Open-source e-learning platform for DevOps training.**

Phœbus follows a **content-as-code** philosophy: learning paths are authored in Markdown, stored in Git repositories, and synced automatically. The platform handles content parsing, progress tracking, exercise validation, and analytics — all from a single self-hosted binary.

![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)
![React](https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-17-4169E1?logo=postgresql&logoColor=white)
![License](https://img.shields.io/badge/License-Apache%202.0-blue)

---

## Features

- 📚 **Content as Code** — Author learning paths in Markdown with YAML front matter (Docusaurus-style)
- 🔄 **Git Sync** — Register Git repositories, auto-sync via webhooks or manual triggers
- 🧩 **4 Exercise Types** — Lessons, quizzes (multiple-choice & short-answer), terminal exercises, code exercises (identify-and-fix / choose-the-fix)
- 🧭 **Competency-Based Navigation** — Prerequisites & competencies on modules, topological sort in catalog, prerequisite enforcement popup
- 🖼️ **Asset Management** — Attach images, videos, and files to lessons with pluggable storage (local filesystem or S3/MinIO)
- 📊 **Progress Tracking** — Per-step completion, exercise attempts with server-side validation
- 📈 **Analytics** — Enrollment, completion rates, failure points, learner activity
- 🔐 **Authentication** — Local accounts, OIDC, LDAP, proxy auth with role-based access (learner / instructor / admin)
- 🎨 **Modern UI** — React 19 + Ant Design SPA with dark/light mode, syntax highlighting, Mermaid diagrams, and admonitions
- 📦 **Single Binary** — Frontend embedded via `go:embed`, deploy with Docker Compose

## Quick Start

```bash
git clone https://github.com/fsamin/phoebus.git
cd phoebus
docker compose up -d
```

Open [http://localhost:8080](http://localhost:8080) and login with `admin` / `admin`.

To load sample DevOps content, add the [phoebus-content-samples](https://github.com/fsamin/phoebus-content-samples) repository via the admin UI (*Repositories → Add*) with the clone URL:

```
https://github.com/fsamin/phoebus-content-samples.git
```

## Architecture

```
┌─────────────────────────────────────────────┐
│  React 19 SPA (Ant Design, React Router)    │
├─────────────────────────────────────────────┤
│  Go HTTP Server (Chi router, go:embed)      │
│  ├── REST API (/api/*)                      │
│  ├── Auth (JWT, OIDC, LDAP, Proxy)          │
│  ├── Content Syncer (git clone + parser)    │
│  ├── Asset Store (Filesystem or S3/MinIO)   │
│  └── Prometheus metrics (/metrics)          │
├─────────────────────────────────────────────┤
│  PostgreSQL 17                              │
└─────────────────────────────────────────────┘
```

## Content Format

Learning content lives in a Git repository with this structure:

```
my-learning-path/
├── phoebus.yaml              # Learning path metadata & prerequisites
├── assets/                   # Images, videos, and other files
│   └── diagram.png
├── 01-getting-started/
│   ├── index.md              # Module metadata (front matter + competencies)
│   ├── 01-introduction.md    # Lesson
│   ├── 02-quiz.md            # Quiz
│   └── 03-exercise/
│       ├── instructions.md   # Code exercise
│       └── codebase/         # Read-only starter files
│           └── main.go
└── 02-advanced/
    ├── index.md
    └── ...
```

The `phoebus.yaml` file defines path-level metadata and prerequisite competencies:

```yaml
title: "Containerization with Docker & Helm"
description: "Master containers from basics to Helm charts"
icon: "🐳"
tags: [docker, devops, containers]
estimated_duration: "12h"
prerequisites:
  - "linux-cli"
```

Each module `index.md` uses front matter to declare competencies:

```markdown
---
title: "Docker Fundamentals"
competencies:
  - "docker"
---

Module overview...
```

Each step file uses YAML front matter:

```markdown
---
title: "Introduction to Containers"
type: lesson
estimated_duration: "15m"
---

# Introduction to Containers

Your markdown content here...

:::tip
Admonitions are supported with :::tip, :::warning, :::danger, :::info, :::note syntax.
:::
```

See the [Detailed Specifications](docs/detailed-specifications.md) for the full content format reference.

## Configuration

Phœbus uses [configstore](https://github.com/ovh/configstore) for configuration. Each config key is a file in the config directory:

| File | Description | Default |
|------|-------------|---------|
| `database` | `url: postgres://user:pass@host:5432/db?sslmode=disable` | — (required) |
| `jwt` | `secret: your-jwt-secret` | — (required) |
| `http` | `port: 8080` | `8080` |
| `admin` | `username: admin` / `password: admin` | `admin:admin` |
| `auth` | `local_enabled: true` | `true` |
| `encryption` | `key: "32-byte-AES-key"` | — (optional) |
| `assets` | `backend: filesystem` or `s3` | `filesystem` |

### OIDC Authentication

```yaml
# config/auth
local_enabled: true
oidc:
  enabled: true
  issuer_url: https://accounts.google.com
  client_id: your-client-id
  client_secret: your-client-secret
  redirect_url: https://phoebus.example.com/api/auth/oidc/callback
  scopes: [openid, profile, email]
  claim_mapping:
    display_name: name
    email: email
    external_id: sub
```

### LDAP Authentication

```yaml
# config/auth
ldap:
  enabled: true
  server_url: ldap://ldap.example.com:389
  base_dn: dc=example,dc=com
  bind_dn: cn=admin,dc=example,dc=com
  bind_password: secret
  user_search_filter: "(uid=%s)"
  group_search_base: ou=groups,dc=example,dc=com
  group_search_filter: "(member=%s)"
  group_to_role:
    cn=admins,ou=groups,dc=example,dc=com: admin
    cn=instructors,ou=groups,dc=example,dc=com: instructor
```

### Proxy Authentication

For deployment behind a reverse proxy (e.g., Traefik, OAuth2 Proxy):

```yaml
# config/auth
proxy_auth:
  enabled: true
  header_user: X-Remote-User
  header_groups: X-Remote-Groups
  header_email: X-Remote-Email
  header_display_name: X-Remote-Display-Name
  default_role: learner
  group_to_role:
    admins: admin
    trainers: instructor
```

### S3 Asset Storage

```yaml
# config/assets
backend: s3
max_file_size: 52428800
filesystem:
  data_dir: /data/assets
s3:
  bucket: phoebus-assets
  region: us-east-1
  endpoint: https://s3.amazonaws.com
  access_key: AKIAIOSFODNN7EXAMPLE
  secret_key: wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
  prefix: ""
  force_path_style: false
```

For local development with MinIO, use `endpoint: http://localhost:9000` and `force_path_style: true`.

## Development

### Prerequisites

- Go 1.26+
- Node.js 24+ (LTS)
- Docker (for PostgreSQL)

### Backend

```bash
# Start PostgreSQL
docker compose up -d db

# Run the server
export CONFIGURATION_FROM="filetree:./config"
go run ./cmd/phoebus

# Run tests (starts ephemeral PostgreSQL container automatically)
go test ./internal/... -v
```

### Frontend

```bash
cd frontend
npm install
npm run dev    # Dev server with HMR on port 5173
npm run build  # Production build → frontend/dist/
```

### Build

```bash
# Build frontend + embed into Go binary
cd frontend && npm run build && cd ..
go build -o phoebus ./cmd/phoebus
```

## API Overview

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| `GET` | `/api/health` | — | Health check |
| `POST` | `/api/auth/login` | — | Local login |
| `POST` | `/api/auth/register` | — | Self-registration |
| `POST` | `/api/auth/ldap/login` | — | LDAP login |
| `GET` | `/api/auth/oidc/redirect` | — | OIDC redirect |
| `GET` | `/api/auth/oidc/callback` | — | OIDC callback |
| `GET` | `/api/auth/providers` | — | List auth providers |
| `POST` | `/api/webhooks/{uuid}` | — | Webhook trigger |
| `GET` | `/api/assets/{hash}` | — | Serve lesson asset |
| `GET` | `/api/me` | ✅ | Current user info |
| `GET` | `/api/me/dashboard` | ✅ | User dashboard |
| `POST` | `/api/auth/logout` | ✅ | Logout |
| `POST` | `/api/auth/refresh` | ✅ | Refresh JWT token |
| `GET` | `/api/learning-paths` | ✅ | List learning paths (with competencies & prereqs) |
| `GET` | `/api/learning-paths/{id}` | ✅ | Path detail with modules & steps |
| `GET` | `/api/learning-paths/{id}/steps/{stepId}` | ✅ | Step content & exercise data |
| `GET` | `/api/competencies` | ✅ | List all competencies |
| `GET` | `/api/progress` | ✅ | User progress |
| `POST` | `/api/progress` | ✅ | Update progress |
| `POST` | `/api/exercises/{stepId}/attempt` | ✅ | Submit exercise attempt |
| `POST` | `/api/exercises/{stepId}/reset` | ✅ | Reset exercise |
| `GET` | `/api/exercises/{stepId}/attempts` | ✅ | Get step attempts |
| `GET` | `/api/analytics/overview` | 👨‍🏫 | Platform analytics |
| `GET` | `/api/analytics/activity` | 👨‍🏫 | Activity timeline |
| `GET` | `/api/analytics/paths/{pathId}` | 👨‍🏫 | Path analytics |
| `GET` | `/api/analytics/paths/{pathId}/steps/{stepId}` | 👨‍🏫 | Step-level analytics |
| `GET` | `/api/analytics/learners/{learnerId}` | 👨‍🏫 | Learner analytics |
| `GET/POST` | `/api/admin/users` | 🔑 | List / create users |
| `PATCH` | `/api/admin/users/{userId}` | 🔑 | Update user role |
| `GET/POST` | `/api/admin/repos` | 🔑 | List / register Git repos |
| `GET/PUT/DELETE` | `/api/admin/repos/{repoId}` | 🔑 | Manage Git repository |
| `POST` | `/api/admin/repos/{repoId}/sync` | 🔑 | Trigger sync |
| `GET` | `/api/admin/repos/{repoId}/sync-logs` | 🔑 | Sync job history |
| `GET` | `/api/admin/health` | 🔑 | Detailed health check |
| `GET` | `/api/admin/ssh-public-key` | 🔑 | Instance SSH public key |
| `GET` | `/metrics` | — | Prometheus metrics |

Roles: ✅ = any authenticated user, 👨‍🏫 = instructor+, 🔑 = admin only

## Documentation

Full documentation is available via [MkDocs](https://www.mkdocs.org/):

```bash
pip install mkdocs-material
mkdocs serve
```

- [Product Specification](docs/product-specification.md) — Vision, personas, use cases
- [Technical Architecture](docs/technical-architecture.md) — Stack, schema, deployment
- [Detailed Specifications](docs/detailed-specifications.md) — Functional specs, UI wireframes, content format
- [Instructor Guide](docs/instructor-guide.md) — Content authoring guide for instructors

## License

Apache 2.0
