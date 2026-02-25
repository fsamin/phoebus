package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"regexp"
	"time"

	"github.com/fsamin/phoebus/internal/model"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// GetProgress returns all progress records for the authenticated user,
// optionally filtered by learning_path_id query parameter.
func (h *Handler) GetProgress(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())

	pathID := r.URL.Query().Get("learning_path_id")
	if pathID != "" {
		// Per-path progress: all step progress for steps in this learning path
		var progress []model.Progress
		err := h.db.SelectContext(r.Context(), &progress, `
			SELECT p.id, p.user_id, p.step_id, p.status, p.completed_at, p.created_at, p.updated_at
			FROM progress p
			JOIN steps s ON s.id = p.step_id
			JOIN modules m ON m.id = s.module_id AND m.deleted_at IS NULL
			WHERE p.user_id = $1 AND m.learning_path_id = $2 AND s.deleted_at IS NULL
			ORDER BY p.created_at
		`, claims.UserID, pathID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to fetch progress"})
			return
		}
		if progress == nil {
			progress = []model.Progress{}
		}
		writeJSON(w, http.StatusOK, progress)
		return
	}

	// All progress for the user
	var progress []model.Progress
	err := h.db.SelectContext(r.Context(), &progress, `
		SELECT id, user_id, step_id, status, completed_at, created_at, updated_at
		FROM progress WHERE user_id = $1
		ORDER BY updated_at DESC
	`, claims.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to fetch progress"})
		return
	}
	if progress == nil {
		progress = []model.Progress{}
	}
	writeJSON(w, http.StatusOK, progress)
}

