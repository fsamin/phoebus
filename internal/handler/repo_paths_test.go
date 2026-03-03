package handler

import (
	"encoding/json"
	"io"
	"testing"

	"github.com/google/uuid"
)

func TestListRepoPaths(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()
	cookie := loginAs(t, "admin")

	// Create a repo
	resp := doRequest(t, srv, "POST", "/api/admin/repos", map[string]string{
		"clone_url": "file:///tmp/test-repo-paths",
		"branch":    "main",
		"auth_type": "none",
	}, cookie)
	if resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("create repo: status=%d body=%s", resp.StatusCode, body)
	}
	data := readJSON(t, resp)
	repoID := data["id"].(string)

	// Insert a learning path for this repo
	lpID := uuid.New()
	testDB.Exec(`
		INSERT INTO learning_paths (id, repo_id, title, description, file_path, enabled)
		VALUES ($1, $2, 'Test Path', 'A test path', '', true)
	`, lpID, repoID)

	// List paths
	resp = doRequest(t, srv, "GET", "/api/admin/repos/"+repoID+"/paths", nil, cookie)
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("list paths: status=%d body=%s", resp.StatusCode, body)
	}
	defer resp.Body.Close()
	var paths []map[string]any
	json.NewDecoder(resp.Body).Decode(&paths)
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(paths))
	}
	if paths[0]["title"] != "Test Path" {
		t.Errorf("title = %v, want Test Path", paths[0]["title"])
	}
	if paths[0]["enabled"] != true {
		t.Errorf("enabled = %v, want true", paths[0]["enabled"])
	}

	// Cleanup
	testDB.Exec("DELETE FROM learning_paths WHERE id = $1", lpID)
	testDB.Exec("DELETE FROM git_repositories WHERE id = $1", repoID)
}

func TestToggleRepoPath(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()
	cookie := loginAs(t, "admin")

	// Create a repo
	resp := doRequest(t, srv, "POST", "/api/admin/repos", map[string]string{
		"clone_url": "file:///tmp/test-repo-toggle",
		"branch":    "main",
		"auth_type": "none",
	}, cookie)
	data := readJSON(t, resp)
	repoID := data["id"].(string)

	// Insert a learning path
	lpID := uuid.New()
	testDB.Exec(`
		INSERT INTO learning_paths (id, repo_id, title, description, file_path, enabled)
		VALUES ($1, $2, 'Toggle Path', 'Test', '', true)
	`, lpID, repoID)

	// Disable the path
	resp = doRequest(t, srv, "PATCH", "/api/admin/repos/"+repoID+"/paths/"+lpID.String(), map[string]bool{"enabled": false}, cookie)
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("disable: status=%d body=%s", resp.StatusCode, body)
	}
	result := readJSON(t, resp)
	if result["enabled"] != false {
		t.Errorf("enabled = %v, want false", result["enabled"])
	}

	// Verify it's disabled in the DB
	var enabled bool
	testDB.Get(&enabled, "SELECT enabled FROM learning_paths WHERE id = $1", lpID)
	if enabled {
		t.Error("expected path to be disabled in DB")
	}

	// Re-enable
	resp = doRequest(t, srv, "PATCH", "/api/admin/repos/"+repoID+"/paths/"+lpID.String(), map[string]bool{"enabled": true}, cookie)
	if resp.StatusCode != 200 {
		t.Fatalf("enable: status=%d", resp.StatusCode)
	}
	testDB.Get(&enabled, "SELECT enabled FROM learning_paths WHERE id = $1", lpID)
	if !enabled {
		t.Error("expected path to be enabled in DB")
	}

	// Cleanup
	testDB.Exec("DELETE FROM learning_paths WHERE id = $1", lpID)
	testDB.Exec("DELETE FROM git_repositories WHERE id = $1", repoID)
}

func TestDisabledPathNotInList(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()
	cookie := loginAs(t, "admin")

	// Create a repo
	resp := doRequest(t, srv, "POST", "/api/admin/repos", map[string]string{
		"clone_url": "file:///tmp/test-repo-filter",
		"branch":    "main",
		"auth_type": "none",
	}, cookie)
	data := readJSON(t, resp)
	repoID := data["id"].(string)

	// Insert two learning paths — one enabled, one disabled
	lpEnabled := uuid.New()
	lpDisabled := uuid.New()
	testDB.Exec(`
		INSERT INTO learning_paths (id, repo_id, title, description, file_path, enabled)
		VALUES ($1, $2, 'Enabled Path', 'Visible', 'enabled', true)
	`, lpEnabled, repoID)
	testDB.Exec(`
		INSERT INTO learning_paths (id, repo_id, title, description, file_path, enabled)
		VALUES ($1, $2, 'Disabled Path', 'Hidden', 'disabled', false)
	`, lpDisabled, repoID)

	// List learning paths (public API — should only show enabled)
	learnerCookie := loginAs(t, "learner")
	resp = doRequest(t, srv, "GET", "/api/learning-paths", nil, learnerCookie)
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("list paths: status=%d body=%s", resp.StatusCode, body)
	}
	defer resp.Body.Close()
	var paths []map[string]any
	json.NewDecoder(resp.Body).Decode(&paths)

	for _, p := range paths {
		if p["title"] == "Disabled Path" {
			t.Error("disabled path should not appear in learning paths list")
		}
	}

	// Admin API should show both
	resp = doRequest(t, srv, "GET", "/api/admin/repos/"+repoID+"/paths", nil, cookie)
	defer resp.Body.Close()
	var adminPaths []map[string]any
	json.NewDecoder(resp.Body).Decode(&adminPaths)
	if len(adminPaths) != 2 {
		t.Fatalf("admin should see 2 paths, got %d", len(adminPaths))
	}

	// Cleanup
	testDB.Exec("DELETE FROM learning_paths WHERE id IN ($1, $2)", lpEnabled, lpDisabled)
	testDB.Exec("DELETE FROM git_repositories WHERE id = $1", repoID)
}
