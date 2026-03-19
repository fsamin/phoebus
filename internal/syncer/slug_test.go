package syncer

import "testing"

func TestGenerateSlug(t *testing.T) {
	tests := []struct {
		title string
		want  string
	}{
		{"Kubernetes Basics", "kubernetes-basics"},
		{"Install & Configure kubectl", "install-configure-kubectl"},
		{"  Hello   World  ", "hello-world"},
		{"Déploiement avec Docker", "d-ploiement-avec-docker"},
		{"Module #1: Introduction!", "module-1-introduction"},
		{"", "untitled"},
		{"   ", "untitled"},
		{"---test---", "test"},
		{"Simple", "simple"},
		// Unicode characters
		{"Introduction à Docker", "introduction-docker"},
		{"日本語テスト", "untitled"},
		{"Hébergement réseau", "h-bergement-r-seau"},
		// Special characters
		{"C++ Basics", "c-basics"},
		{"node.js & npm", "node-js-npm"},
		{"What is CI/CD?", "what-is-ci-cd"},
		{"100% Coverage!", "100-coverage"},
		// Only special characters
		{"@#$%^&*()", "untitled"},
		{"---", "untitled"},
		{"!!!", "untitled"},
		// Very long title
		{"This is a very long title that should still produce a valid slug even though it contains many words and is much longer than most titles would normally be in practice for a learning module", "this-is-a-very-long-title-that-should-still-produce-a-valid-slug-even-though-it-contains-many-words-and-is-much-longer-than-most-titles-would-normally-be-in-practice-for-a-learning-module"},
		// Mixed unicode and ASCII
		{"Étape 1: Installation", "tape-1-installation"},
		// Numbers only
		{"12345", "12345"},
		// Single character
		{"A", "a"},
	}
	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			got := GenerateSlug(tt.title)
			if got != tt.want {
				t.Errorf("GenerateSlug(%q) = %q, want %q", tt.title, got, tt.want)
			}
		})
	}
}
