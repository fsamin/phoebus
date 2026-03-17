package handler

import (
	"encoding/json"
	"net/http"

	"github.com/fsamin/phoebus/internal/model"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// --- Learning Paths (public read API) ---

type learningPathResponse struct {
	model.LearningPath
	ModuleCount            int      `json:"module_count" db:"module_count"`
	StepCount              int      `json:"step_count" db:"step_count"`
	CompetenciesProvided   []string `json:"competencies_provided"`
}

func (h *Handler) ListCompetencies(w http.ResponseWriter, r *http.Request) {
	type competencyRow struct {
		Name           string `json:"name" db:"name"`
		LearningPathID string `json:"learning_path_id" db:"learning_path_id"`
	}
	var rows []competencyRow
	err := h.db.SelectContext(r.Context(), &rows, `
		SELECT DISTINCT unnest(m.competencies) AS name, m.learning_path_id::text AS learning_path_id
		FROM modules m
		JOIN learning_paths lp ON lp.id = m.learning_path_id AND lp.deleted_at IS NULL AND lp.enabled = true
		WHERE m.deleted_at IS NULL AND array_length(m.competencies, 1) > 0
		ORDER BY name
	`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list competencies"})
		return
	}

	type competencyResponse struct {
		Name            string   `json:"name"`
		LearningPathIDs []string `json:"learning_path_ids"`
	}
	compMap := map[string]*competencyResponse{}
	for _, row := range rows {
		if c, ok := compMap[row.Name]; ok {
			// Deduplicate
			found := false
			for _, id := range c.LearningPathIDs {
				if id == row.LearningPathID {
					found = true
					break
				}
			}
			if !found {
				c.LearningPathIDs = append(c.LearningPathIDs, row.LearningPathID)
			}
		} else {
			compMap[row.Name] = &competencyResponse{Name: row.Name, LearningPathIDs: []string{row.LearningPathID}}
		}
	}
	out := make([]competencyResponse, 0, len(compMap))
	for _, c := range compMap {
		out = append(out, *c)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) ListLearningPaths(w http.ResponseWriter, r *http.Request) {
	var paths []learningPathResponse
	err := h.db.SelectContext(r.Context(), &paths, `
		SELECT lp.id, lp.repo_id, lp.title, lp.description, lp.icon, lp.tags,
		       lp.estimated_duration, lp.prerequisites, lp.slug, lp.created_at, lp.updated_at,
		       COUNT(DISTINCT m.id) AS module_count,
		       COUNT(DISTINCT s.id) AS step_count
		FROM learning_paths lp
		LEFT JOIN modules m ON m.learning_path_id = lp.id AND m.deleted_at IS NULL
		LEFT JOIN steps s ON s.module_id = m.id AND s.deleted_at IS NULL
		WHERE lp.deleted_at IS NULL AND lp.enabled = true
		GROUP BY lp.id
		ORDER BY lp.title
	`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list learning paths"})
		return
	}
	if paths == nil {
		paths = []learningPathResponse{}
	}

	// Fetch competencies provided by each path (aggregated from modules)
	type compRow struct {
		LearningPathID string `db:"learning_path_id"`
		Competency     string `db:"competency"`
	}
	var compRows []compRow
	h.db.SelectContext(r.Context(), &compRows, `
		SELECT m.learning_path_id::text AS learning_path_id, unnest(m.competencies) AS competency
		FROM modules m
		JOIN learning_paths lp ON lp.id = m.learning_path_id AND lp.deleted_at IS NULL AND lp.enabled = true
		WHERE m.deleted_at IS NULL AND array_length(m.competencies, 1) > 0
	`)
	compByPath := map[string][]string{}
	for _, cr := range compRows {
		// Deduplicate
		found := false
		for _, c := range compByPath[cr.LearningPathID] {
			if c == cr.Competency {
				found = true
				break
			}
		}
		if !found {
			compByPath[cr.LearningPathID] = append(compByPath[cr.LearningPathID], cr.Competency)
		}
	}
	for i := range paths {
		if comps, ok := compByPath[paths[i].ID.String()]; ok {
			paths[i].CompetenciesProvided = comps
		} else {
			paths[i].CompetenciesProvided = []string{}
		}
	}

	// Enrich with user progress if authenticated
	claims := ClaimsFromContext(r.Context())
	type pathProg struct {
		PathID    string `db:"path_id"`
		Total     int    `db:"total"`
		Completed int    `db:"completed"`
	}
	var userProgress []pathProg
	if claims != nil {
		h.db.SelectContext(r.Context(), &userProgress, `
			SELECT m.learning_path_id AS path_id,
			       COUNT(DISTINCT s.id) AS total,
			       COUNT(DISTINCT CASE WHEN p.status = 'completed' THEN s.id END) AS completed
			FROM steps s
			JOIN modules m ON m.id = s.module_id AND m.deleted_at IS NULL
			LEFT JOIN progress p ON p.step_id = s.id AND p.user_id = $1
			WHERE s.deleted_at IS NULL AND EXISTS (
				SELECT 1 FROM progress p2 JOIN steps s2 ON s2.id = p2.step_id
				JOIN modules m2 ON m2.id = s2.module_id AND m2.deleted_at IS NULL
				WHERE p2.user_id = $1 AND m2.learning_path_id = m.learning_path_id
			)
			GROUP BY m.learning_path_id
		`, claims.UserID)
	}
	progMap := map[string]pathProg{}
	for _, pp := range userProgress {
		progMap[pp.PathID] = pp
	}

	// Build set of acquired competencies (from fully completed paths)
	acquiredCompetencies := map[string]bool{}
	for _, pp := range userProgress {
		if pp.Total > 0 && pp.Completed == pp.Total {
			// This path is completed — all its competencies are acquired
			if comps, ok := compByPath[pp.PathID]; ok {
				for _, c := range comps {
					acquiredCompetencies[c] = true
				}
			}
		}
	}

	// Fetch owners per learning path (via repo_id)
	type ownerRow struct {
		RepoID      string `db:"repo_id"`
		DisplayName string `db:"display_name"`
	}
	var ownerRows []ownerRow
	h.db.SelectContext(r.Context(), &ownerRows, `
		SELECT DISTINCT lp.repo_id::text AS repo_id, u.display_name
		FROM learning_paths lp
		JOIN repository_owners ro ON ro.repo_id = lp.repo_id
		JOIN users u ON u.id = ro.user_id
		WHERE lp.deleted_at IS NULL AND lp.enabled = true
		ORDER BY u.display_name
	`)
	ownersByRepo := map[string][]string{}
	for _, o := range ownerRows {
		ownersByRepo[o.RepoID] = append(ownersByRepo[o.RepoID], o.DisplayName)
	}

	type enrichedPath struct {
		learningPathResponse
		ProgressTotal     *int     `json:"progress_total,omitempty"`
		ProgressCompleted *int     `json:"progress_completed,omitempty"`
		PrerequisitesMet  bool     `json:"prerequisites_met"`
		Owners            []string `json:"owners"`
	}
	out := make([]enrichedPath, len(paths))
	for i, p := range paths {
		owners := ownersByRepo[p.RepoID.String()]
		if owners == nil {
			owners = []string{}
		}
		out[i] = enrichedPath{learningPathResponse: p, PrerequisitesMet: true, Owners: owners}
		if pp, ok := progMap[p.ID.String()]; ok {
			out[i].ProgressTotal = &pp.Total
			out[i].ProgressCompleted = &pp.Completed
		}
		// Check prerequisites
		if len(p.Prerequisites) > 0 && claims != nil {
			for _, prereq := range p.Prerequisites {
				if !acquiredCompetencies[prereq] {
					out[i].PrerequisitesMet = false
					break
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, out)
}

type learningPathDetailResponse struct {
	model.LearningPath
	Modules []moduleWithSteps `json:"modules"`
}

type moduleWithSteps struct {
	model.Module
	Steps []stepSummary `json:"steps"`
}

type stepSummary struct {
	ID       string `json:"id" db:"id"`
	Slug     string `json:"slug" db:"slug"`
	Title    string `json:"title" db:"title"`
	Type     string `json:"type" db:"type"`
	Duration string `json:"estimated_duration,omitempty" db:"estimated_duration"`
	Position int    `json:"position" db:"position"`
}

func (h *Handler) GetLearningPath(w http.ResponseWriter, r *http.Request) {
	pathParam := chi.URLParam(r, "pathId")

	var lp model.LearningPath
	// Detect if param is UUID or slug
	if _, err := uuid.Parse(pathParam); err == nil {
		// Param is a UUID — lookup by ID, then redirect to slug URL
		err = h.db.GetContext(r.Context(), &lp, `
			SELECT id, repo_id, title, description, icon, tags, estimated_duration, prerequisites, slug, enabled, created_at, updated_at
			FROM learning_paths WHERE id = $1 AND deleted_at IS NULL AND enabled = true
		`, pathParam)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "learning path not found"})
			return
		}
		// 301 redirect to slug-based URL
		http.Redirect(w, r, "/api/learning-paths/"+lp.Slug, http.StatusMovedPermanently)
		return
	}

	// Param is a slug — lookup by slug
	err := h.db.GetContext(r.Context(), &lp, `
		SELECT id, repo_id, title, description, icon, tags, estimated_duration, prerequisites, slug, enabled, created_at, updated_at
		FROM learning_paths WHERE slug = $1 AND deleted_at IS NULL AND enabled = true
	`, pathParam)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "learning path not found"})
		return
	}

	var modules []model.Module
	err = h.db.SelectContext(r.Context(), &modules, `
		SELECT id, learning_path_id, slug, title, description, competencies, position, file_path, created_at, updated_at
		FROM modules WHERE learning_path_id = $1 AND deleted_at IS NULL
		ORDER BY position
	`, lp.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to fetch modules"})
		return
	}

	result := learningPathDetailResponse{LearningPath: lp}
	for _, m := range modules {
		var steps []stepSummary
		err = h.db.SelectContext(r.Context(), &steps, `
			SELECT id, slug, title, type, COALESCE(estimated_duration, '') as estimated_duration, position
			FROM steps WHERE module_id = $1 AND deleted_at IS NULL
			ORDER BY position
		`, m.ID)
		if err != nil {
			steps = []stepSummary{}
		}
		result.Modules = append(result.Modules, moduleWithSteps{Module: m, Steps: steps})
	}
	if result.Modules == nil {
		result.Modules = []moduleWithSteps{}
	}

	writeJSON(w, http.StatusOK, result)
}

