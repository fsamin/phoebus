package testutil

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/fsamin/phoebus/internal/database"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// SetupTestDB starts an ephemeral PostgreSQL container and returns a connected *sqlx.DB.
// The container is automatically removed when the test completes.
func SetupTestDB(t *testing.T) *sqlx.DB {
	t.Helper()

	containerName := fmt.Sprintf("phoebus-test-pg-%s", uuid.New().String()[:8])

	// Find a free port
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	// Start PostgreSQL container
	cmd := exec.Command("docker", "run", "-d",
		"--name", containerName,
		"-e", "POSTGRES_USER=test",
		"-e", "POSTGRES_PASSWORD=test",
		"-e", "POSTGRES_DB=phoebus_test",
		"-p", fmt.Sprintf("%d:5432", port),
		"postgres:16-alpine",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to start postgres container: %v\n%s", err, out)
	}
	containerID := strings.TrimSpace(string(out))

	t.Cleanup(func() {
		exec.Command("docker", "rm", "-f", containerID).Run()
	})

	dsn := fmt.Sprintf("postgres://test:test@localhost:%d/phoebus_test?sslmode=disable", port)

	// Wait for PostgreSQL to be ready
	var db *sqlx.DB
	for i := 0; i < 30; i++ {
		time.Sleep(500 * time.Millisecond)
		db, err = sqlx.Connect("postgres", dsn)
		if err == nil {
			if err = db.Ping(); err == nil {
				break
			}
			db.Close()
		}
	}
	if err != nil {
		t.Fatalf("postgres not ready after 15s: %v", err)
	}

	// Run migrations
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	return db
}
