package syncer

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

type Syncer struct {
	db     *sqlx.DB
	notify chan struct{} // signals the worker that a new job is available
}

func New(db *sqlx.DB) *Syncer {
	return &Syncer{
		db:     db,
		notify: make(chan struct{}, 1),
	}
}

// Start launches the background worker that picks up sync jobs.
// It runs until ctx is cancelled.
func (s *Syncer) Start(ctx context.Context) {
	slog.Info("sync worker started")
	for {
		select {
		case <-ctx.Done():
			slog.Info("sync worker stopped")
			return
		case <-s.notify:
			s.processAllPending(ctx)
		case <-time.After(30 * time.Second):
			// Poll periodically as a safety net
			s.processAllPending(ctx)
		}
	}
}

// Notify wakes the worker to pick up new jobs.
func (s *Syncer) Notify() {
	select {
	case s.notify <- struct{}{}:
	default:
		// Already notified, skip
	}
}

// processAllPending processes all pending jobs one by one using SELECT FOR UPDATE SKIP LOCKED.
func (s *Syncer) processAllPending(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		processed := s.pickAndProcess(ctx)
		if !processed {
			return
		}
	}
}

// pickAndProcess atomically picks one pending job and processes it.
// Returns true if a job was processed, false if no job was available.
func (s *Syncer) pickAndProcess(ctx context.Context) bool {
	// Pick a pending job with SELECT FOR UPDATE SKIP LOCKED
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		slog.Error("failed to begin job tx", "error", err)
		return false
	}

	var job struct {
		ID     uuid.UUID `db:"id"`
		RepoID uuid.UUID `db:"repo_id"`
	}
	err = tx.GetContext(ctx, &job, `
		SELECT id, repo_id FROM sync_jobs
		WHERE status = 'pending'
		ORDER BY created_at ASC
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	`)
	if err != nil {
		tx.Rollback()
		if err == sql.ErrNoRows {
			return false
		}
		slog.Error("failed to pick sync job", "error", err)
		return false
	}

	// Mark as processing
	tx.ExecContext(ctx, `
		UPDATE sync_jobs SET status = 'processing', attempts = attempts + 1, started_at = now(), updated_at = now() WHERE id = $1
	`, job.ID)
	tx.ExecContext(ctx, `
		UPDATE git_repositories SET sync_status = 'syncing', updated_at = now() WHERE id = $1
	`, job.RepoID)
	if err := tx.Commit(); err != nil {
		slog.Error("failed to commit job pickup", "error", err)
		return false
	}

	// Process the job (outside the lock transaction)
	s.processJob(ctx, job.ID, job.RepoID)
	return true
}

// processJob clones the repo, parses content, and updates the database.
func (s *Syncer) processJob(ctx context.Context, jobID, repoID uuid.UUID) {
	logger := slog.With("repo_id", repoID, "job_id", jobID)

	// Fetch repo details
	var repo struct {
		ID          uuid.UUID `db:"id"`
		CloneURL    string    `db:"clone_url"`
		Branch      string    `db:"branch"`
		AuthType    string    `db:"auth_type"`
		Credentials []byte    `db:"credentials"`
	}
	if err := s.db.GetContext(ctx, &repo, `
		SELECT id, clone_url, branch, auth_type, credentials FROM git_repositories WHERE id = $1
	`, repoID); err != nil {
		logger.Error("failed to fetch repo", "error", err)
		s.failJob(ctx, jobID, repoID, err)
		return
	}

	// Clone to temp dir
	tmpDir, err := os.MkdirTemp("", "phoebus-sync-*")
	if err != nil {
		logger.Error("failed to create temp dir", "error", err)
		s.failJob(ctx, jobID, repoID, err)
		return
	}
	defer os.RemoveAll(tmpDir)

	if err := gitClone(repo.CloneURL, repo.Branch, tmpDir); err != nil {
		logger.Error("git clone failed", "error", err)
		s.failJob(ctx, jobID, repoID, fmt.Errorf("git clone failed: %w", err))
		return
	}

	// Parse and sync content
	if err := s.syncContent(ctx, repoID, tmpDir); err != nil {
		logger.Error("content sync failed", "error", err)
		s.failJob(ctx, jobID, repoID, err)
		return
	}

	// Mark success
	s.db.ExecContext(ctx, `
		UPDATE git_repositories SET sync_status = 'synced', sync_error = NULL, last_synced_at = now(), updated_at = now() WHERE id = $1
	`, repoID)
	s.db.ExecContext(ctx, `
		UPDATE sync_jobs SET status = 'done', completed_at = now(), updated_at = now() WHERE id = $1
	`, jobID)

	logger.Info("sync completed successfully")
}

