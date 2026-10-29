---
name: Phœbus
description: Self-hosted e-learning platform for DevOps training, content-as-code.
colors:
  phoebus-flame: "#ff7a45"
  phoebus-flame-deep: "#d4380d"
  on-flame-dark: "#141414"
  link-blue: "#0958d9"
  link-blue-dark: "#69b1ff"
  success-green: "#52c41a"
  warning-amber: "#faad14"
  danger-red: "#ff4d4f"
  note-violet: "#722ed1"
  header-navy: "#001529"
  text-primary: "#213547"
  text-secondary: "#666666"
  bg-page: "#ffffff"
  bg-elevated: "#fafafa"
  bg-login: "#f0f2f5"
  border-subtle: "#f0f0f0"
  border-input: "#d9d9d9"
  dark-bg-page: "#141414"
  dark-bg-container: "#1f1f1f"
  dark-bg-elevated: "#252525"
  dark-border-subtle: "#303030"
  terminal-bg-light: "#1a1a2e"
  terminal-bg-dark: "#1c1c1c"
  terminal-text: "#e0e0e0"
  terminal-command: "#00ff41"
  terminal-prompt-light: "#6ec6ff"
  terminal-prompt-dark: "#ff9a6c"
typography:
  body:
    fontFamily: "system-ui, Avenir, Helvetica, Arial, sans-serif"
    fontSize: "14px"
    fontWeight: 400
    lineHeight: 1.5
  title-page:
    fontFamily: "system-ui, Avenir, Helvetica, Arial, sans-serif"
    fontSize: "30px"
    fontWeight: 600
  title-brand:
    fontFamily: "system-ui, Avenir, Helvetica, Arial, sans-serif"
    fontSize: "20px"
    fontWeight: 600
  label-meta:
    fontFamily: "system-ui, Avenir, Helvetica, Arial, sans-serif"
    fontSize: "12px"
    fontWeight: 400
  code:
    fontFamily: "'JetBrains Mono', 'Fira Code', 'Cascadia Code', 'Consolas', monospace"
    fontSize: "13.5px"
    lineHeight: 1.6
rounded:
  xs: "4px"
  sm: "6px"
  md: "8px"
  full: "50%"
spacing:
  xs: "4px"
  sm: "8px"
  md: "12px"
  lg: "16px"
  xl: "24px"
  2xl: "48px"
components:
  button-primary:
    backgroundColor: "{colors.phoebus-flame-deep}"
    textColor: "{colors.bg-page}"
  button-primary-dark:
    backgroundColor: "{colors.phoebus-flame}"
    textColor: "{colors.on-flame-dark}"
    rounded: "{rounded.sm}"
    height: "32px"
  card:
    backgroundColor: "{colors.bg-page}"
    rounded: "{rounded.md}"
    padding: "24px"
  quiz-choice:
    backgroundColor: "{colors.bg-page}"
    rounded: "{rounded.sm}"
    padding: "8px 12px"
  admonition:
    backgroundColor: "{colors.bg-elevated}"
    border: "1px solid (semantic colour)"
    rounded: "{rounded.xs}"
    padding: "12px 16px"
  terminal-panel:
    backgroundColor: "{colors.terminal-bg-light}"
    textColor: "{colors.terminal-text}"
    rounded: "{rounded.md}"
    padding: "16px 20px"
  app-header:
    backgroundColor: "{colors.header-navy}"
    textColor: "{colors.bg-page}"
    height: "64px"
    padding: "0 24px"
---

<!-- Describes what ships as of 2026-10-05, after the learner-surface refinement pass (accessibility, contrast, small screens, copy, first run). Known gaps are listed under Do's and Don'ts. -->

# Design System: Phœbus

## Overview

**Creative North Star: "The Practice Console"**