type stepDetailResponse struct {
	ID           string  `json:"id" db:"id"`
	ModuleID     string  `json:"module_id" db:"module_id"`
	Title        string  `json:"title" db:"title"`
	Type         string  `json:"type" db:"type"`
	Duration     *string `json:"estimated_duration,omitempty" db:"estimated_duration"`
	ContentMD    string  `json:"content_md" db:"content_md"`
	ExerciseData any     `json:"exercise_data,omitempty" db:"exercise_data"`
	Position     int     `json:"position" db:"position"`
}

func (h *Handler) GetStep(w http.ResponseWriter, r *http.Request) {
	pathParam := chi.URLParam(r, "pathId")
	stepParam := chi.URLParam(r, "stepId")

	// Resolve the learning path first (to build redirect URL if needed)
	var lpSlug string
	if _, err := uuid.Parse(pathParam); err == nil {
		err = h.db.GetContext(r.Context(), &lpSlug, `
			SELECT slug FROM learning_paths WHERE id = $1 AND deleted_at IS NULL AND enabled = true
		`, pathParam)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "learning path not found"})
			return
		}
	} else {
		lpSlug = pathParam
	}

	var step model.Step
	if _, err := uuid.Parse(stepParam); err == nil {
		// Step param is UUID — lookup by ID
		err = h.db.GetContext(r.Context(), &step, `
			SELECT id, module_id, slug, title, type, estimated_duration, content_md,
			       COALESCE(exercise_data, 'null'::jsonb) AS exercise_data,
			       position, file_path, created_at, updated_at
			FROM steps WHERE id = $1 AND deleted_at IS NULL
		`, stepParam)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "step not found"})
			return
		}
		// 301 redirect to slug-based URL
		http.Redirect(w, r, "/api/learning-paths/"+lpSlug+"/steps/"+step.Slug, http.StatusMovedPermanently)
		return
	}

	// Step param is slug — lookup by slug within the learning path
	err := h.db.GetContext(r.Context(), &step, `
		SELECT s.id, s.module_id, s.slug, s.title, s.type, s.estimated_duration, s.content_md,
		       COALESCE(s.exercise_data, 'null'::jsonb) AS exercise_data,
		       s.position, s.file_path, s.created_at, s.updated_at
		FROM steps s
		JOIN modules m ON m.id = s.module_id AND m.deleted_at IS NULL
		JOIN learning_paths lp ON lp.id = m.learning_path_id AND lp.deleted_at IS NULL AND lp.enabled = true
		WHERE s.slug = $1 AND lp.slug = $2 AND s.deleted_at IS NULL
	`, stepParam, lpSlug)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "step not found"})
		return
	}

	response := map[string]any{
		"id":                 step.ID,
		"module_id":          step.ModuleID,
		"slug":               step.Slug,
		"title":              step.Title,
		"type":               step.Type,
		"estimated_duration": step.Duration,
		"content_md":         step.ContentMD,
		"position":           step.Position,
	}

	// Parse exercise_data JSONB into a generic object for the response
	if len(step.ExerciseData) > 0 && string(step.ExerciseData) != "null" {
		var ed any
		if err := json.Unmarshal(step.ExerciseData, &ed); err == nil {
			response["exercise_data"] = ed
		}
	}

	// For code exercises, include codebase files
	if step.Type == model.StepTypeCodeExercise {
		var files []model.CodebaseFile
		h.db.SelectContext(r.Context(), &files, `
			SELECT id, step_id, file_path, content
			FROM codebase_files WHERE step_id = $1
			ORDER BY file_path
		`, step.ID)
		if files == nil {
			files = []model.CodebaseFile{}
		}
		response["codebase_files"] = files
	}

	writeJSON(w, http.StatusOK, response)
}
