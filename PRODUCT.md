# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

- **Learner** (primary): a developer, sysadmin or ops engineer in an enterprise, upskilling on DevOps practices and tooling. Follows learning paths at their own pace, reads lessons, practises through quizzes, simulated terminal exercises and code review exercises, and tracks their own progress.
- **Instructor**: a tech lead, staff engineer or training lead who authors content in Git and Markdown with their own tools, and monitors learner progress and failure points through analytics.
- **Platform administrator**: deploys and operates a single-tenant instance, wires identity providers, registers repositories, manages users, sets the dashboard banner.

When a design choice opposes personas, the learner wins.

## Product Purpose

Phœbus is an open-source, self-hosted e-learning platform for DevOps and software engineering training. Content lives in Git repositories as Markdown and is synced automatically: what is in Git is what learners see. Success means learners build practical skills through hands-on exercises with immediate automated feedback, and instructors keep content current with the same workflow as code.

## Positioning

- Content-as-code: no CMS, no WYSIWYG editor, no publishing pipeline. Branch, edit, review, merge, sync.
- Self-assessment: no grading, no ranking, no instructor gatekeeping. Feedback serves the learner.
- Hands-on exercises rendered entirely in the browser (simulated terminal, read-only code viewer with proposed patches), with no infrastructure to provision.

## Operating Context

- Single-tenant, on-premise or company cloud deployment, single Go binary with the React SPA embedded.
- Authentication through local accounts, OIDC, LDAP or a reverse proxy; roles learner / instructor / admin.
- Learners work in the browser, often alongside their usual engineering tools; instructors author in their IDE and Git hosting, then use the UI for sync status and analytics.

## Capabilities and Constraints

- Hierarchy: learning path → module → step. Step types: lesson, quiz (multiple choice, short answer), terminal exercise, code exercise (identify-and-fix, choose-the-fix).
- Competencies and prerequisites on paths and modules; catalog ordered by topological sort; DAG view of path dependencies; prerequisite enforcement popup.
- Markdown rendering with syntax highlighting, Mermaid diagrams and admonitions; lesson assets (images, video, files).
- Progress tracking per step, recorded exercise attempts, reset of exercises.
- Analytics for instructors and admins: enrollment, completion, failure points, per-learner KPIs.
- Admin: repositories with owners and sync logs, users and roles, health, SSH public key, dashboard banner (info / warning / danger).
- Guided onboarding tour (react-joyride).
- Stack: React 19 + TypeScript + Vite, Ant Design 6, React Router 7, Monaco editor, @xyflow/react. Ant Design remains the component base.
- Dark and light modes must both be preserved.
- UI copy is in English.

## Brand Commitments

- The name is always spelled **Phœbus**, with the œ ligature, never "Phoebus" in user-facing text.

## Evidence on Hand

- Sample content: the public `phoebus-content-samples` repository (DevOps learning paths), usable to populate a local instance.
- Specifications: `docs/product-specification.md`, `docs/detailed-specifications.md` (includes UI wireframes), `docs/instructor-guide.md`.
- Playwright e2e suite under `e2e/`.
- No testimonials, customer references, usage figures or benchmarks exist; none must be invented.

## Product Principles

1. The learning experience comes first: reading a lesson and working an exercise must be focused and frictionless.
2. Feedback informs, it never grades: results, attempts and progress are framed for the learner's own iteration, not as scores to be judged on.
3. Content is authored elsewhere: the UI renders and organises Git content faithfully rather than offering editing.
4. Enterprise-ready and self-hosted: no dependency on external services at runtime.

## Accessibility & Inclusion

- Target: WCAG 2.2 AA, in both dark and light modes.
