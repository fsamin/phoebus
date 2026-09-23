package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/fsamin/phoebus/internal/model"
	"github.com/google/uuid"
)

// sanitizeExerciseData exists so a learner cannot read the answer off the wire.
// Each case below asserts the answer is gone AND that what the UI needs to
// render the exercise survives — stripping too much would break the page.
func TestSanitizeExerciseDataQuiz(t *testing.T) {
	raw := json.RawMessage(`{"questions":[
		{"text":"Which flag?","type":"multiple-choice","multi_select":false,
		 "choices":[{"text":"-a","correct":true},{"text":"-b","correct":false}],
		 "explanation":"-a means all"},
		{"text":"Type the command","type":"short-answer","pattern":"^docker ps$","explanation":"docker ps"}
	]}`)

	out, ok := sanitizeExerciseData(model.StepTypeQuiz, raw)
	if !ok {
		t.Fatal("sanitizeExerciseData returned not ok")
	}
	encoded := mustEncode(t, out)

	for _, leak := range []string{"correct", "pattern", "explanation", "-a means all", "^docker ps$"} {
		if strings.Contains(encoded, leak) {
			t.Errorf("quiz payload still leaks %q: %s", leak, encoded)
		}
	}
	// The question and its choices must still be renderable.
	for _, kept := range []string{"Which flag?", "multi_select", `"-a"`, "Type the command"} {
		if !strings.Contains(encoded, kept) {
			t.Errorf("quiz payload lost %q, the UI needs it: %s", kept, encoded)
		}
	}
}

func TestSanitizeExerciseDataTerminal(t *testing.T) {
	raw := json.RawMessage(`{"introduction":"intro","steps":[
		{"context":"ctx","prompt":"$ ","output":"out","proposals":[
			{"command":"ls -l","correct":true,"explanation":"lists files"},
			{"command":"rm -rf /","correct":false,"explanation":"do not"}
		]}
	]}`)

	out, ok := sanitizeExerciseData(model.StepTypeTerminalExercise, raw)
	if !ok {
		t.Fatal("sanitizeExerciseData returned not ok")
	}
	encoded := mustEncode(t, out)

	for _, leak := range []string{"correct", "explanation", "lists files"} {
		if strings.Contains(encoded, leak) {
			t.Errorf("terminal payload still leaks %q: %s", leak, encoded)
		}
	}
	for _, kept := range []string{"ls -l", "rm -rf /", "intro"} {
		if !strings.Contains(encoded, kept) {
			t.Errorf("terminal payload lost %q, the UI needs it: %s", kept, encoded)
		}
	}
}

// In identify-and-fix mode, locating the faulty lines IS the exercise, so
// target.lines must not be sent. target.file must be, the editor opens it.
func TestSanitizeExerciseDataCodeIdentifyAndFix(t *testing.T) {
	raw := json.RawMessage(`{"mode":"A","description":"fix it",
		"target":{"file":"main.go","lines":[12,13]},
		"patches":[{"label":"Patch 1","correct":true,"explanation":"why","diff":"--- a\n+++ b"}]}`)

	out, ok := sanitizeExerciseData(model.StepTypeCodeExercise, raw)
	if !ok {
		t.Fatal("sanitizeExerciseData returned not ok")
	}
	encoded := mustEncode(t, out)

	for _, leak := range []string{"lines", "correct", "explanation", "12", "why"} {
		if strings.Contains(encoded, leak) {
			t.Errorf("code exercise payload still leaks %q: %s", leak, encoded)
		}
	}
	for _, kept := range []string{"main.go", "Patch 1", "fix it", "+++ b"} {
		if !strings.Contains(encoded, kept) {
			t.Errorf("code exercise payload lost %q, the UI needs it: %s", kept, encoded)
		}
	}
}

// Mode B has no identify phase: the target lines are a hint, not an answer.
func TestSanitizeExerciseDataCodeChooseTheFixKeepsTargetLines(t *testing.T) {
	raw := json.RawMessage(`{"mode":"B","description":"pick one",
		"target":{"file":"main.go","lines":[7]},
		"patches":[{"label":"Patch 1","correct":true,"explanation":"why","diff":"d"}]}`)

	out, ok := sanitizeExerciseData(model.StepTypeCodeExercise, raw)
	if !ok {
		t.Fatal("sanitizeExerciseData returned not ok")
	}
	encoded := mustEncode(t, out)

	if !strings.Contains(encoded, "lines") {
		t.Errorf("mode B should keep target.lines as a hint: %s", encoded)
	}
	if strings.Contains(encoded, `"correct"`) {
		t.Errorf("mode B still leaks the correct patch: %s", encoded)
	}
}

