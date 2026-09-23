package handler

import (
	"encoding/json"

	"github.com/fsamin/phoebus/internal/model"
)

// sanitizeExerciseData strips the answers out of an exercise payload before it
// is served to a learner. Attempts are validated server-side against the data
// stored in the database, so the client never needs the answers — sending them
// would let anyone read the solution from the network tab.
//
// What is removed, per exercise type:
//   - quiz: the `correct` flag on every choice, the short-answer `pattern`, and
//     the per-question `explanation` (revealed in the attempt response instead).
//   - terminal-exercise: the `correct` flag and `explanation` of every proposal.
//   - code-exercise: the `correct` flag and `explanation` of every patch, plus
//     `target.lines` in identify-and-fix mode, where finding those lines is the
//     exercise itself. `target.file` is kept — the editor needs it.
func sanitizeExerciseData(stepType model.StepType, raw json.RawMessage) (any, bool) {
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, false
	}

	switch stepType {
	case model.StepTypeQuiz:
		for _, q := range mapsIn(data["questions"]) {
			delete(q, "pattern")
			delete(q, "explanation")
			for _, c := range mapsIn(q["choices"]) {
				delete(c, "correct")
			}
		}

	case model.StepTypeTerminalExercise:
		for _, s := range mapsIn(data["steps"]) {
			for _, p := range mapsIn(s["proposals"]) {
				delete(p, "correct")
				delete(p, "explanation")
			}
		}

	case model.StepTypeCodeExercise:
		for _, p := range mapsIn(data["patches"]) {
			delete(p, "correct")
			delete(p, "explanation")
		}
		// Mode B ("choose the fix") has no identify phase, so the target lines
		// are a hint rather than an answer and stay in the payload.
		if mode, _ := data["mode"].(string); mode != "B" {
			if target, ok := data["target"].(map[string]any); ok {
				delete(target, "lines")
			}
		}
	}

	return data, true
}

// mapsIn returns the JSON objects contained in a decoded JSON array, skipping
// anything that is not an object.
func mapsIn(v any) []map[string]any {
	items, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}