func (s *Syncer) failJob(ctx context.Context, jobID, repoID uuid.UUID, syncErr error) {
	errMsg := syncErr.Error()
	s.db.ExecContext(ctx, `
		UPDATE git_repositories SET sync_status = 'error', sync_error = $1, updated_at = now() WHERE id = $2
	`, errMsg, repoID)
	s.db.ExecContext(ctx, `
		UPDATE sync_jobs SET status = 'failed', error = $1, completed_at = now(), updated_at = now() WHERE id = $2
	`, errMsg, jobID)
}

func gitClone(cloneURL, branch, destDir string) error {
	cmd := exec.Command("git", "clone", "--branch", branch, "--depth", "1", "--single-branch", cloneURL, destDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (s *Syncer) syncContent(ctx context.Context, repoID uuid.UUID, repoDir string) error {
	// Parse phoebus.yaml
	lpMeta, err := parsePhoebus(repoDir)
	if err != nil {
		return fmt.Errorf("parse phoebus.yaml: %w", err)
	}

	// Begin transaction
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Upsert learning path
	var lpID uuid.UUID
	err = tx.GetContext(ctx, &lpID, `
		INSERT INTO learning_paths (repo_id, title, description, icon, tags, estimated_duration, prerequisites)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (repo_id) DO UPDATE SET
			title = EXCLUDED.title,
			description = EXCLUDED.description,
			icon = EXCLUDED.icon,
			tags = EXCLUDED.tags,
			estimated_duration = EXCLUDED.estimated_duration,
			prerequisites = EXCLUDED.prerequisites,
			updated_at = now()
		RETURNING id
	`, repoID, lpMeta.Title, lpMeta.Description, lpMeta.Icon, pq.Array(lpMeta.Tags), lpMeta.EstimatedDuration, pq.Array(lpMeta.Prerequisites))
	if err != nil {
		return fmt.Errorf("upsert learning path: %w", err)
	}

	// Find module directories
	moduleDirs, err := findOrderedDirs(repoDir)
	if err != nil {
		return fmt.Errorf("find modules: %w", err)
	}

	// Soft-delete all existing steps (will be restored if still present)
	tx.ExecContext(ctx, `
		UPDATE steps SET deleted_at = now()
		WHERE module_id IN (SELECT id FROM modules WHERE learning_path_id = $1)
		AND deleted_at IS NULL
	`, lpID)

	existingModulePaths := map[string]bool{}
	for position, moduleDir := range moduleDirs {
		modulePath := filepath.Base(moduleDir)

		// Parse module index.md
		modMeta, err := parseModuleIndex(moduleDir)
		if err != nil {
			return fmt.Errorf("parse module %s: %w", modulePath, err)
		}

		// Upsert module
		var moduleID uuid.UUID
		err = tx.GetContext(ctx, &moduleID, `
			INSERT INTO modules (learning_path_id, title, description, competencies, position, file_path)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (learning_path_id, file_path) DO UPDATE SET
				title = EXCLUDED.title,
				description = EXCLUDED.description,
				competencies = EXCLUDED.competencies,
				position = EXCLUDED.position,
				updated_at = now()
			RETURNING id
		`, lpID, modMeta.Title, modMeta.Description, pq.Array(modMeta.Competencies), position, modulePath)
		if err != nil {
			return fmt.Errorf("upsert module %s: %w", modulePath, err)
		}

		existingModulePaths[modulePath] = true

		// Parse steps
		stepFiles, err := findOrderedSteps(moduleDir)
		if err != nil {
			return fmt.Errorf("find steps in %s: %w", modulePath, err)
		}

		for stepPos, stepPath := range stepFiles {
			stepFilePath := filepath.Base(stepPath)
			stepMeta, contentMD, exerciseData, err := parseStep(stepPath)
			if err != nil {
				return fmt.Errorf("parse step %s/%s: %w", modulePath, stepFilePath, err)
			}

			// Upsert step (restore if soft-deleted)
			var stepID uuid.UUID
			err = tx.GetContext(ctx, &stepID, `
				INSERT INTO steps (module_id, title, type, estimated_duration, content_md, exercise_data, position, file_path)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
				ON CONFLICT (module_id, file_path) WHERE deleted_at IS NULL DO UPDATE SET
					title = EXCLUDED.title,
					type = EXCLUDED.type,
					estimated_duration = EXCLUDED.estimated_duration,
					content_md = EXCLUDED.content_md,
					exercise_data = EXCLUDED.exercise_data,
					position = EXCLUDED.position,
					deleted_at = NULL,
					updated_at = now()
				RETURNING id
			`, moduleID, stepMeta.Title, stepMeta.Type, stepMeta.Duration, contentMD, exerciseData, stepPos, stepFilePath)
			if err != nil {
				// Try to restore soft-deleted step
				err = tx.GetContext(ctx, &stepID, `
					UPDATE steps SET
						title = $1, type = $2, estimated_duration = $3,
						content_md = $4, exercise_data = $5, position = $6,
						deleted_at = NULL, updated_at = now()
					WHERE module_id = $7 AND file_path = $8
					RETURNING id
				`, stepMeta.Title, stepMeta.Type, stepMeta.Duration, contentMD, exerciseData, stepPos, moduleID, stepFilePath)
				if err != nil {
					return fmt.Errorf("upsert step %s/%s: %w", modulePath, stepFilePath, err)
				}
			}

			// Handle codebase files for code exercises
			if stepMeta.Type == "code-exercise" {
				codebaseDir := filepath.Join(filepath.Dir(stepPath), "codebase")
				if err := s.syncCodebaseFiles(ctx, tx, stepID, codebaseDir); err != nil {
					return fmt.Errorf("sync codebase %s/%s: %w", modulePath, stepFilePath, err)
				}
			}
		}
	}

	// Delete modules that no longer exist
	tx.ExecContext(ctx, `
		DELETE FROM modules WHERE learning_path_id = $1 AND file_path != ALL($2)
	`, lpID, pq.Array(keys(existingModulePaths)))

	return tx.Commit()
}

func (s *Syncer) syncCodebaseFiles(ctx context.Context, tx *sqlx.Tx, stepID uuid.UUID, codebaseDir string) error {
	// Delete existing files
	tx.ExecContext(ctx, "DELETE FROM codebase_files WHERE step_id = $1", stepID)

	if _, err := os.Stat(codebaseDir); os.IsNotExist(err) {
		return nil
	}

	position := 0
	return filepath.Walk(codebaseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		// Skip binary files (simple heuristic)
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if isBinary(content) {
			return nil
		}

		relPath, _ := filepath.Rel(codebaseDir, path)
		lang := inferLanguage(filepath.Ext(relPath))
		_, err = tx.ExecContext(ctx, `
			INSERT INTO codebase_files (step_id, file_path, content, language, position) VALUES ($1, $2, $3, $4, $5)
		`, stepID, relPath, string(content), lang, position)
		position++
		return err
	})
}

// --- Helpers ---

func findOrderedDirs(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") && e.Name() != "assets" {
			dirs = append(dirs, filepath.Join(root, e.Name()))
		}
	}
	sort.Slice(dirs, func(i, j int) bool {
		return extractOrder(filepath.Base(dirs[i])) < extractOrder(filepath.Base(dirs[j]))
	})
	return dirs, nil
}

