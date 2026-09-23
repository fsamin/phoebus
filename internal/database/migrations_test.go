package database

import (
	"strings"
	"testing"
)

// knownDuplicateVersions records version numbers that were already shipped twice
// before this check existed. They cannot be renamed: deployed instances have
// recorded both file names in schema_migrations, and renaming one would replay it.
var knownDuplicateVersions = map[string]bool{"014": true}

// TestNoDownMigrations keeps the migrations directory honest. Migrate() only
// ever applies *.up.sql and the application exposes no rollback path, so a
// .down.sql would never run — shipping one would suggest a rollback that does
// not exist. Adding real rollback support means teaching Migrate() to run them,
// and this test should go in the same change.
func TestNoDownMigrations(t *testing.T) {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".down.sql") {
			t.Errorf("%s would never be applied: Migrate() only reads *.up.sql", e.Name())
		}
	}
}

// TestMigrationVersionsAreUnique guards the ordering of migrations. Migrate()
// applies *.up.sql in lexicographic order, so two migrations sharing a version
// number have an arbitrary relative order — which breaks as soon as one depends
// on the other. This test fails on any new collision.
func TestMigrationVersionsAreUnique(t *testing.T) {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}

	byVersion := map[string][]string{}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		version, _, found := strings.Cut(name, "_")
		if !found || len(version) != 3 {
			t.Errorf("migration %s: expected a NNN_name.up.sql file name", name)
			continue
		}
		for _, c := range version {
			if c < '0' || c > '9' {
				t.Errorf("migration %s: version prefix %q is not numeric", name, version)
				break
			}
		}
		byVersion[version] = append(byVersion[version], name)
	}

	if len(byVersion) == 0 {
		t.Fatal("no migrations found")
	}

	for version, files := range byVersion {
		if len(files) > 1 && !knownDuplicateVersions[version] {
			t.Errorf("version %s is used by %d migrations (%s); pick the next free number",
				version, len(files), strings.Join(files, ", "))
		}
	}
}
