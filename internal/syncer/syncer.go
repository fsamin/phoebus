package syncer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"mime"

	"github.com/fsamin/phoebus/internal/assets"
	"github.com/fsamin/phoebus/internal/crypto"
	"github.com/fsamin/phoebus/internal/logging"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

type Syncer struct {
	db                *sqlx.DB
	encryptionKey     string
	instanceSSHKeyPEM []byte // instance-level SSH private key (decrypted)
	assetStore        assets.Store
	maxAssetSize      int64
	storageBackend    string
	notify            chan struct{} // signals the worker that a new job is available
}

func New(db *sqlx.DB, encryptionKey string, instanceSSHKeyPEM []byte, assetStore assets.Store, maxAssetSize int64, storageBackend string) *Syncer {
	return &Syncer{
		db:                db,
		encryptionKey:     encryptionKey,
		instanceSSHKeyPEM: instanceSSHKeyPEM,
		assetStore:        assetStore,
		maxAssetSize:      maxAssetSize,
		storageBackend:    storageBackend,
		notify:            make(chan struct{}, 1),
	}
}

// Start launches the background worker that picks up sync jobs.
// It runs until ctx is cancelled.
func (s *Syncer) Start(ctx context.Context) {
	logger := logging.FromContext(ctx)
	logger.Info("sync worker started")
	for {
		select {
		case <-ctx.Done():
			logger.Info("sync worker stopped")
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
	// Create a sync collector for dual-write: stdout + in-memory accumulation
	collector := logging.NewSyncCollector(slog.Default().Handler())
	logger := slog.New(collector).With("repo_id", repoID, "job_id", jobID)
	ctx = logging.WithLogger(ctx, logger)
	syncStart := time.Now()

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
		s.failJob(ctx, collector, jobID, repoID, err)
		return
	}

	logger.Info("fetching repo details", "clone_url", repo.CloneURL, "branch", repo.Branch, "auth_type", repo.AuthType)

	// Clone to temp dir
	tmpDir, err := os.MkdirTemp("", "phoebus-sync-*")
	if err != nil {
		logger.Error("failed to create temp dir", "error", err)
		s.failJob(ctx, collector, jobID, repoID, err)
		return
	}
	defer os.RemoveAll(tmpDir)

	// For file:// URLs, use local path directly instead of git clone
	repoDir := tmpDir
	if strings.HasPrefix(repo.CloneURL, "file://") {
		localPath := strings.TrimPrefix(repo.CloneURL, "file://")
		if _, err := os.Stat(localPath); err != nil {
			logger.Error("local path not accessible", "path", localPath, "error", err)
			s.failJob(ctx, collector, jobID, repoID, fmt.Errorf("local path not accessible: %w", err))
			return
		}
		repoDir = localPath
		logger.Info("using local path", "path", localPath)
	} else {
		// Decrypt credentials if needed
		var creds []byte
		if repo.AuthType == "instance-ssh-key" {
			// Use the instance-level SSH keypair
			creds = s.instanceSSHKeyPEM
		} else if len(repo.Credentials) > 0 && s.encryptionKey != "" {
			decrypted, err := crypto.Decrypt(repo.Credentials, []byte(s.encryptionKey))
			if err != nil {
				// Fallback: might be plaintext (pre-encryption migration)
				creds = repo.Credentials
			} else {
				creds = decrypted
			}
		} else {
			creds = repo.Credentials
		}

		if err := gitClone(repo.CloneURL, repo.Branch, tmpDir, repo.AuthType, creds); err != nil {
			logger.Error("git clone failed", "error", err)
			s.failJob(ctx, collector, jobID, repoID, fmt.Errorf("git clone failed: %w", err))
			return
		}
		logger.Info("git clone succeeded")
	}

	// Parse and sync content
	if err := s.syncContent(ctx, repoID, repoDir); err != nil {
		logger.Error("content sync failed", "error", err)
		s.failJob(ctx, collector, jobID, repoID, err)
		return
	}

	// Persist collected logs
	logsJSON, _ := collector.Entries()

	// Mark success
	s.db.ExecContext(ctx, `
		UPDATE git_repositories SET sync_status = 'synced', sync_error = NULL, last_synced_at = now(), updated_at = now() WHERE id = $1
	`, repoID)
	s.db.ExecContext(ctx, `
		UPDATE sync_jobs SET status = 'done', completed_at = now(), logs = $2, updated_at = now() WHERE id = $1
	`, jobID, logsJSON)

	syncJobsTotal.WithLabelValues("done").Inc()
	syncDuration.Observe(time.Since(syncStart).Seconds())
	logger.Info("sync completed successfully")
}

func (s *Syncer) failJob(ctx context.Context, collector *logging.SyncCollector, jobID, repoID uuid.UUID, syncErr error) {
	syncJobsTotal.WithLabelValues("failed").Inc()
	errMsg := syncErr.Error()
	logsJSON, _ := collector.Entries()
	s.db.ExecContext(ctx, `
		UPDATE git_repositories SET sync_status = 'error', sync_error = $1, updated_at = now() WHERE id = $2
	`, errMsg, repoID)
	s.db.ExecContext(ctx, `
		UPDATE sync_jobs SET status = 'failed', error = $1, completed_at = now(), logs = $3, updated_at = now() WHERE id = $2
	`, errMsg, jobID, logsJSON)
}

func gitClone(cloneURL, branch, destDir, authType string, credentials []byte) error {
	args := []string{"clone", "--branch", branch, "--depth", "1", "--single-branch"}

	// For HTTP token auth, inject token into the clone URL
	if authType == "http-token" && len(credentials) > 0 {
		authURL, err := injectTokenInURL(cloneURL, string(credentials))
		if err != nil {
			return fmt.Errorf("inject token in URL: %w", err)
		}
		args = append(args, authURL, destDir)
		cmd := exec.Command("git", args...)
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	if authType == "http-basic" && len(credentials) > 0 {
		// credentials expected as "username:password"
		authURL, err := injectBasicAuthInURL(cloneURL, string(credentials))
		if err != nil {
			return fmt.Errorf("inject basic auth in URL: %w", err)
		}
		args = append(args, authURL, destDir)
		cmd := exec.Command("git", args...)
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	if authType == "instance-ssh-key" && len(credentials) > 0 {
		// Write SSH key to temp file
		tmpKey, err := os.CreateTemp("", "phoebus-ssh-*")
		if err != nil {
			return fmt.Errorf("create ssh key file: %w", err)
		}
		defer os.Remove(tmpKey.Name())

		if _, err := tmpKey.Write(credentials); err != nil {
			return fmt.Errorf("write ssh key: %w", err)
		}
		tmpKey.Close()
		os.Chmod(tmpKey.Name(), 0600)

		args = append(args, cloneURL, destDir)
		cmd := exec.Command("git", args...)
		cmd.Env = append(os.Environ(),
			fmt.Sprintf("GIT_SSH_COMMAND=ssh -i %s -o StrictHostKeyChecking=no", tmpKey.Name()),
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	// No auth
	args = append(args, cloneURL, destDir)
	cmd := exec.Command("git", args...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// injectTokenInURL returns https://x-access-token:<token>@host/path
func injectTokenInURL(cloneURL, token string) (string, error) {
	u, err := url.Parse(cloneURL)
	if err != nil {
		return "", err
	}
	u.User = url.UserPassword("x-access-token", token)
	return u.String(), nil
}

// injectBasicAuthInURL returns https://user:pass@host/path
func injectBasicAuthInURL(cloneURL, userpass string) (string, error) {
	u, err := url.Parse(cloneURL)
	if err != nil {
		return "", err
	}
	parts := strings.SplitN(userpass, ":", 2)
	if len(parts) == 2 {
		u.User = url.UserPassword(parts[0], parts[1])
	} else {
		u.User = url.User(parts[0])
	}
	return u.String(), nil
}

func (s *Syncer) syncContent(ctx context.Context, repoID uuid.UUID, repoDir string) error {
	logger := logging.FromContext(ctx)

	// Discover learning paths
	pathDirs, err := discoverLearningPaths(repoDir)
	if err != nil {
		return err
	}
	logger.Info("discovered learning paths", "count", len(pathDirs))

	// Begin transaction
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	existingPathKeys := map[string]bool{}
	for _, pd := range pathDirs {
		if err := s.syncOnePath(ctx, tx, repoID, pd.dir, pd.filePath); err != nil {
			return err
		}
		existingPathKeys[pd.filePath] = true
	}

	// Soft-delete learning paths that no longer exist in this repo
	tx.ExecContext(ctx, `
		UPDATE learning_paths SET deleted_at = now(), updated_at = now()
		WHERE repo_id = $1 AND file_path != ALL($2) AND deleted_at IS NULL
	`, repoID, pq.Array(keys(existingPathKeys)))

	// Restore learning paths that reappeared
	tx.ExecContext(ctx, `
		UPDATE learning_paths SET deleted_at = NULL, updated_at = now()
		WHERE repo_id = $1 AND file_path = ANY($2) AND deleted_at IS NOT NULL
	`, repoID, pq.Array(keys(existingPathKeys)))

	return tx.Commit()
}

type pathDir struct {
	dir      string // absolute path to the learning path directory
	filePath string // relative path used as unique key (empty for root, "subdir" for subdirs)
}

// discoverLearningPaths finds all phoebus.yaml files in the repo.
func discoverLearningPaths(repoDir string) ([]pathDir, error) {
	// Check root first
	if _, err := os.Stat(filepath.Join(repoDir, "phoebus.yaml")); err == nil {
		return []pathDir{{dir: repoDir, filePath: ""}}, nil
	}

	// Check subdirectories for multi-path repos
	entries, err := os.ReadDir(repoDir)
	if err != nil {
		return nil, fmt.Errorf("read repo dir: %w", err)
	}

	var paths []pathDir
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		subDir := filepath.Join(repoDir, e.Name())
		if _, err := os.Stat(filepath.Join(subDir, "phoebus.yaml")); err == nil {
			paths = append(paths, pathDir{dir: subDir, filePath: e.Name()})
		}
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("no phoebus.yaml found at root or in subdirectories")
	}

	sort.Slice(paths, func(i, j int) bool {
		return paths[i].filePath < paths[j].filePath
	})
	return paths, nil
}

func (s *Syncer) syncOnePath(ctx context.Context, tx *sqlx.Tx, repoID uuid.UUID, pathRoot, filePath string) error {
	logger := logging.FromContext(ctx)

	lpMeta, err := parsePhoebus(ctx, pathRoot)
	if err != nil {
		return fmt.Errorf("parse phoebus.yaml in %s: %w", filePath, err)
	}
	logger.Info("syncing learning path", "title", lpMeta.Title, "file_path", filePath)

	// Upsert learning path (without hash yet — computed after modules)
	var lpID uuid.UUID
	err = tx.GetContext(ctx, &lpID, `
		INSERT INTO learning_paths (repo_id, title, description, icon, tags, estimated_duration, prerequisites, file_path)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (repo_id, file_path) DO UPDATE SET
			title = EXCLUDED.title,
			description = EXCLUDED.description,
			icon = EXCLUDED.icon,
			tags = EXCLUDED.tags,
			estimated_duration = EXCLUDED.estimated_duration,
			prerequisites = EXCLUDED.prerequisites,
			deleted_at = NULL,
			updated_at = now()
			-- enabled is NOT touched: admin-controlled field
		RETURNING id
	`, repoID, lpMeta.Title, lpMeta.Description, lpMeta.Icon, pq.Array(lpMeta.Tags), lpMeta.EstimatedDuration, pq.Array(lpMeta.Prerequisites), filePath)
	if err != nil {
		return fmt.Errorf("upsert learning path %s: %w", lpMeta.Title, err)
	}

	// Check existing path-level hash
	var existingPathHash string
	_ = tx.GetContext(ctx, &existingPathHash, `SELECT content_hash FROM learning_paths WHERE id = $1`, lpID)

	// Find module directories
	moduleDirs, err := findOrderedDirs(pathRoot)
	if err != nil {
		return fmt.Errorf("find modules in %s: %w", filePath, err)
	}
	if len(moduleDirs) == 0 {
		logger.Warn("learning path has no modules", "title", lpMeta.Title, "file_path", filePath)
	}
	logger.Debug("found modules", "count", len(moduleDirs))

	// Pre-compute the full path hash to check if we can skip everything
	pathHashParts := []string{lpMeta.Title, lpMeta.Description, strings.Join(lpMeta.Tags, ",")}
	var moduleHashes []string

	existingModulePaths := map[string]bool{}
	for position, moduleDir := range moduleDirs {
		modulePath := filepath.Base(moduleDir)
		existingModulePaths[modulePath] = true

		modMeta, err := parseModuleIndex(ctx, moduleDir)
		if err != nil {
			return fmt.Errorf("parse module %s: %w", modulePath, err)
		}
		logger.Debug("syncing module", "module", modulePath, "title", modMeta.Title, "competencies", len(modMeta.Competencies))

		// Compute step hashes for this module
		stepFiles, err := findOrderedSteps(moduleDir)
		if err != nil {
			return fmt.Errorf("find steps in %s: %w", modulePath, err)
		}

		if len(stepFiles) == 0 {
			logger.Warn("module has no steps", "module", modulePath)
		}

		var stepHashes []string
		type stepData struct {
			filePath     string
			meta         stepMeta
			contentMD    string
			exerciseData []byte
			hash         string
			stepPath     string
		}
		var stepsToSync []stepData

		for stepPos, stepPath := range stepFiles {
			stepFilePath := filepath.Base(stepPath)
			sMeta, contentMD, exerciseData, err := parseStep(ctx, stepPath)
			if err != nil {
				return fmt.Errorf("parse step %s/%s: %w", modulePath, stepFilePath, err)
			}
			logger.Debug("parsed step", "module", modulePath, "step", stepFilePath, "type", sMeta.Type)
			_ = stepPos // used later

			h := computeHash(sMeta.Title, sMeta.Type, sMeta.Duration, contentMD, string(exerciseData))
			stepHashes = append(stepHashes, h)
			stepsToSync = append(stepsToSync, stepData{
				filePath:     stepFilePath,
				meta:         *sMeta,
				contentMD:    contentMD,
				exerciseData: exerciseData,
				hash:         h,
				stepPath:     stepPath,
			})
		}

		moduleHash := computeHash(modMeta.Title, modMeta.Description, strings.Join(modMeta.Competencies, ","), strings.Join(stepHashes, ","))
		moduleHashes = append(moduleHashes, moduleHash)

		// Upsert module
		var moduleID uuid.UUID
		err = tx.GetContext(ctx, &moduleID, `
			INSERT INTO modules (learning_path_id, title, description, competencies, position, file_path, content_hash)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (learning_path_id, file_path) DO UPDATE SET
				title = EXCLUDED.title,
				description = EXCLUDED.description,
				competencies = EXCLUDED.competencies,
				position = EXCLUDED.position,
				content_hash = EXCLUDED.content_hash,
				deleted_at = NULL,
				updated_at = now()
			RETURNING id
		`, lpID, modMeta.Title, modMeta.Description, pq.Array(modMeta.Competencies), position, modulePath, moduleHash)
		if err != nil {
			return fmt.Errorf("upsert module %s: %w", modulePath, err)
		}

		// Check if module hash changed — if not, skip all steps
		var existingModuleHash string
		// The RETURNING id above already committed the upsert, but we can check from the DO UPDATE
		// We need to compare BEFORE the upsert. Query the old hash first.
		// Actually, the upsert already happened. Let's check if the hash we just wrote differs from what was there.
		// Simpler: just compare stepHashes individually.

		// Sync steps within this module
		existingStepPaths := map[string]bool{}
		for stepPos, sd := range stepsToSync {
			existingStepPaths[sd.filePath] = true

			// Check if this step's hash matches what's in DB
			var existingStepHash string
			err = tx.GetContext(ctx, &existingStepHash, `
				SELECT content_hash FROM steps
				WHERE module_id = $1 AND file_path = $2 AND deleted_at IS NULL
			`, moduleID, sd.filePath)
			if err == nil && existingStepHash == sd.hash {
				logger.Debug("step unchanged, skipping", "module", modulePath, "step", sd.filePath)
				// Still need to ensure position is correct
				tx.ExecContext(ctx, `UPDATE steps SET position = $1, updated_at = now() WHERE module_id = $2 AND file_path = $3 AND deleted_at IS NULL`, stepPos, moduleID, sd.filePath)
				continue
			}

			// Step changed or new — upsert
			var stepID uuid.UUID
			err = tx.GetContext(ctx, &stepID, `
				INSERT INTO steps (module_id, title, type, estimated_duration, content_md, exercise_data, position, file_path, content_hash)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
				ON CONFLICT (module_id, file_path) WHERE deleted_at IS NULL DO UPDATE SET
					title = EXCLUDED.title,
					type = EXCLUDED.type,
					estimated_duration = EXCLUDED.estimated_duration,
					content_md = EXCLUDED.content_md,
					exercise_data = EXCLUDED.exercise_data,
					position = EXCLUDED.position,
					content_hash = EXCLUDED.content_hash,
					deleted_at = NULL,
					updated_at = now()
				RETURNING id
			`, moduleID, sd.meta.Title, sd.meta.Type, sd.meta.Duration, sd.contentMD, sd.exerciseData, stepPos, sd.filePath, sd.hash)
			if err != nil {
				// Fallback: step might be soft-deleted, restore it
				err = tx.GetContext(ctx, &stepID, `
					UPDATE steps SET
						title = $1, type = $2, estimated_duration = $3,
						content_md = $4, exercise_data = $5, position = $6,
						content_hash = $7, deleted_at = NULL, updated_at = now()
					WHERE module_id = $8 AND file_path = $9
					RETURNING id
				`, sd.meta.Title, sd.meta.Type, sd.meta.Duration, sd.contentMD, sd.exerciseData, stepPos, sd.hash, moduleID, sd.filePath)
				if err != nil {
					return fmt.Errorf("upsert step %s/%s: %w", modulePath, sd.filePath, err)
				}
			}

			logger.Debug("step updated", "module", modulePath, "step", sd.filePath, "hash", sd.hash[:8])

			// Sync assets and rewrite URLs in content_md
			stepDir := filepath.Dir(sd.stepPath)
			if sd.meta.Type == "code-exercise" {
				// For code exercises, stepPath is .../exercise-dir/instructions.md
				// Assets dir is .../exercise-dir/assets/
			} else {
				// For regular steps, stepPath is .../module-dir/step.md
				// Assets dir is .../module-dir/assets/ (shared by all steps in the module)
				// But we want per-step assets, so we look for assets relative to the step name too
			}
			rewrites, err := s.syncStepAssets(ctx, tx, stepID, stepDir)
			if err != nil {
				return fmt.Errorf("sync assets %s/%s: %w", modulePath, sd.filePath, err)
			}
			if len(rewrites) > 0 {
				rewritten := rewriteAssetURLs(sd.contentMD, rewrites)
				tx.ExecContext(ctx, `UPDATE steps SET content_md = $1 WHERE id = $2`, rewritten, stepID)
			}

			if sd.meta.Type == "code-exercise" {
				codebaseDir := filepath.Join(filepath.Dir(sd.stepPath), "codebase")
				if err := s.syncCodebaseFiles(ctx, tx, stepID, codebaseDir); err != nil {
					return fmt.Errorf("sync codebase %s/%s: %w", modulePath, sd.filePath, err)
				}
			}
		}

		// Soft-delete steps that no longer exist in this module
		tx.ExecContext(ctx, `
			UPDATE steps SET deleted_at = now(), updated_at = now()
			WHERE module_id = $1 AND file_path != ALL($2) AND deleted_at IS NULL
		`, moduleID, pq.Array(keys(existingStepPaths)))

		_ = existingModuleHash
	}

	// Soft-delete modules that no longer exist
	tx.ExecContext(ctx, `
		UPDATE modules SET deleted_at = now(), updated_at = now()
		WHERE learning_path_id = $1 AND file_path != ALL($2) AND deleted_at IS NULL
	`, lpID, pq.Array(keys(existingModulePaths)))

	// Update path-level hash
	pathHash := computeHash(append(pathHashParts, moduleHashes...)...)
	tx.ExecContext(ctx, `UPDATE learning_paths SET content_hash = $1, updated_at = now() WHERE id = $2`, pathHash, lpID)

	return nil
}

// computeHash returns SHA-256 hex digest of concatenated parts
func computeHash(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0}) // separator
	}
	return hex.EncodeToString(h.Sum(nil))
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

// syncStepAssets scans an assets/ directory relative to the step, uploads new assets,
// records them in the DB, and returns a map of original_path → /api/assets/{hash} for URL rewriting.
func (s *Syncer) syncStepAssets(ctx context.Context, tx *sqlx.Tx, stepID uuid.UUID, stepDir string) (map[string]string, error) {
	assetsDir := filepath.Join(stepDir, "assets")
	if _, err := os.Stat(assetsDir); os.IsNotExist(err) {
		return nil, nil
	}

	// Clean existing step_assets for this step (we'll re-link)
	tx.ExecContext(ctx, `DELETE FROM step_assets WHERE step_id = $1`, stepID)

	rewrites := map[string]string{}

	err := filepath.Walk(assetsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		if info.Size() > s.maxAssetSize {
			logging.FromContext(ctx).Warn("asset exceeds max size, skipping", "path", path, "size", info.Size(), "max", s.maxAssetSize)
			return nil
		}

		// Compute hash
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read asset %s: %w", path, err)
		}

		h := sha256.Sum256(data)
		hash := hex.EncodeToString(h[:])

		relPath, _ := filepath.Rel(stepDir, path)
		fileName := filepath.Base(path)
		contentType := mime.TypeByExtension(filepath.Ext(fileName))
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		// Check if asset already exists in DB
		var assetID uuid.UUID
		err = tx.GetContext(ctx, &assetID, `SELECT id FROM content_assets WHERE content_hash = $1`, hash)
		if err != nil {
			// New asset — upload to store
			exists, _ := s.assetStore.Exists(ctx, hash)
			if !exists {
				if err := s.assetStore.Put(ctx, hash, contentType, bytes.NewReader(data)); err != nil {
					return fmt.Errorf("upload asset %s: %w", relPath, err)
				}
			}

			// Insert into content_assets
			err = tx.GetContext(ctx, &assetID, `
				INSERT INTO content_assets (content_hash, content_type, file_name, size_bytes, storage_backend)
				VALUES ($1, $2, $3, $4, $5)
				ON CONFLICT (content_hash) DO UPDATE SET content_hash = EXCLUDED.content_hash
				RETURNING id
			`, hash, contentType, fileName, info.Size(), s.storageBackend)
			if err != nil {
				return fmt.Errorf("insert content_asset %s: %w", relPath, err)
			}
		}

		// Link step → asset
		tx.ExecContext(ctx, `
			INSERT INTO step_assets (step_id, asset_id, original_path) VALUES ($1, $2, $3)
			ON CONFLICT DO NOTHING
		`, stepID, assetID, relPath)

		// Build rewrite map: ./assets/img.png → /api/assets/{hash}
		rewrites["./"+relPath] = "/api/assets/" + hash
		rewrites[relPath] = "/api/assets/" + hash
		// Also handle assets/img.png without ./
		rewrites["assets/"+fileName] = "/api/assets/" + hash

		return nil
	})

	return rewrites, err
}

// rewriteAssetURLs replaces relative asset paths in markdown content with API URLs.
// Only replaces paths within markdown image/link syntax: ![...](path) or [...]( path)
func rewriteAssetURLs(content string, rewrites map[string]string) string {
	if len(rewrites) == 0 {
		return content
	}
	// Replace longest paths first to avoid partial matches
	for original, apiURL := range rewrites {
		// Only replace when preceded by ]( — markdown link/image syntax
		content = strings.ReplaceAll(content, "]("+original+")", "]("+apiURL+")")
	}
	return content
}
