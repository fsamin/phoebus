package handler

import "net/http"

func (h *Handler) SSHPublicKey(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"public_key": h.sshPublicKey})
}
