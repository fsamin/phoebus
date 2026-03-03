package syncer

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// --- parsePhoebus ---

func TestParsePhoebusValid(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "phoebus.yaml"), []byte(`
title: "Test Path"
description: "A test learning path"
icon: "🧪"
tags: ["go", "testing"]
estimated_duration: "2h"
`), 0644)

	meta, err := parsePhoebus(context.Background(), dir)
	if err != nil {
		t.Fatalf("parsePhoebus: %v", err)
	}
	if meta.Title != "Test Path" {
		t.Errorf("Title = %q, want %q", meta.Title, "Test Path")
	}
	if meta.Description != "A test learning path" {
		t.Errorf("Description = %q", meta.Description)
	}
	if len(meta.Tags) != 2 {
		t.Errorf("Tags = %v, want 2 items", meta.Tags)
	}
}

func TestParsePhoebusInvalid(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "phoebus.yaml"), []byte(`[not valid yaml`), 0644)

	_, err := parsePhoebus(context.Background(), dir)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestParsePhoebusMissing(t *testing.T) {
	dir := t.TempDir()
	_, err := parsePhoebus(context.Background(), dir)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestParsePhoebusMissingTitle(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "phoebus.yaml"), []byte(`description: "no title"`), 0644)

	_, err := parsePhoebus(context.Background(), dir)
	if err == nil {
		t.Fatal("expected error for missing title")
	}
}

// --- parseModuleIndex ---

