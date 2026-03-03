package syncer

import (
	"os"
	"path/filepath"
	"testing"
)

// --- ValidateContent ---

func createValidContent(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "phoebus.yaml"), []byte(`title: "Test Path"
description: "desc"
`), 0644)

	modDir := filepath.Join(dir, "01-module")
	os.MkdirAll(modDir, 0755)
	os.WriteFile(filepath.Join(modDir, "index.md"), []byte(`---
title: "Module One"
description: "first module"
---
`), 0644)
	os.WriteFile(filepath.Join(modDir, "01-lesson.md"), []byte(`---
title: "Lesson One"
type: lesson
estimated_duration: "10m"
---

Content here.
`), 0644)

	return dir
}

func TestValidateContentValid(t *testing.T) {
	dir := createValidContent(t)
	errs := ValidateContent(dir)
	if len(errs) != 0 {
		t.Fatalf("expected 0 errors, got %d: %v", len(errs), errs)
	}
}

func TestValidateContentMissingPhoebus(t *testing.T) {
	dir := t.TempDir()
	errs := ValidateContent(dir)
	if len(errs) == 0 {
		t.Fatal("expected errors for missing phoebus.yaml")
	}
}

func TestValidateContentMissingModuleIndex(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "phoebus.yaml"), []byte(`title: "Test"`), 0644)

	modDir := filepath.Join(dir, "01-module")
	os.MkdirAll(modDir, 0755)
	// No index.md
	os.WriteFile(filepath.Join(modDir, "01-step.md"), []byte(`---
title: "Step"
type: lesson
---
`), 0644)

	errs := ValidateContent(dir)
	if len(errs) == 0 {
		t.Fatal("expected errors for missing index.md")
	}
}

func TestValidateContentInvalidStepFrontMatter(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "phoebus.yaml"), []byte(`title: "Test"`), 0644)

	modDir := filepath.Join(dir, "01-module")
	os.MkdirAll(modDir, 0755)
	os.WriteFile(filepath.Join(modDir, "index.md"), []byte(`---
title: "Module"
---
`), 0644)
	os.WriteFile(filepath.Join(modDir, "01-step.md"), []byte(`---
type: lesson
---
`), 0644) // Missing title

	errs := ValidateContent(dir)
	if len(errs) == 0 {
		t.Fatal("expected errors for invalid step front matter")
	}
}

// --- discoverLearningPaths ---

func TestDiscoverLearningPathsRoot(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "phoebus.yaml"), []byte(`title: "Root"`), 0644)

	paths, err := discoverLearningPaths(dir)
	if err != nil {
		t.Fatalf("discoverLearningPaths: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("got %d paths, want 1", len(paths))
	}
	if paths[0].filePath != "" {
		t.Errorf("filePath = %q, want empty for root", paths[0].filePath)
	}
}

func TestDiscoverLearningPathsSubdirs(t *testing.T) {
	dir := t.TempDir()
	// No root phoebus.yaml

	for _, name := range []string{"path-a", "path-b", "path-c"} {
		sub := filepath.Join(dir, name)
		os.MkdirAll(sub, 0755)
		os.WriteFile(filepath.Join(sub, "phoebus.yaml"), []byte(`title: "`+name+`"`), 0644)
	}

	paths, err := discoverLearningPaths(dir)
	if err != nil {
		t.Fatalf("discoverLearningPaths: %v", err)
	}
	if len(paths) != 3 {
		t.Fatalf("got %d paths, want 3", len(paths))
	}
	// Should be sorted
	if paths[0].filePath != "path-a" {
		t.Errorf("first path = %q, want %q", paths[0].filePath, "path-a")
	}
}

func TestDiscoverLearningPathsNone(t *testing.T) {
	dir := t.TempDir()
	_, err := discoverLearningPaths(dir)
	if err == nil {
		t.Fatal("expected error when no phoebus.yaml found")
	}
}
