package syncer

import "testing"

func TestRewriteAssetURLs(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		rewrites map[string]string
		expected string
	}{
		{
			name:     "no rewrites",
			content:  "# Hello\n\n![img](https://example.com/img.png)",
			rewrites: nil,
			expected: "# Hello\n\n![img](https://example.com/img.png)",
		},
		{
			name:    "single image rewrite with ./",
			content: "# Hello\n\n![diagram](./assets/diagram.png)\n\nSome text.",
			rewrites: map[string]string{
				"./assets/diagram.png": "/api/assets/abc123",
				"assets/diagram.png":   "/api/assets/abc123",
			},
			expected: "# Hello\n\n![diagram](/api/assets/abc123)\n\nSome text.",
		},
		{
			name:    "multiple assets",
			content: "![a](./assets/a.png)\n![b](./assets/b.mp4)",
			rewrites: map[string]string{
				"./assets/a.png": "/api/assets/hash_a",
				"assets/a.png":   "/api/assets/hash_a",
				"./assets/b.mp4": "/api/assets/hash_b",
				"assets/b.mp4":   "/api/assets/hash_b",
			},
			expected: "![a](/api/assets/hash_a)\n![b](/api/assets/hash_b)",
		},
		{
			name:    "does not rewrite absolute URLs",
			content: "![img](https://cdn.example.com/assets/img.png)",
			rewrites: map[string]string{
				"./assets/img.png": "/api/assets/hash_img",
				"assets/img.png":   "/api/assets/hash_img",
			},
			expected: "![img](https://cdn.example.com/assets/img.png)",
		},
		{
			name:    "rewrite without dot prefix",
			content: "![img](assets/photo.jpg)",
			rewrites: map[string]string{
				"./assets/photo.jpg": "/api/assets/hash_photo",
				"assets/photo.jpg":   "/api/assets/hash_photo",
			},
			expected: "![img](/api/assets/hash_photo)",
		},
		{
			name:     "empty content",
			content:  "",
			rewrites: map[string]string{"./assets/x.png": "/api/assets/hash_x"},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rewriteAssetURLs(tt.content, tt.rewrites)
			if got != tt.expected {
				t.Errorf("rewriteAssetURLs():\ngot:  %q\nwant: %q", got, tt.expected)
			}
		})
	}
}