func TestParseModuleIndexValid(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.md"), []byte(`---
title: "Module One"
description: "First module"
competencies: ["basics", "intro"]
---

Some content here.
`), 0644)

	meta, err := parseModuleIndex(context.Background(), dir)
	if err != nil {
		t.Fatalf("parseModuleIndex: %v", err)
	}
	if meta.Title != "Module One" {
		t.Errorf("Title = %q", meta.Title)
	}
	if len(meta.Competencies) != 2 {
		t.Errorf("Competencies = %v", meta.Competencies)
	}
}

func TestParseModuleIndexMissing(t *testing.T) {
	dir := t.TempDir()
	_, err := parseModuleIndex(context.Background(), dir)
	if err == nil {
		t.Fatal("expected error for missing index.md")
	}
}

func TestParseModuleIndexMissingTitle(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.md"), []byte(`---
description: "no title here"
---
`), 0644)

	_, err := parseModuleIndex(context.Background(), dir)
	if err == nil {
		t.Fatal("expected error for missing title")
	}
}

// --- splitFrontMatter ---

func TestSplitFrontMatterValid(t *testing.T) {
	fm, body, err := splitFrontMatter(`---
title: Test
---

Body content here.`)
	if err != nil {
		t.Fatalf("splitFrontMatter: %v", err)
	}
	if fm != "title: Test" {
		t.Errorf("fm = %q", fm)
	}
	if body != "Body content here." {
		t.Errorf("body = %q", body)
	}
}

func TestSplitFrontMatterNoFrontMatter(t *testing.T) {
	fm, body, err := splitFrontMatter("Just a body with no front matter")
	if err != nil {
		t.Fatalf("splitFrontMatter: %v", err)
	}
	if fm != "" {
		t.Errorf("fm should be empty, got %q", fm)
	}
	if body != "Just a body with no front matter" {
		t.Errorf("body = %q", body)
	}
}

func TestSplitFrontMatterUnterminated(t *testing.T) {
	_, _, err := splitFrontMatter("---\ntitle: Test\nno closing")
	if err == nil {
		t.Fatal("expected error for unterminated front matter")
	}
}

// --- parseStep lesson ---

func TestParseStepLesson(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "01-intro.md"), []byte(`---
title: "Introduction"
type: lesson
estimated_duration: "15m"
---

# Introduction

This is a lesson about testing.
`), 0644)

	meta, body, exerciseData, err := parseStep(context.Background(), filepath.Join(dir, "01-intro.md"))
	if err != nil {
		t.Fatalf("parseStep: %v", err)
	}
	if meta.Title != "Introduction" {
		t.Errorf("Title = %q", meta.Title)
	}
	if meta.Type != "lesson" {
		t.Errorf("Type = %q", meta.Type)
	}
	if body == "" {
		t.Error("body should not be empty")
	}
	if exerciseData != nil {
		t.Error("lesson should have nil exercise data")
	}
}

// --- parseStep quiz ---

func TestParseStepQuizMultipleChoice(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "01-quiz.md"), []byte(`---
title: "Basic Quiz"
type: quiz
estimated_duration: "10m"
---

## [multiple-choice] What is Go?

- [x] A programming language
- [ ] A database
- [ ] An operating system

> Go is a programming language created at Google.
`), 0644)

	meta, _, exerciseData, err := parseStep(context.Background(), filepath.Join(dir, "01-quiz.md"))
	if err != nil {
		t.Fatalf("parseStep: %v", err)
	}
	if meta.Type != "quiz" {
		t.Fatalf("Type = %q", meta.Type)
	}

	var data map[string]any
	json.Unmarshal(exerciseData, &data)
	questions := data["questions"].([]any)
	if len(questions) != 1 {
		t.Fatalf("questions = %d, want 1", len(questions))
	}
	q := questions[0].(map[string]any)
	if q["type"] != "multiple-choice" {
		t.Errorf("question type = %v", q["type"])
	}
	choices := q["choices"].([]any)
	if len(choices) != 3 {
		t.Errorf("choices = %d, want 3", len(choices))
	}
	// First choice should be correct
	first := choices[0].(map[string]any)
	if first["correct"] != true {
		t.Error("first choice should be correct")
	}
}

func TestParseStepQuizShortAnswer(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "02-quiz.md"), []byte(`---
title: "Short Answer"
type: quiz
estimated_duration: "5m"
---

## [short-answer] What command compiles Go code?

    go build
`), 0644)

	_, _, exerciseData, err := parseStep(context.Background(), filepath.Join(dir, "02-quiz.md"))
	if err != nil {
		t.Fatalf("parseStep: %v", err)
	}

	var data map[string]any
	json.Unmarshal(exerciseData, &data)
	questions := data["questions"].([]any)
	q := questions[0].(map[string]any)
	if q["pattern"] != "go build" {
		t.Errorf("pattern = %v, want %q", q["pattern"], "go build")
	}
}

func TestParseStepQuizInvalidRegex(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "03-quiz.md"), []byte(`---
title: "Bad Regex"
type: quiz
estimated_duration: "5m"
---

## [short-answer] Question

    [invalid(regex
`), 0644)

	_, _, _, err := parseStep(context.Background(), filepath.Join(dir, "03-quiz.md"))
	if err == nil {
		t.Fatal("expected error for invalid regex pattern")
	}
}

// --- parseStep terminal-exercise ---

func TestParseStepTerminalExercise(t *testing.T) {
	dir := t.TempDir()
	content := "---\ntitle: \"Terminal Test\"\ntype: terminal-exercise\nestimated_duration: \"10m\"\n---\n\nIntro text here.\n\n## Step 1\n\nRun the following:\n\n```console\necho hello\n```\n\nExpected output:\n\n```output\nhello\n```\n\n- [x] `echo hello` — prints hello\n- [ ] `echo world` — prints world\n"
	os.WriteFile(filepath.Join(dir, "01-terminal.md"), []byte(content), 0644)

	meta, _, exerciseData, err := parseStep(context.Background(), filepath.Join(dir, "01-terminal.md"))
	if err != nil {
		t.Fatalf("parseStep: %v", err)
	}
	if meta.Type != "terminal-exercise" {
		t.Fatalf("Type = %q", meta.Type)
	}

	var data map[string]any
	json.Unmarshal(exerciseData, &data)
	steps := data["steps"].([]any)
	if len(steps) != 1 {
		t.Fatalf("steps = %d, want 1", len(steps))
	}
	if data["introduction"] != "Intro text here." {
		t.Errorf("introduction = %v", data["introduction"])
	}
}

// --- parseStep missing required fields ---

func TestParseStepMissingTitle(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "01-bad.md"), []byte(`---
type: lesson
---
`), 0644)

	_, _, _, err := parseStep(context.Background(), filepath.Join(dir, "01-bad.md"))
	if err == nil {
		t.Fatal("expected error for missing title")
	}
}

func TestParseStepUnknownType(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "01-bad.md"), []byte(`---
title: "Test"
type: unknown-type
---
`), 0644)

	_, _, _, err := parseStep(context.Background(), filepath.Join(dir, "01-bad.md"))
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
}

// --- convertYAMLToJSON ---

func TestConvertYAMLToJSON(t *testing.T) {
	input := map[interface{}]interface{}{
		"key1": "value1",
		"nested": map[interface{}]interface{}{
			"inner": 42,
		},
		"list": []interface{}{"a", "b"},
	}

	result := convertYAMLToJSON(input)
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", result)
	}
	if m["key1"] != "value1" {
		t.Errorf("key1 = %v", m["key1"])
	}
	nested, ok := m["nested"].(map[string]interface{})
	if !ok {
		t.Fatalf("nested type = %T", m["nested"])
	}
	if nested["inner"] != 42 {
		t.Errorf("nested.inner = %v", nested["inner"])
	}

	// Should be JSON-marshallable now
	if _, err := json.Marshal(result); err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
}
