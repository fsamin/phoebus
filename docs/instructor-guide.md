# Phœbus — Guide de l'Instructeur

> Ce guide détaille comment créer et maintenir des parcours de formation (learning paths) sur la plateforme Phœbus. Tout le contenu est rédigé en **Markdown**, versionné dans des **dépôts Git**, et synchronisé automatiquement.

---

## Table des matières

1. [Principes fondamentaux](#1-principes-fondamentaux)
2. [Structure d'un dépôt de contenu](#2-structure-dun-dépôt-de-contenu)
3. [Le fichier `phoebus.yaml`](#3-le-fichier-phoebusyaml)
4. [Les modules (`index.md`)](#4-les-modules-indexmd)
5. [Les étapes (steps)](#5-les-étapes-steps)
   - [Leçon (lesson)](#51-leçon-lesson)
   - [Quiz](#52-quiz)
   - [Exercice terminal](#53-exercice-terminal-terminal-exercise)
   - [Exercice de code](#54-exercice-de-code-code-exercise)
6. [Markdown supporté](#6-markdown-supporté)
7. [Synchronisation et mise à jour](#7-synchronisation-et-mise-à-jour)
8. [Bonnes pratiques](#8-bonnes-pratiques)
9. [Référence rapide](#9-référence-rapide)

---

## 1. Principes fondamentaux

Phœbus adopte une approche **content-as-code** : les parcours de formation sont de simples fichiers Markdown organisés dans un dépôt Git. Cette approche apporte :

- **Versionnement** — chaque modification est tracée via Git
- **Collaboration** — les instructeurs travaillent avec des pull requests et des revues de code
- **Agilité** — mettre à jour un contenu = modifier un fichier + push
- **Reproductibilité** — le contenu est toujours dans un état connu

Un dépôt Git = un learning path. Chaque sous-dossier est un module, et chaque fichier `.md` est une étape (step).

---

## 2. Structure d'un dépôt de contenu

Voici la structure complète attendue par Phœbus :

```
mon-learning-path/
├── phoebus.yaml                          # ① Métadonnées du learning path
│
├── 01-premier-module/                    # ② Module (préfixe numérique = ordre)
│   ├── index.md                          #    Métadonnées du module
│   ├── 01-introduction.md                #    Étape : leçon
│   ├── 02-commandes-essentielles.md      #    Étape : leçon
│   ├── 03-exercice-navigation.md         #    Étape : exercice terminal
│   ├── 04-quiz.md                        #    Étape : quiz
│   └── 05-fix-config/                    #    Étape : exercice de code (dossier)
│       ├── instructions.md               #        Instructions + patches
│       └── codebase/                     #        Fichiers de code à analyser
│           ├── config.yaml
│           └── main.go
│
├── 02-deuxieme-module/
│   ├── index.md
│   ├── 01-theorie.md
│   └── 02-quiz.md
│
└── 03-troisieme-module/
    ├── index.md
    └── ...
```

### Règles d'ordonnancement

L'ordre des modules et des étapes est déterminé par le **préfixe numérique** du nom de fichier ou dossier :

- `01-introduction.md` sera affiché avant `02-commandes.md`
- `01-basics/` sera affiché avant `02-advanced/`
- Les fichiers sans préfixe numérique sont triés alphabétiquement après les fichiers numérotés
- Le préfixe numérique est **retiré** du nom affiché dans la plateforme

> 💡 **Convention recommandée** : utilisez des préfixes à deux chiffres (`01-`, `02-`, ..., `99-`) pour garder un ordonnancement clair.

---

## 3. Le fichier `phoebus.yaml`

Le fichier `phoebus.yaml` à la racine du dépôt décrit les métadonnées du learning path. C'est le point d'entrée que Phœbus utilise pour identifier et indexer le parcours.

### Champs disponibles

| Champ | Type | Obligatoire | Description |
|-------|------|:-----------:|-------------|
| `title` | string | ✅ | Titre du learning path |
| `description` | string | | Description affichée sur la carte du parcours |
| `icon` | string | | Icône (emoji ou identifiant) |
| `tags` | string[] | | Tags pour le filtrage et la recherche |
| `estimated_duration` | string | | Durée estimée (ex : `"12h"`, `"2h30m"`) |
| `prerequisites` | string[] | | Titres des learning paths prérequis |

### Exemple complet

Tiré du parcours [Linux Fundamentals](https://github.com/fsamin/phoebus-content-samples) :

```yaml
title: "Linux Fundamentals"
description: "Master the Linux command line, SSH remote access, and GPG encryption. The essential foundation for any DevOps engineer."
icon: "linux"
tags: ["linux", "ssh", "gpg", "security", "cli"]
estimated_duration: "12h"
prerequisites: []
```

Autre exemple, avec des prérequis :

```yaml
title: "Containerization with Docker & Helm"
description: "Build, ship, and run applications with Docker. Package Kubernetes applications with Helm charts."
icon: "docker"
tags: ["docker", "containers", "helm", "packaging", "devops"]
estimated_duration: "14h"
prerequisites:
  - "linux-fundamentals"
  - "git-mastery"
```

---

## 4. Les modules (`index.md`)

Chaque dossier de module **doit** contenir un fichier `index.md` qui décrit le module. Les métadonnées sont écrites en **front matter YAML** (délimité par `---`).

### Champs disponibles

| Champ | Type | Obligatoire | Description |
|-------|------|:-----------:|-------------|
| `title` | string | ✅ | Titre du module |
| `description` | string | | Description du module |
| `competencies` | string[] | | Compétences couvertes par le module |

### Exemple

```markdown
---
title: "Linux Basics"
description: "Navigate the filesystem, master essential commands, and understand file permissions."
competencies:
  - "linux-filesystem"
  - "linux-commands"
  - "linux-permissions"
---

# Linux Basics

This module covers the essential Linux skills every DevOps engineer needs:
navigating the filesystem, manipulating files, and understanding permissions.
By the end of this module, you will be comfortable working in a Linux terminal.
```

Le contenu Markdown après le front matter est affiché comme introduction du module.

---

## 5. Les étapes (steps)

Chaque étape est un fichier `.md` dans un dossier de module. Le front matter définit le type d'étape.

### Front matter commun à toutes les étapes

| Champ | Type | Obligatoire | Description |
|-------|------|:-----------:|-------------|
| `title` | string | ✅ | Titre de l'étape |
| `type` | string | ✅ | Type : `lesson`, `quiz`, `terminal-exercise`, `code-exercise` |
| `estimated_duration` | string | | Durée estimée (ex : `"15m"`, `"1h"`) |

---

### 5.1 Leçon (`lesson`)

La leçon est le type le plus simple : du contenu Markdown pur, affiché tel quel au learner.

#### Exemple : `01-filesystem.md`

```markdown
---
title: "The Linux Filesystem"
type: lesson
estimated_duration: "20m"
---

# The Linux Filesystem

## Everything is a File

In Linux, everything is represented as a file — regular files, directories,
devices, and even processes.

## The Filesystem Hierarchy

| Path | Purpose |
|------|---------|
| `/` | Root of the filesystem |
| `/home` | User home directories |
| `/etc` | System configuration files |
| `/var` | Variable data (logs, databases) |
| `/tmp` | Temporary files |

## Navigation Commands

\`\`\`bash
pwd          # Print working directory
ls -la       # List all files with details
cd /etc      # Change directory
\`\`\`
```

#### Ce qui est supporté dans les leçons

- Tout le Markdown standard (titres, listes, tableaux, emphase, liens, images)
- Blocs de code avec coloration syntaxique (spécifiez le langage : ` ```bash`, ` ```yaml`, etc.)
- Liens vers des images (URLs `http://` et `https://` uniquement)

> ⚠️ **Sécurité** : les URLs `file://` et `javascript:` sont bloquées. Seuls les protocoles `http`, `https` et `mailto` sont autorisés dans les liens.

---

### 5.2 Quiz

Le quiz permet d'évaluer la compréhension du learner avec des questions à choix multiples ou à réponse courte.

#### Syntaxe

Le corps du fichier Markdown utilise une syntaxe spéciale :

```
## [type-de-question] Texte de la question

(options ou pattern)

> Explication affichée après soumission.
```

#### Types de questions

| Type | Syntaxe du heading | Description |
|------|-------------------|-------------|
| Choix multiple | `## [multiple-choice]` | Le learner sélectionne une ou plusieurs réponses |
| Réponse courte | `## [short-answer]` | Le learner tape une réponse libre |

#### Question à choix multiple

Les réponses sont des listes à puces avec des cases à cocher :

- `- [x]` = réponse **correcte**
- `- [ ]` = réponse **incorrecte**

```markdown
## [multiple-choice] What does the `/etc` directory contain?

- [ ] User home directories
- [x] System configuration files
- [ ] Temporary files
- [ ] Device files

> **Explanation:** `/etc` contains system-wide configuration files.
> Examples include `/etc/ssh/sshd_config` and `/etc/hosts`.
```

> 💡 Vous pouvez mettre **plusieurs** `[x]` pour créer une question à sélection multiple (le learner devra cocher toutes les bonnes réponses).

#### Question à réponse courte

La réponse attendue est un **pattern regex** écrit dans un bloc de code indenté (4 espaces) :

```markdown
## [short-answer] Which command follows a log file in real-time?

    tail -f

> **Explanation:** `tail -f` keeps the file open and displays new content
> as it's appended.
```

Le pattern est évalué comme une **expression régulière** (insensible à la casse). Vous pouvez donc utiliser :

- `tail -f` — correspondance exacte
- `tail\s+(-f|--follow)` — accepte `-f` ou `--follow`
- `mkdir\s+-p\s+.*` — accepte n'importe quel chemin après `mkdir -p`

> ⚠️ **Le pattern regex est validé au moment de la synchronisation.** Une regex invalide fera échouer le sync.

#### Exemple complet de quiz

Tiré du parcours [Linux Fundamentals](https://github.com/fsamin/phoebus-content-samples) :

```markdown
---
title: "Linux Basics Quiz"
type: quiz
estimated_duration: "10m"
---

# Linux Basics Quiz

## [multiple-choice] What does the `/etc` directory contain?

- [ ] User home directories
- [x] System configuration files
- [ ] Temporary files
- [ ] Device files

> **Explanation:** `/etc` (et cetera) contains system-wide configuration files.

## [multiple-choice] What permission octal value represents `rwxr-xr-x`?

- [ ] 777
- [x] 755
- [ ] 644
- [ ] 700

> **Explanation:** `rwx` = 7, `r-x` = 5, `r-x` = 5. So the octal value is 755.

## [short-answer] Which command follows a log file in real-time?

    tail -f

> **Explanation:** `tail -f` (follow) keeps the file open and displays
> new content as it's appended.

## [short-answer] What command creates a nested directory structure in one command?

    mkdir -p /opt/myapp/config/templates

> **Explanation:** The `-p` flag creates all intermediate directories as needed.
```

---

### 5.3 Exercice terminal (`terminal-exercise`)

L'exercice terminal simule un environnement de ligne de commande. Le learner doit sélectionner la bonne commande à chaque étape, dans un terminal interactif stylisé.

#### Syntaxe

```
(texte d'introduction avant le premier step)

## Step N: Titre de l'étape

Contexte et instructions.

\`\`\`console
$ ▌
\`\`\`

- [x] `commande-correcte` — Explication de pourquoi c'est correct.
- [ ] `commande-incorrecte` — Explication de pourquoi c'est incorrect.
- [ ] `autre-commande-incorrecte` — Autre explication.

\`\`\`output
sortie simulée après la bonne commande
\`\`\`
```

#### Règles

| Élément | Règle |
|---------|-------|
| Introduction | Texte libre avant le premier `## Step` |
| Steps | Numérotés avec `## Step N` |
| Prompt | Bloc ` ```console ` avec `$ ▌` |
| Propositions | `- [x]` (correcte) ou `- [ ]` (incorrecte), commande entre backticks |
| Explication | Après ` — ` (tiret cadratin) dans chaque proposition |
| Output | Bloc ` ```output ` affiché après la bonne réponse |
| Exactement 1 `[x]` | ✅ **Une et une seule** réponse correcte par step |

#### Exemple complet

Tiré du parcours [Linux Fundamentals — Navigate the Filesystem](https://github.com/fsamin/phoebus-content-samples) :

```markdown
---
title: "Navigate the Filesystem"
type: terminal-exercise
estimated_duration: "10m"
---

# Navigate the Filesystem

You've just connected to a freshly provisioned Linux server.
Your first task is to explore the system.

## Step 1: Find your current location

Where are you in the filesystem? Print your current working directory.

\`\`\`console
$ ▌
\`\`\`

- [x] `pwd` — Prints the absolute path of the current working directory.
- [ ] `ls` — Lists directory contents but doesn't show your location.
- [ ] `whoami` — Shows your username, not your location.

\`\`\`output
/home/devops
\`\`\`

## Step 2: List all files including hidden ones

Your home directory might contain hidden configuration files (dotfiles).

\`\`\`console
$ ▌
\`\`\`

- [ ] `ls` — Only shows non-hidden files.
- [x] `ls -la` — Shows all files including hidden ones, with details.
- [ ] `ls -l` — Shows details but skips hidden files.

\`\`\`output
total 24
drwxr-xr-x 3 devops devops 4096 Jan 15 10:30 .
drwxr-xr-x 4 root   root   4096 Jan 15 10:00 ..
-rw-r--r-- 1 devops devops  220 Jan 15 10:00 .bash_logout
-rw-r--r-- 1 devops devops 3771 Jan 15 10:00 .bashrc
drwx------ 2 devops devops 4096 Jan 15 10:30 .ssh
\`\`\`

## Step 3: Find configuration files

Locate all `.conf` files under `/etc` that contain the word "listen".

\`\`\`console
$ ▌
\`\`\`

- [ ] `find /etc -name "*.conf"` — Finds conf files but doesn't search contents.
- [ ] `ls /etc/*.conf` — Only lists files in /etc, not subdirectories.
- [x] `grep -rl "listen" /etc/*.conf 2>/dev/null` — Recursively searches for "listen" in .conf files.

\`\`\`output
/etc/ssh/sshd_config.conf
\`\`\`
```

#### Rendu dans Phœbus

L'exercice terminal s'affiche comme un vrai terminal : le learner voit le prompt, les propositions de commandes, et la sortie simulée s'affiche progressivement après chaque bonne réponse.

---

### 5.4 Exercice de code (`code-exercise`)

L'exercice de code est le type le plus riche. Il présente au learner un **codebase complet** dans un éditeur Monaco (le même que VS Code), et lui demande d'**identifier un problème** puis de **sélectionner le bon correctif** parmi plusieurs patches (diffs).

#### Structure sur le filesystem

Contrairement aux autres types, l'exercice de code utilise un **dossier** au lieu d'un simple fichier :

```
03-fix-dockerfile/
├── instructions.md      # Instructions, description, patches
└── codebase/            # Fichiers de code affichés dans l'éditeur
    ├── Dockerfile
    ├── main.go
    └── go.mod
```

> ⚠️ Phœbus détecte automatiquement qu'une étape est un exercice de code quand un **dossier** contient un fichier `instructions.md`.

#### Front matter de `instructions.md`

| Champ | Type | Obligatoire | Description |
|-------|------|:-----------:|-------------|
| `title` | string | ✅ | Titre de l'exercice |
| `type` | string | ✅ | Doit être `code-exercise` |
| `mode` | string | ✅ | Mode de l'exercice (voir ci-dessous) |
| `estimated_duration` | string | | Durée estimée |
| `target` | object | ✅* | Fichier et lignes contenant le problème |
| `target.file` | string | ✅* | Chemin relatif du fichier dans `codebase/` |
| `target.lines` | int[] | ✅* | Numéros des lignes problématiques |

\* Obligatoire pour le mode `identify-and-fix`.

#### Modes d'exercice

| Mode | Phase 1 | Phase 2 | Description |
|------|---------|---------|-------------|
| `identify-and-fix` | Identifier les lignes problématiques | Choisir le bon patch | Le learner doit d'abord cliquer sur les bonnes lignes, puis sélectionner le diff correct |

#### Syntaxe de `instructions.md`

```
(description libre du problème en Markdown)

## Patches

### [x] Titre du patch correct

Explication de pourquoi c'est la bonne solution.

\`\`\`diff
--- a/Dockerfile
+++ b/Dockerfile
@@ -1,8 +1,14 @@
-FROM golang:1.22
+FROM golang:1.22-alpine AS builder
 ...
\`\`\`

### [ ] Titre du patch incorrect

Explication de pourquoi cette approche ne fonctionne pas.

\`\`\`diff
--- a/Dockerfile
+++ b/Dockerfile
@@ -7,3 +7,3 @@
-USER root
+USER nobody
\`\`\`
```

#### Règles

| Élément | Règle |
|---------|-------|
| Section Patches | Commence par `## Patches` |
| Patches | Délimités par `### [x]` (correct) ou `### [ ]` (incorrect) |
| Diff | Bloc ` ```diff ` au format **unified diff** |
| Exactement 1 `[x]` | ✅ **Un et un seul** patch correct |
| Codebase | Tous les fichiers texte dans `codebase/` (les binaires sont ignorés) |

#### Exemple complet

Tiré du parcours [Containerization — Fix the Dockerfile](https://github.com/fsamin/phoebus-content-samples) :

**`03-fix-dockerfile/instructions.md`** :

```markdown
---
title: "Fix the Dockerfile"
type: code-exercise
mode: identify-and-fix
estimated_duration: "10m"
target:
  file: "Dockerfile"
  lines: [3, 8]
---

# Fix the Dockerfile

A teammate wrote a Dockerfile for a Go application, but it has several issues
that make the image insecure and bloated. The image is 1.2 GB instead of the
expected ~15 MB.

Review the Dockerfile and identify the problems.

## Patches

### [x] Use multi-stage build and run as non-root

The Dockerfile should use a multi-stage build to separate compilation from
runtime, and the final image should not run as root.

\`\`\`diff
--- a/Dockerfile
+++ b/Dockerfile
@@ -1,10 +1,16 @@
-FROM golang:1.22
-WORKDIR /app
-COPY . .
-RUN go build -o server .
-EXPOSE 8080
-ENV GIN_MODE=release
-USER root
-CMD ["./server"]
+FROM golang:1.22-alpine AS builder
+WORKDIR /src
+COPY go.mod go.sum ./
+RUN go mod download
+COPY . .
+RUN CGO_ENABLED=0 go build -o /server .
+
+FROM alpine:3.19
+RUN addgroup -S app && adduser -S app -G app
+COPY --from=builder /server /server
+EXPOSE 8080
+ENV GIN_MODE=release
+USER app
+CMD ["/server"]
\`\`\`

### [ ] Just change the USER directive

Changing `USER root` to `USER nobody` fixes the security issue but the image
is still bloated because the entire Go toolchain is included.

\`\`\`diff
--- a/Dockerfile
+++ b/Dockerfile
@@ -7,3 +7,3 @@
 ENV GIN_MODE=release
-USER root
+USER nobody
 CMD ["./server"]
\`\`\`

### [ ] Add .dockerignore only

A `.dockerignore` file helps but doesn't solve the fundamental problem of
including the Go toolchain in the final image.

\`\`\`diff
--- a/Dockerfile
+++ b/Dockerfile
@@ -1,4 +1,4 @@
-FROM golang:1.22
+FROM golang:1.22-alpine
 WORKDIR /app
 COPY . .
 RUN go build -o server .
\`\`\`
```

**`03-fix-dockerfile/codebase/Dockerfile`** :

```dockerfile
FROM golang:1.22
WORKDIR /app
COPY . .
RUN go build -o server .
EXPOSE 8080
ENV GIN_MODE=release
USER root
CMD ["./server"]
```

#### Rendu dans Phœbus

1. **Phase « Identify »** — L'éditeur Monaco affiche le codebase. Le learner clique sur les lignes qu'il pense problématiques (les lignes définies dans `target.lines`).
2. **Phase « Fix »** — Les patches proposés s'affichent en tant que diffs. Le learner sélectionne celui qu'il pense correct. Le diff s'affiche dans un éditeur de comparaison (diff viewer).

---

## 6. Markdown supporté

Phœbus utilise un moteur de rendu Markdown complet. Voici ce qui est supporté dans les leçons :

### Syntaxe de base

| Élément | Syntaxe |
|---------|---------|
| Titres | `# H1`, `## H2`, `### H3`, etc. |
| Gras | `**texte en gras**` |
| Italique | `*texte en italique*` |
| Code inline | `` `code` `` |
| Lien | `[texte](https://url.com)` |
| Image | `![alt](https://url.com/image.png)` |
| Liste non ordonnée | `- item` ou `* item` |
| Liste ordonnée | `1. item` |
| Blockquote | `> citation` |
| Ligne horizontale | `---` |

### Blocs de code

Utilisez les triple backticks avec le langage pour la coloration syntaxique :

````markdown
```go
func main() {
    fmt.Println("Hello, World!")
}
```
````

Langages supportés : `bash`, `sh`, `go`, `python`, `javascript`, `typescript`, `yaml`, `json`, `dockerfile`, `hcl`, `sql`, `html`, `css`, et plus.

### Tableaux

```markdown
| Colonne 1 | Colonne 2 | Colonne 3 |
|-----------|-----------|-----------|
| valeur    | valeur    | valeur    |
```

### Protocoles autorisés

Pour des raisons de sécurité, seuls ces protocoles sont acceptés dans les liens et images :

| Attribut | Protocoles autorisés |
|----------|---------------------|
| `href` (liens) | `http`, `https`, `mailto` |
| `src` (images) | `http`, `https` |

Les URLs `file://`, `javascript:` et `data:` sont **bloquées**.

---

## 7. Synchronisation et mise à jour

### Ajouter un dépôt de contenu

1. Connectez-vous en tant qu'**administrateur** sur Phœbus
2. Allez dans **Admin → Repositories**
3. Ajoutez l'URL de votre dépôt Git (HTTPS ou SSH)
4. Lancez la synchronisation

### Synchronisation intelligente (hash-based)

Phœbus utilise un système de **hash SHA-256** pour optimiser la synchronisation :

- Chaque étape est hashée (titre + type + durée + contenu + exercice)
- Chaque module est hashé (métadonnées + hash des étapes)
- Chaque learning path est hashé (métadonnées + hash des modules)

**Conséquences pratiques :**

| Situation | Comportement |
|-----------|-------------|
| Contenu inchangé | ⏭ Ignoré (aucune écriture en DB) |
| Contenu modifié | ✏️ Mis à jour, progression du learner **conservée** |
| Nouveau contenu | ✅ Ajouté |
| Contenu supprimé | 🗑 Soft-delete (la progression des learners est préservée) |
| Contenu réapparu | ♻️ Restauré automatiquement |

> 💡 **La progression des apprenants n'est jamais perdue** lors d'une re-synchronisation, même si le contenu évolue.

### Webhook pour la synchronisation automatique

Vous pouvez configurer un webhook Git (GitHub, GitLab, Bitbucket) pour déclencher la synchronisation automatiquement à chaque push. L'URL du webhook est disponible dans l'interface d'administration.

---

## 8. Bonnes pratiques

### Organisation du contenu

- **Un dépôt = un learning path** — Ne mélangez pas plusieurs parcours dans un même dépôt
- **Modules de 3 à 7 étapes** — Suffisamment pour couvrir un sujet, pas trop pour ne pas décourager
- **Alterner les types** — Leçon → Exercice → Quiz pour maintenir l'engagement
- **Terminer chaque module par un quiz** — Pour valider les acquis

### Rédaction des leçons

- Allez à l'essentiel, évitez les paragraphes trop longs
- Utilisez des **tableaux** pour les référence rapide
- Incluez des **blocs de code** avec le langage pour la coloration
- Utilisez des **listes** plutôt que des paragraphes pour les séquences d'instructions

### Rédaction des quiz

- **3 à 6 questions par quiz** est un bon équilibre
- Mélangez `multiple-choice` et `short-answer`
- Écrivez des **explications** pour chaque question (`>`) — c'est le moment pédagogique
- Pour les `short-answer`, utilisez des regex souples : `mkdir\s+-p` plutôt que `mkdir -p /exact/path`

### Rédaction des exercices terminal

- **3 à 5 steps** par exercice
- Chaque step doit être **auto-suffisant** avec assez de contexte
- Proposez **3 commandes** par step (1 correcte + 2 incorrectes)
- Les commandes incorrectes doivent être **plausibles** (erreurs courantes de débutants)
- Incluez toujours un bloc `output` — il aide le learner à visualiser le résultat

### Rédaction des exercices de code

- Le `codebase/` doit être **minimal et réaliste** (pas de fichier inutile)
- Le fichier `target.file` doit correspondre exactement à un fichier dans `codebase/`
- Les `target.lines` doivent pointer les lignes réellement problématiques
- Proposez **3 patches** (1 correct + 2 incorrects) avec des approches différentes
- Les patches incorrects doivent représenter des **erreurs courantes** (fix partiel, mauvaise approche)
- Les diffs doivent être au format **unified diff** valide

### Workflow Git recommandé

```
main
 └── feature/update-module-2
      ├── Modifier le contenu
      ├── Commit + Push
      ├── Pull Request + Review par un pair
      └── Merge → Sync automatique via webhook
```

---

## 9. Référence rapide

### Types d'étapes

| Type | Fichier | Interactif | Description |
|------|---------|:----------:|-------------|
| `lesson` | `*.md` | ❌ | Contenu Markdown affiché |
| `quiz` | `*.md` | ✅ | Questions à choix multiples et réponse courte |
| `terminal-exercise` | `*.md` | ✅ | Simulation de terminal avec choix de commandes |
| `code-exercise` | `dossier/instructions.md` + `codebase/` | ✅ | Analyse de code + sélection de patch |

### Syntaxe des questions quiz

```
## [multiple-choice] Texte de la question
- [x] Bonne réponse
- [ ] Mauvaise réponse
> Explication

## [short-answer] Texte de la question
    regex-pattern
> Explication
```

### Syntaxe des exercices terminal

```
## Step N: Titre
Contexte.
\`\`\`console
$ ▌
\`\`\`
- [x] `bonne-commande` — Explication
- [ ] `mauvaise-commande` — Explication
\`\`\`output
résultat
\`\`\`
```

### Syntaxe des exercices de code

```yaml
# Front matter de instructions.md
type: code-exercise
mode: identify-and-fix
target:
  file: "chemin/fichier.go"
  lines: [3, 8]
```

```
## Patches
### [x] Bon patch
Explication.
\`\`\`diff
unified diff
\`\`\`
### [ ] Mauvais patch
Explication.
\`\`\`diff
unified diff
\`\`\`
```

### Arbre de décision : quel type d'étape choisir ?

```
Le learner doit-il faire quelque chose ?
├── Non → lesson
└── Oui
    ├── Répondre à des questions de connaissance ? → quiz
    ├── Exécuter des commandes dans un terminal ? → terminal-exercise
    └── Analyser et corriger du code source ? → code-exercise
```

---

## Ressources

- **Dépôt d'exemples** : [fsamin/phoebus-content-samples](https://github.com/fsamin/phoebus-content-samples)
  - `linux-fundamentals/` — Leçons, quiz, exercices terminal
  - `containerization/` — Exercice de code (Fix the Dockerfile)
  - `golang-programming/` — Parcours complet avec tous les types d'exercices
  - `kubernetes/` — Exercices terminal avancés
  - `git-mastery/` — Exercices Git