// UpdateProgress sets a step's progress to in_progress or completed (for lessons).
// For exercises, completion is handled by the exercise validation endpoint.
func (h *Handler) UpdateProgress(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())

	var req struct {
		StepID string `json:"step_id"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	stepID, err := uuid.Parse(req.StepID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid step_id"})
		return
	}

	// Verify step exists
	var step model.Step
	err = h.db.GetContext(r.Context(), &step, `
		SELECT id, type FROM steps WHERE id = $1 AND deleted_at IS NULL
	`, stepID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "step not found"})
		return
	}

	status := model.ProgressStatus(req.Status)

	switch status {
	case model.ProgressInProgress:
		// Upsert: set to in_progress (only if not already completed)
		_, err = h.db.ExecContext(r.Context(), `
			INSERT INTO progress (user_id, step_id, status)
			VALUES ($1, $2, 'in_progress')
			ON CONFLICT (user_id, step_id)
			DO UPDATE SET status = CASE WHEN progress.status = 'completed' THEN 'completed' ELSE 'in_progress' END,
			             updated_at = now()
		`, claims.UserID, stepID)

	case model.ProgressCompleted:
		// Only lessons can be marked completed via this endpoint.
		// Exercises are completed through the exercise validation endpoint.
		if step.Type != model.StepTypeLesson {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "exercises are completed through the attempt endpoint"})
			return
		}
		now := time.Now()
		_, err = h.db.ExecContext(r.Context(), `
			INSERT INTO progress (user_id, step_id, status, completed_at)
			VALUES ($1, $2, 'completed', $3)
			ON CONFLICT (user_id, step_id)
			DO UPDATE SET status = 'completed', completed_at = $3, updated_at = now()
		`, claims.UserID, stepID, now)

	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "status must be 'in_progress' or 'completed'"})
		return
	}

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update progress"})
		return
	}

	// Return updated progress
	var progress model.Progress
	err = h.db.GetContext(r.Context(), &progress, `
		SELECT id, user_id, step_id, status, completed_at, created_at, updated_at
		FROM progress WHERE user_id = $1 AND step_id = $2
	`, claims.UserID, stepID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to fetch progress"})
		return
	}
	writeJSON(w, http.StatusOK, progress)
}

// GetStepAttempts returns exercise attempts for a specific step for the authenticated user.
func (h *Handler) GetStepAttempts(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	stepID := chi.URLParam(r, "stepId")

	var attempts []model.ExerciseAttempt
	err := h.db.SelectContext(r.Context(), &attempts, `
		SELECT id, user_id, step_id, answers, is_correct, created_at
		FROM exercise_attempts
		WHERE user_id = $1 AND step_id = $2
		ORDER BY created_at
	`, claims.UserID, stepID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to fetch attempts"})
		return
	}
	if attempts == nil {
		attempts = []model.ExerciseAttempt{}
	}
	writeJSON(w, http.StatusOK, attempts)
}

// ResetExercise resets progress to in_progress and clears completed_at.
// Previous exercise_attempts are preserved (historical data is never deleted).
func (h *Handler) ResetExercise(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	stepID := chi.URLParam(r, "stepId")

	// Verify step exists and is an exercise
	var step model.Step
	err := h.db.GetContext(r.Context(), &step, `
		SELECT id, type FROM steps WHERE id = $1 AND deleted_at IS NULL
	`, stepID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "step not found"})
		return
	}
	if step.Type == model.StepTypeLesson {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "lessons cannot be reset"})
		return
	}

	// Reset progress to in_progress, clear completed_at
	result, err := h.db.ExecContext(r.Context(), `
		UPDATE progress
		SET status = 'in_progress', completed_at = NULL, updated_at = now()
		WHERE user_id = $1 AND step_id = $2
	`, claims.UserID, stepID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to reset progress"})
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		// No progress record exists — nothing to reset
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no progress found for this step"})
		return
	}

	// Return updated progress
	var progress model.Progress
	_ = h.db.GetContext(r.Context(), &progress, `
		SELECT id, user_id, step_id, status, completed_at, created_at, updated_at
		FROM progress WHERE user_id = $1 AND step_id = $2
	`, claims.UserID, stepID)
	writeJSON(w, http.StatusOK, progress)
}

// Logout clears the session cookie.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "phoebus_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1, // Delete cookie
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}

// --- exercise attempt validation ---

// SubmitAttempt validates an exercise attempt server-side and records it.
// The correct answers are never exposed to the client.
func (h *Handler) SubmitAttempt(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	stepID := chi.URLParam(r, "stepId")

	// Fetch step with exercise data
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

	if step.Type == model.StepTypeLesson {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "lessons do not have exercises"})
		return
	}

	// Parse request body
	var rawBody json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&rawBody); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Ensure progress is at least in_progress
	h.db.ExecContext(r.Context(), `
		INSERT INTO progress (user_id, step_id, status)
		VALUES ($1, $2, 'in_progress')
		ON CONFLICT (user_id, step_id)
		DO UPDATE SET updated_at = now()
		WHERE progress.status = 'not_started'
	`, claims.UserID, stepID)

	// Validate based on exercise type
	var result attemptResult
	switch step.Type {
	case model.StepTypeQuiz:
		result, err = validateQuizAttempt(rawBody, step.ExerciseData)
	case model.StepTypeTerminalExercise:
		result, err = validateTerminalAttempt(rawBody, step.ExerciseData)
	case model.StepTypeCodeExercise:
		result, err = validateCodeExerciseAttempt(rawBody, step.ExerciseData)
	}
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Record the attempt
	answersJSON, _ := json.Marshal(result.answers)
	_, err = h.db.ExecContext(r.Context(), `
		INSERT INTO exercise_attempts (user_id, step_id, answers, is_correct)
		VALUES ($1, $2, $3, $4)
	`, claims.UserID, step.ID, answersJSON, result.isCorrect)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to record attempt"})
		return
	}

	// If correct and this completes the exercise, check completion
	if result.shouldComplete {
		now := time.Now()
		h.db.ExecContext(r.Context(), `
			INSERT INTO progress (user_id, step_id, status, completed_at)
			VALUES ($1, $2, 'completed', $3)
			ON CONFLICT (user_id, step_id)
			DO UPDATE SET status = 'completed', completed_at = $3, updated_at = now()
		`, claims.UserID, step.ID, now)
	}

	writeJSON(w, http.StatusOK, result.response)
}

type attemptResult struct {
	isCorrect      bool
	shouldComplete bool
	answers        map[string]any
	response       map[string]any
}

// --- Quiz validation ---

func validateQuizAttempt(body json.RawMessage, exerciseData json.RawMessage) (attemptResult, error) {
	var req struct {
		QuestionIndex int      `json:"question_index"`
		Type          string   `json:"type"`
		Selected      []string `json:"selected"` // for multiple-choice
		Answer        string   `json:"answer"`    // for short-answer
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return attemptResult{}, err
	}

	var data struct {
		Questions []struct {
			Text        string `json:"text"`
			Type        string `json:"type"`
			Explanation string `json:"explanation"`
			MultiSelect bool   `json:"multi_select"`
			Choices     []struct {
				Text    string `json:"text"`
				Correct bool   `json:"correct"`
			} `json:"choices"`
			Pattern string `json:"pattern"`
		} `json:"questions"`
	}
	if err := json.Unmarshal(exerciseData, &data); err != nil {
		return attemptResult{}, err
	}

	if req.QuestionIndex < 0 || req.QuestionIndex >= len(data.Questions) {
		return attemptResult{}, sql.ErrNoRows
	}

	q := data.Questions[req.QuestionIndex]
	isCorrect := false
	answers := map[string]any{
		"question_index": req.QuestionIndex,
		"type":           q.Type,
	}
	response := map[string]any{
		"question_index": req.QuestionIndex,
		"explanation":    q.Explanation,
	}

	switch q.Type {
	case "multiple-choice":
		answers["selected"] = req.Selected
		// Check if selected answers match correct answers exactly
		correctSet := map[string]bool{}
		for _, c := range q.Choices {
			if c.Correct {
				correctSet[c.Text] = true
			}
		}
		selectedSet := map[string]bool{}
		for _, s := range req.Selected {
			selectedSet[s] = true
		}
		isCorrect = len(correctSet) == len(selectedSet)
		if isCorrect {
			for k := range correctSet {
				if !selectedSet[k] {
					isCorrect = false
					break
				}
			}
		}

		// Build feedback: for each choice, indicate if it's correct
		var feedback []map[string]any
		for _, c := range q.Choices {
			feedback = append(feedback, map[string]any{
				"text":     c.Text,
				"correct":  c.Correct,
				"selected": selectedSet[c.Text],
			})
		}
		response["choices_feedback"] = feedback

	case "short-answer":
		answers["answer"] = req.Answer
		re, err := regexp.Compile("(?i)" + q.Pattern)
		if err != nil {
			return attemptResult{}, err
		}
		isCorrect = re.MatchString(req.Answer)
		if !isCorrect {
			response["hint"] = "Try again"
		}
	}

	answers["is_correct"] = isCorrect
	response["is_correct"] = isCorrect

	// A quiz is completed when the last question is submitted (regardless of correctness)
	isLastQuestion := req.QuestionIndex == len(data.Questions)-1

	return attemptResult{
		isCorrect:      isCorrect,
		shouldComplete: isLastQuestion,
		answers:        answers,
		response:       response,
	}, nil
}

// --- Terminal exercise validation ---

func validateTerminalAttempt(body json.RawMessage, exerciseData json.RawMessage) (attemptResult, error) {
	var req struct {
		StepNumber      int    `json:"step_number"`
		SelectedCommand string `json:"selected_command"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return attemptResult{}, err
	}

	var data struct {
		Steps []struct {
			Proposals []struct {
				Command     string `json:"command"`
				Correct     bool   `json:"correct"`
				Explanation string `json:"explanation"`
			} `json:"proposals"`
			Output string `json:"output"`
		} `json:"steps"`
	}
	if err := json.Unmarshal(exerciseData, &data); err != nil {
		return attemptResult{}, err
	}

	stepIdx := req.StepNumber - 1
	if stepIdx < 0 || stepIdx >= len(data.Steps) {
		return attemptResult{}, sql.ErrNoRows
	}

	termStep := data.Steps[stepIdx]
	isCorrect := false
	var explanation string
	for _, p := range termStep.Proposals {
		if p.Command == req.SelectedCommand {
			isCorrect = p.Correct
			explanation = p.Explanation
			break
		}
	}

	answers := map[string]any{
		"step_number":      req.StepNumber,
		"selected_command": req.SelectedCommand,
		"is_correct":       isCorrect,
	}

	response := map[string]any{
		"is_correct":  isCorrect,
		"step_number": req.StepNumber,
		"explanation": explanation,
	}
	if isCorrect {
		response["output"] = termStep.Output
	}

	// Terminal exercise is completed when last step is answered correctly
	isLastStep := stepIdx == len(data.Steps)-1

	return attemptResult{
		isCorrect:      isCorrect,
		shouldComplete: isCorrect && isLastStep,
		answers:        answers,
		response:       response,
	}, nil
}