func findOrderedSteps(moduleDir string) ([]string, error) {
	entries, err := os.ReadDir(moduleDir)
	if err != nil {
		return nil, err
	}
	var steps []string
	for _, e := range entries {
		name := e.Name()
		if name == "index.md" {
			continue
		}
		if e.IsDir() {
			// Code exercise directory — look for instructions.md
			instrPath := filepath.Join(moduleDir, name, "instructions.md")
			if _, err := os.Stat(instrPath); err == nil {
				steps = append(steps, instrPath)
			}
		} else if strings.HasSuffix(name, ".md") {
			steps = append(steps, filepath.Join(moduleDir, name))
		}
	}
	sort.Slice(steps, func(i, j int) bool {
		// For regular .md files, extract order from filename.
		// For code exercise dirs (instructions.md), extract order from parent dir name.
		nameI := filepath.Base(steps[i])
		if nameI == "instructions.md" {
			nameI = filepath.Base(filepath.Dir(steps[i]))
		}
		nameJ := filepath.Base(steps[j])
		if nameJ == "instructions.md" {
			nameJ = filepath.Base(filepath.Dir(steps[j]))
		}
		return extractOrder(nameI) < extractOrder(nameJ)
	})
	return steps, nil
}

func extractOrder(name string) int {
	parts := strings.SplitN(name, "-", 2)
	if len(parts) > 0 {
		if n, err := strconv.Atoi(parts[0]); err == nil {
			return n
		}
	}
	return 9999
}

func isBinary(content []byte) bool {
	for _, b := range content {
		if b == 0 {
			return true
		}
	}
	return false
}

func keys(m map[string]bool) []string {
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}

// inferLanguage maps file extensions to language identifiers for syntax highlighting.
func inferLanguage(ext string) string {
	ext = strings.ToLower(ext)
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".jsx":
		return "jsx"
	case ".tsx":
		return "tsx"
	case ".java":
		return "java"
	case ".rs":
		return "rust"
	case ".rb":
		return "ruby"
	case ".sh", ".bash":
		return "shell"
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".toml":
		return "toml"
	case ".xml":
		return "xml"
	case ".html", ".htm":
		return "html"
	case ".css":
		return "css"
	case ".sql":
		return "sql"
	case ".md":
		return "markdown"
	case ".dockerfile":
		return "dockerfile"
	case ".tf", ".hcl":
		return "hcl"
	case ".c":
		return "c"
	case ".cpp", ".cc", ".cxx":
		return "cpp"
	case ".h", ".hpp":
		return "cpp"
	case ".cs":
		return "csharp"
	default:
		return ""
	}
}