Ant Design 6 with one brand accent, tuned for WCAG AA in both themes. `colorPrimary` is Deep Flame (#d4380d) in light mode and Phœbus Flame (#ff7a45) with dark text in dark mode; link, description, placeholder, status-text and Tag colours are overridden in `App.tsx` for contrast. There is no custom font and no shadow vocabulary of our own. The shell is a dark top bar with a horizontal menu; pages are built from Ant `Card`, `List`, `Collapse` and `Tag`.

The learning surfaces carry the product's identity: the simulated terminal, the Monaco code-exercise IDE, the quiz choices and the Markdown renderer. Lessons read in a 65ch column at 16px; quizzes and terminal exercises sit in a centred 900px column.

Theme-sensitive values are CSS custom properties in `frontend/src/index.css`, switched with `[data-theme="dark"]`, alongside Ant's algorithm switch.

**Key Characteristics:**
- Ant Design defaults, one orange accent, AA contrast in light and dark.
- Learner pages work from 320px up: steps get a rail and drawer on phones.
- Icons from `@ant-design/icons`, never emoji, in UI chrome.
- Distinct terminal and IDE surfaces for exercises.
- Styling mostly inline (`style={{}}`), with CSS variables for theme-sensitive values.

## Colors

One warm accent on Ant Design's neutral greys, with Ant's semantic status colours.

### Primary
- **Deep Flame** (#d4380d): light-mode `colorPrimary` and `--color-primary-fill`. White text on it passes AA; white on Phœbus Flame (2.6:1) does not.
- **Phœbus Flame** (#ff7a45): dark-mode `colorPrimary`, with #141414 text on filled surfaces (`--color-on-primary`). Also the header fire icon and the dark-mode header border.

### Secondary
- **Link Blue** (#0958d9 light, #69b1ff dark): Ant `colorLink`. #1890ff remains for info admonitions, selected quiz choice border, selected IDE lines and the resize-handle hover.

### Tertiary
- **Note Violet** (#722ed1): `:::note` admonitions only.

### Status
- **Success / Warning / Danger** fills: #52c41a, #faad14, #ff4d4f, for icons, borders and progress strokes.
- **Status text**: `--color-success-text` (#237804, dark #73d13d), `--color-danger-text` (#a8071a, dark #ff7875); Ant `colorWarningText` #874d00 in light mode. Use these, never the fills, for text.

### Neutral
- **Header Navy** (#001529 light, #1f1f1f dark) via `--color-bg-header`.
- **Text Primary** (#213547) / dark rgba(255,255,255,0.87).
- **Text Secondary** (#666666) / dark rgba(255,255,255,0.65); Ant descriptions at 0.6 alpha.
- **Page / container / elevated backgrounds**: #ffffff / #ffffff / #fafafa light, #141414 / #1f1f1f / #252525 dark.
- **Borders**: subtle #f0f0f0 (dark #303030), input #d9d9d9 (dark #434343).

### Terminal
- **Terminal Night** (#1a1a2e light theme, #1c1c1c dark theme): dark in both themes.
- **Command Green** (#00ff41) for chosen commands, #e0e0e0 / #ccc for text and output.
- **Prompt**: cyan #6ec6ff in light theme, orange #ff9a6c in dark theme.

### Named Rules
**The Variables-First Rule.** Theme-sensitive colours are defined once in `index.css` for both themes. Learner pages follow it; the terminal dots and feedback colours and the header icon are still inline hex.

## Typography

**Body Font:** system-ui (with Avenir, Helvetica, Arial, sans-serif)
**Mono Font:** JetBrains Mono (with Fira Code, Cascadia Code, Consolas, monospace). No web font is loaded, so it only renders when installed locally.

### Hierarchy
- **Page title** (600, 30px): one `Typography.Title level={1}` per page (Dashboard, Catalog, path, quiz, terminal and login). Lessons take their h1 from the Markdown.
- **Brand title** (600, 20px): plain text in the header, not a heading.
- **Body** (400, 14px, 1.5): Ant default. Lessons render at 16px in a 65ch column.
- **Meta / label** (400, 12px): secondary text, durations, step context ("Module · Step X of Y").
- **Code** (400, 13.5px, 1.6): Markdown `pre` blocks.

## Layout

- **App shell:** 64px top bar; content padded 24px × 48px, 16px on phones.
- **Reading column:** lessons 65ch; quizzes and terminal exercises 900px, centred.
- **Step view:** full-height split; a 280px sidebar, collapsible and resizable by pointer or arrow keys (180–500px). Below `md` it becomes a 48px rail opening a drawer. Code exercises take the full content area.
- **Grids:** Ant `Row` / `Col`, 16px gutters.
- **Spacing:** 4, 8, 12, 16, 24 and 48px, applied inline.

## Elevation & Depth

Flat, following Ant Design: borders and background tone, not shadows. The only custom shadow is the terminal panel (`0 4px 24px rgba(0,0,0,0.18)`, dark `rgba(0,0,0,0.5)`).

## Shapes

6px on buttons, inputs and quiz choices; 8px on cards, tables, code blocks and the terminal; 4px on admonitions and inline code; circles for the terminal dots.

## Interaction & Motion

- **Focus:** a 2px `--color-focus-ring` outline (Deep Flame light, Phœbus Flame dark) with 2px offset on links, focusable elements, buttons, menu items, radios and checkboxes.
- **Reduced motion:** `prefers-reduced-motion` cuts animations and transitions to near zero, and scripted scrolling uses `auto`.
- **No layout transitions:** width and height are never animated in our code (Ant internals excepted).
- **Navigating cards and rows** are real links (`.card-link`, `.row-link`).

## Components

### Buttons
- **Primary:** Deep Flame with white text (light); Phœbus Flame with #141414 text (dark).
- **Disabled on dark surfaces** (IDE bottom panel, terminal Run): explicit muted colours, since Ant's disabled grey vanishes there.

### Tags
- Ant `Tag` with preset colours, text shades overridden for AA.

### Cards, inputs, navigation
- Ant defaults. Top bar: `--color-bg-header`, 2px Phœbus Flame bottom border in dark mode, fire icon plus "Phœbus", dark horizontal menu, then tour, theme and user controls.
- **Step sidebar:** Ant inline `Menu` by module; status icons (green check, amber clock) or step-type icon; bold current step.

### Quiz choice
- Bordered row (1px, 6px radius, 8px 12px). Selected: light blue tint and blue border. After submit: green / red tint plus a text mark ("Correct answer", "Your answer"), so colour is never the only signal. Answer groups are named by the question text (`aria-labelledby`).

### Admonition
- 1px border in the semantic colour, elevated background, 4px radius, 12px 16px padding; title prefixed with an Ant icon (Bulb, Warning, Stop, InfoCircle, Edit), `aria-hidden`.

### Simulated terminal (signature)
- Dark window in both themes, red / yellow / green title-bar dots, 8px radius, terminal lift. History lines `~ › command` then output. Proposals listed under a separator: pick by tap, click or ↑↓, run with the **Run** button or Enter. A wrong answer strikes the command through in its explanation and in the list; nothing is auto-selected after an error. A polite live region announces selection and feedback.

### Code exercise IDE (signature)
- Monaco (`vs` / `vs-dark`) with a file explorer, resizable panels and a bottom panel for patches. Selected lines blue, target lines green, selected patch tinted with Phœbus Flame.

## Do's and Don'ts

### Do:
- **Do** spell the product name **Phœbus** with the œ ligature in every UI string.
- **Do** read theme-sensitive colours from the CSS variables in `index.css`.
- **Do** use the status-text colours, not the fills, for text.
- **Do** keep the terminal dark in both themes.
- **Do** give every learner page one h1 and every icon-only control an accessible name.
- **Do** use Ant Design components and tokens rather than rebuilding them.

### Don't:
- **Don't** put white text on Phœbus Flame (#ff7a45): 2.6:1.
- **Don't** use emoji as UI icons.
- **Don't** animate width, height or padding.
- **Don't** add new inline hex values; remaining drift is the terminal dots and feedback colours (#28c840, #ff6b6b), the header icon, and the light `--color-primary` (#ff7a45) used as "Executing…" text.
- **Don't** expect Mermaid diagrams to follow dark mode yet: the renderer is initialised with the light theme.
- **Don't** reuse `App.css`: leftover Vite scaffold, not imported.
