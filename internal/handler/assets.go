package handler

import (
	"errors"
	"io"
	"net/http"
	"regexp"

	"github.com/fsamin/phoebus/internal/assets"
	"github.com/go-chi/chi/v5"
)

var validAssetHash = regexp.MustCompile(`^[a-f0-9]{64}$`)

// ServeAsset serves an asset by its content hash.
// Assets are immutable (hash = content), so we use aggressive caching.
func (h *Handler) ServeAsset(w http.ResponseWriter, r *http.Request) {
	hash := chi.URLParam(r, "hash")
	if !validAssetHash.MatchString(hash) {
		http.Error(w, "invalid asset hash", http.StatusBadRequest)
		return
	}

	reader, contentType, err := h.assetStore.Get(r.Context(), hash)
	if err != nil {
		if errors.Is(err, assets.ErrAssetNotFound) || errors.Is(err, assets.ErrInvalidHash) {
			http.Error(w, "asset not found", http.StatusNotFound)
		} else {
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	io.Copy(w, reader)
}
