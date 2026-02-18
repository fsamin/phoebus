# 🔥 Phœbus

> **Phœbus** `/ˈfiːbəs/` — from Greek *Φοῖβος* (Phoîbos), meaning "bright, radiant". An epithet of Apollo, god of light, knowledge, music, and arts. A fitting name for a platform that illuminates the path to learning.
>
> Spelled with the **œ** ligature: **Phœbus**, not ~~Phoebus~~.

**Open-source e-learning platform for DevOps training.**

Phœbus follows a **content-as-code** philosophy: learning paths are authored in Markdown, stored in Git repositories, and synced automatically. The platform handles content parsing, progress tracking, exercise validation, and analytics — all from a single self-hosted binary.

![Go](https://img.shields.io/badge/Go-1.24-00ADD8?logo=go&logoColor=white)
![React](https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-17-4169E1?logo=postgresql&logoColor=white)
![License](https://img.shields.io/badge/License-Apache%202.0-blue)

---

## Features

- 📚 **Content as Code** — Author learning paths in Markdown with YAML front matter (Docusaurus-style)
- 🔄 **Git Sync** — Register Git repositories, auto-sync via webhooks or manual triggers
- 🧩 **4 Exercise Types** — Lessons, quizzes (multiple-choice & short-answer), terminal exercises, code exercises (identify-and-fix / choose-the-fix)
- 📊 **Progress Tracking** — Per-step completion, exercise attempts with server-side validation
- 📈 **Analytics** — Enrollment, completion rates, failure points, learner activity
- 🔐 **Authentication** — Local accounts, OIDC, LDAP with role-based access (learner / instructor / admin)
- 🎨 **Modern UI** — React 19 + Ant Design SPA with syntax highlighting, Mermaid diagrams, and admonitions
- 📦 **Single Binary** — Frontend embedded via `go:embed`, deploy with Docker Compose

## Quick Start

```bash
git clone https://github.com/fsamin/phoebus.git
cd phoebus
docker compose up -d
```

Open [http://localhost:8080](http://localhost:8080) and login with `admin` / `admin`.

## Architecture

```
┌─────────────────────────────────────────────┐
│  React 19 SPA (Ant Design, React Router)    │
├─────────────────────────────────────────────┤
│  Go HTTP Server (Chi router, go:embed)      │
│  ├── REST API (/api/*)                      │
│  ├── Auth (JWT, OIDC, LDAP)                 │
│  ├── Content Syncer (git clone + parser)    │
│  └── Prometheus metrics (/metrics)          │
├─────────────────────────────────────────────┤
│  PostgreSQL 17                              │
└─────────────────────────────────────────────┘
```

## Content Format

Learning content lives in a Git repository with this structure:

```
my-learning-path/
├── phoebus.yaml              # Learning path metadata
├── 01-getting-started/
│   ├── index.md              # Module metadata (front matter)
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
```

## Development

### Prerequisites

- Go 1.24+
- Node.js 22+
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
| `GET` | `/api/learning-paths` | ✅ | List learning paths |
| `GET` | `/api/learning-paths/:id` | ✅ | Path detail with modules & steps |
| `GET` | `/api/learning-paths/:id/steps/:stepId` | ✅ | Step content & exercise data |
| `POST` | `/api/exercises/:stepId/attempt` | ✅ | Submit exercise attempt |
| `GET` | `/api/progress` | ✅ | User progress |
| `GET` | `/api/analytics/overview` | 👨‍🏫 | Platform analytics |
| `POST` | `/api/admin/repos` | 🔑 | Register Git repository |
| `POST` | `/api/webhooks/:uuid` | — | Webhook trigger (provider-agnostic) |

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

## License

Apache 2.0