// --- Code exercise validation ---

func validateCodeExerciseAttempt(body json.RawMessage, exerciseData json.RawMessage) (attemptResult, error) {
	var req struct {
		Phase         string `json:"phase"`          // "identify" or "fix"
		SelectedLines []int  `json:"selected_lines"` // for identify phase
		SelectedPatch string `json:"selected_patch"` // patch label for fix phase
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return attemptResult{}, err
	}

	var data struct {
		Mode   string `json:"mode"`
		Target struct {
			File  string `json:"file"`
			Lines []int  `json:"lines"`
		} `json:"target"`
		Patches []struct {
			Label       string `json:"label"`
			Correct     bool   `json:"correct"`
			Explanation string `json:"explanation"`
			Diff        string `json:"diff"`
		} `json:"patches"`
	}
	if err := json.Unmarshal(exerciseData, &data); err != nil {
		return attemptResult{}, err
	}

	answers := map[string]any{
		"phase": req.Phase,
	}
	response := map[string]any{
		"phase": req.Phase,
	}

	isCorrect := false

	switch req.Phase {
	case "identify":
		answers["selected_lines"] = req.SelectedLines

		// Compare selected lines with target lines (exact match)
		targetSet := map[int]bool{}
		for _, l := range data.Target.Lines {
			targetSet[l] = true
		}
		selectedSet := map[int]bool{}
		for _, l := range req.SelectedLines {
			selectedSet[l] = true
		}

		isCorrect = len(targetSet) == len(selectedSet)
		if isCorrect {
			for k := range targetSet {
				if !selectedSet[k] {
					isCorrect = false
					break
				}
			}
		}

		// Progressive feedback
		matchCount := 0
		for _, l := range req.SelectedLines {
			if targetSet[l] {
				matchCount++
			}
		}
		response["matched"] = matchCount
		response["total"] = len(data.Target.Lines)
		if !isCorrect && matchCount > 0 {
			response["hint"] = "Right area, refine your selection"
		}

	case "fix":
		answers["selected_patch"] = req.SelectedPatch

		for _, p := range data.Patches {
			if p.Label == req.SelectedPatch {
				isCorrect = p.Correct
				response["explanation"] = p.Explanation
				break
			}
		}
	}

	answers["is_correct"] = isCorrect
	response["is_correct"] = isCorrect

	// Code exercise is completed when the fix phase is answered correctly
	shouldComplete := req.Phase == "fix" && isCorrect
	// Mode B (choose-fix) has no identify phase, so fix is the only phase
	if data.Mode == "B" && req.Phase == "fix" && isCorrect {
		shouldComplete = true
	}

	return attemptResult{
		isCorrect:      isCorrect,
		shouldComplete: shouldComplete,
		answers:        answers,
		response:       response,
	}, nil
}
