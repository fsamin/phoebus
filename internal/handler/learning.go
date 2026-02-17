package handler

import (
	"encoding/json"
	"net/http"

	"github.com/fsamin/phoebus/internal/model"
	"github.com/go-chi/chi/v5"
)

// --- Learning Paths (public read API) ---

type learningPathResponse struct {
	model.LearningPath
	ModuleCount int `json:"module_count" db:"module_count"`
	StepCount   int `json:"step_count" db:"step_count"`
}

func (h *Handler) ListLearningPaths(w http.ResponseWriter, r *http.Request) {
	var paths []learningPathResponse
	err := h.db.SelectContext(r.Context(), &paths, `
		SELECT lp.id, lp.repo_id, lp.title, lp.description, lp.icon, lp.tags,
		       lp.estimated_duration, lp.prerequisites, lp.created_at, lp.updated_at,
		       COUNT(DISTINCT m.id) AS module_count,
		       COUNT(DISTINCT s.id) AS step_count
		FROM learning_paths lp
		LEFT JOIN modules m ON m.learning_path_id = lp.id
		LEFT JOIN steps s ON s.module_id = m.id AND s.deleted_at IS NULL
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
			JOIN modules m ON m.id = s.module_id
			LEFT JOIN progress p ON p.step_id = s.id AND p.user_id = $1
			WHERE s.deleted_at IS NULL AND EXISTS (
				SELECT 1 FROM progress p2 JOIN steps s2 ON s2.id = p2.step_id
				JOIN modules m2 ON m2.id = s2.module_id
				WHERE p2.user_id = $1 AND m2.learning_path_id = m.learning_path_id
			)
			GROUP BY m.learning_path_id
		`, claims.UserID)
	}
	progMap := map[string]pathProg{}
	for _, pp := range userProgress {
		progMap[pp.PathID] = pp
	}

	type enrichedPath struct {
		learningPathResponse
		ProgressTotal     *int `json:"progress_total,omitempty"`
		ProgressCompleted *int `json:"progress_completed,omitempty"`
	}
	out := make([]enrichedPath, len(paths))
	for i, p := range paths {
		out[i] = enrichedPath{learningPathResponse: p}
		if pp, ok := progMap[p.ID.String()]; ok {
			out[i].ProgressTotal = &pp.Total
			out[i].ProgressCompleted = &pp.Completed
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
	Title    string `json:"title" db:"title"`
	Type     string `json:"type" db:"type"`
	Duration string `json:"estimated_duration,omitempty" db:"estimated_duration"`
	Position int    `json:"position" db:"position"`
}

func (h *Handler) GetLearningPath(w http.ResponseWriter, r *http.Request) {
	pathID := chi.URLParam(r, "pathId")

	var lp model.LearningPath
	err := h.db.GetContext(r.Context(), &lp, `
		SELECT id, repo_id, title, description, icon, tags, estimated_duration, prerequisites, created_at, updated_at
		FROM learning_paths WHERE id = $1
	`, pathID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "learning path not found"})
		return
	}

	var modules []model.Module
	err = h.db.SelectContext(r.Context(), &modules, `
		SELECT id, learning_path_id, title, description, competencies, position, file_path, created_at, updated_at
		FROM modules WHERE learning_path_id = $1
		ORDER BY position
	`, pathID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to fetch modules"})
		return
	}

	result := learningPathDetailResponse{LearningPath: lp}
	for _, m := range modules {
		var steps []stepSummary
		err = h.db.SelectContext(r.Context(), &steps, `
			SELECT id, title, type, COALESCE(estimated_duration, '') as estimated_duration, position
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
	stepID := chi.URLParam(r, "stepId")

	var step model.Step
	err := h.db.GetContext(r.Context(), &step, `
		SELECT id, module_id, title, type, estimated_duration, content_md,
		       COALESCE(exercise_data, 'null'::jsonb) AS exercise_data,
		       position, file_path, created_at, updated_at
		FROM steps WHERE id = $1 AND deleted_at IS NULL
	`, stepID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "step not found"})
		return
	}

	response := map[string]any{
		"id":                 step.ID,
		"module_id":          step.ModuleID,
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
