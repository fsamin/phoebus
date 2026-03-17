package syncer

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

var (
	nonAlphanumRe  = regexp.MustCompile(`[^a-z0-9]+`)
	multiHyphenRe  = regexp.MustCompile(`-{2,}`)
)

// GenerateSlug creates a URL-friendly slug from a title.
// It lowercases the string, replaces non-alphanumeric characters with hyphens,
// collapses multiple hyphens, and trims leading/trailing hyphens.
func GenerateSlug(title string) string {
	s := strings.ToLower(strings.TrimSpace(title))
	s = nonAlphanumRe.ReplaceAllString(s, "-")
	s = multiHyphenRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "untitled"
	}
	return s
}

// ensureUniqueSlug checks for slug uniqueness in a table at global scope
// (e.g. learning_paths). If a conflict exists with a different row (identified
// by excludeCondition), appends a numeric suffix (-2, -3, ...).
func ensureUniqueSlug(ctx context.Context, tx *sqlx.Tx, table, column, slug string, excludeCondition string, excludeArgs ...any) (string, error) {
	candidate := slug
	for i := 2; ; i++ {
		// Check if this slug is already taken by a different row
		query := fmt.Sprintf(
			`SELECT COUNT(*) FROM %s WHERE %s = $1 AND deleted_at IS NULL AND NOT (%s)`,
			table, column,
			fmt.Sprintf(excludeCondition, 2, 3),
		)
		args := append([]any{candidate}, excludeArgs...)
		var count int
		if err := tx.GetContext(ctx, &count, query, args...); err != nil {
			return "", err
		}
		if count == 0 {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s-%d", slug, i)
	}
}

// ensureUniqueScopedSlug checks for slug uniqueness within a scope (e.g.
// modules within a learning_path, steps within a module). scopeCol/scopeVal
// define the parent scope. excludeCol/excludeVal identify the current row
// to exclude from conflict check.
func ensureUniqueScopedSlug(ctx context.Context, tx *sqlx.Tx, table, column, slug, scopeCol string, scopeVal uuid.UUID, excludeCol string, excludeVal string) (string, error) {
	candidate := slug
	// Steps have deleted_at, modules don't — check table name
	hasDeletedAt := table == "steps" || table == "learning_paths"
	for i := 2; ; i++ {
		var query string
		if hasDeletedAt {
			query = fmt.Sprintf(
				`SELECT COUNT(*) FROM %s WHERE %s = $1 AND %s = $2 AND %s != $3 AND deleted_at IS NULL`,
				table, column, scopeCol, excludeCol,
			)
		} else {
			query = fmt.Sprintf(
				`SELECT COUNT(*) FROM %s WHERE %s = $1 AND %s = $2 AND %s != $3`,
				table, column, scopeCol, excludeCol,
			)
		}
		var count int
		if err := tx.GetContext(ctx, &count, query, candidate, scopeVal, excludeVal); err != nil {
			return "", err
		}
		if count == 0 {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s-%d", slug, i)
	}
}