func TestSanitizeExerciseDataLessonIsUntouched(t *testing.T) {
	raw := json.RawMessage(`{"anything":"kept"}`)
	out, ok := sanitizeExerciseData(model.StepTypeLesson, raw)
	if !ok {
		t.Fatal("sanitizeExerciseData returned not ok")
	}
	if !strings.Contains(mustEncode(t, out), "kept") {
		t.Error("lesson payload should pass through untouched")
	}
}

func TestSanitizeExerciseDataInvalidJSON(t *testing.T) {
	if _, ok := sanitizeExerciseData(model.StepTypeQuiz, json.RawMessage(`not json`)); ok {
		t.Error("invalid exercise data should be reported as unusable, not served raw")
	}
}

// End to end: a learner fetching a quiz step over the API must not receive the
// answers. This is the regression that matters — the sanitizer being correct is
// worthless if GetStep stops calling it.
func TestGetStepDoesNotLeakQuizAnswers(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	pathSlug, stepSlug := seedQuizStep(t)
	cookie := loginAs(t, model.RoleLearner)

	resp := doRequest(t, srv, "GET", fmt.Sprintf("/api/learning-paths/%s/steps/%s", pathSlug, stepSlug), nil, cookie)
	if resp.StatusCode != 200 {
		t.Fatalf("GET step: status = %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	payload := string(body)

	for _, leak := range []string{"correct", "42 is the answer", "^42$"} {
		if strings.Contains(payload, leak) {
			t.Errorf("step response leaks %q to the learner: %s", leak, payload)
		}
	}
	if !strings.Contains(payload, "exercise_data") || !strings.Contains(payload, "What is the answer?") {
		t.Errorf("step response lost the question itself: %s", payload)
	}
}

func seedQuizStep(t *testing.T) (pathSlug, stepSlug string) {
	t.Helper()
	repoID, pathID, modID, stepID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	suffix := pathID.String()[:8]
	pathSlug = "quiz-leak-path-" + suffix
	stepSlug = "quiz-leak-step-" + suffix

	testDB.MustExec(`INSERT INTO git_repositories (id, clone_url, branch, auth_type, webhook_uuid, sync_status, created_at, updated_at)
		VALUES ($1, 'https://github.com/test/quiz-leak.git', 'main', 'none', $2, 'synced', now(), now())`, repoID, uuid.New())
	testDB.MustExec(`INSERT INTO learning_paths (id, repo_id, title, description, tags, prerequisites, file_path, slug, enabled, created_at, updated_at)
		VALUES ($1, $2, 'Quiz Leak Path', 'x', '{}', '{}', 'quiz-leak/', $3, true, now(), now())`, pathID, repoID, pathSlug)
	testDB.MustExec(`INSERT INTO modules (id, learning_path_id, title, description, competencies, position, file_path, slug, created_at, updated_at)
		VALUES ($1, $2, 'Quiz Leak Module', 'x', '{}', 0, 'quiz-leak/mod/', $3, now(), now())`, modID, pathID, "quiz-leak-mod-"+suffix)
	testDB.MustExec(`INSERT INTO steps (id, module_id, title, type, content_md, exercise_data, position, file_path, slug, created_at, updated_at)
		VALUES ($1, $2, 'Quiz Leak Step', 'quiz', '# Quiz', $3::jsonb, 0, 'quiz-leak/mod/01.md', $4, now(), now())`,
		stepID, modID, `{"questions":[
			{"text":"What is the answer?","type":"multiple-choice","choices":[{"text":"41","correct":false},{"text":"42","correct":true}],"explanation":"42 is the answer"},
			{"text":"Type it","type":"short-answer","pattern":"^42$","explanation":"42 is the answer"}
		]}`, stepSlug)

	return pathSlug, stepSlug
}

func mustEncode(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}
