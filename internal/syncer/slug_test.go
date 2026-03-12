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
