package handler

import (
	"encoding/json"
	"net/http"

	"github.com/fsamin/phoebus/internal/model"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

type dependencyEdge struct {
	Source       string   `json:"source"`
	Target       string   `json:"target"`
	Type         string   `json:"type"`
	Competencies []string `json:"competencies,omitempty"`
}

type dependenciesResponse struct {
	Edges []dependencyEdge `json:"edges"`
}

// ListPathDependencies returns all edges between learning paths:
// - "auto" edges derived from prerequisite/competency matching
// - "manual" and "yaml" edges from the path_dependencies table
func (h *Handler) ListPathDependencies(w http.ResponseWriter, r *http.Request) {
	// Fetch all enabled paths with their prerequisites
	type pathRow struct {
		ID            uuid.UUID      `db:"id"`
		Prerequisites pq.StringArray `db:"prerequisites"`
	}
	var paths []pathRow
	if err := h.db.SelectContext(r.Context(), &paths,
		`SELECT id, prerequisites FROM learning_paths WHERE enabled = true`); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Fetch competencies provided by each path (aggregated from modules)
	type compRow struct {
		LearningPathID uuid.UUID `db:"learning_path_id"`
		Competency     string    `db:"competency"`
	}
	var comps []compRow
	if err := h.db.SelectContext(r.Context(), &comps,
		`SELECT m.learning_path_id, unnest(m.competencies) AS competency
		 FROM modules m
		 JOIN learning_paths lp ON lp.id = m.learning_path_id
		 WHERE lp.enabled = true`); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Build: competency -> list of path IDs that provide it
	compProviders := make(map[string][]string)
	for _, c := range comps {
		compProviders[c.Competency] = append(compProviders[c.Competency], c.LearningPathID.String())
	}

	// Build auto edges: for each path's prerequisite, find which paths provide it
	edgeSet := make(map[string]dependencyEdge)
	for _, p := range paths {
		for _, prereq := range p.Prerequisites {
			providers := compProviders[prereq]
			for _, providerID := range providers {
				if providerID == p.ID.String() {
					continue // skip self-dependency
				}
				key := providerID + "->" + p.ID.String()
				if existing, ok := edgeSet[key]; ok {
					existing.Competencies = append(existing.Competencies, prereq)
					edgeSet[key] = existing
				} else {
					edgeSet[key] = dependencyEdge{
						Source:       providerID,
						Target:       p.ID.String(),
						Type:         "auto",
						Competencies: []string{prereq},
					}
				}
			}
		}
	}

	// Fetch manual/yaml edges from path_dependencies table
	var manualDeps []model.PathDependency
	if err := h.db.SelectContext(r.Context(), &manualDeps,
		`SELECT pd.id, pd.source_path_id, pd.target_path_id, pd.dep_type, pd.created_at
		 FROM path_dependencies pd
		 JOIN learning_paths s ON s.id = pd.source_path_id AND s.enabled = true
		 JOIN learning_paths t ON t.id = pd.target_path_id AND t.enabled = true`); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for _, dep := range manualDeps {
		key := dep.SourcePathID.String() + "->" + dep.TargetPathID.String()
		if _, ok := edgeSet[key]; !ok {
			edgeSet[key] = dependencyEdge{
				Source: dep.SourcePathID.String(),
				Target: dep.TargetPathID.String(),
				Type:   dep.DepType,
			}
		}
	}

	edges := make([]dependencyEdge, 0, len(edgeSet))
	for _, e := range edgeSet {
		edges = append(edges, e)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dependenciesResponse{Edges: edges})
}

type createDependencyRequest struct {
	SourcePathID string `json:"source_path_id"`
	TargetPathID string `json:"target_path_id"`
}

// CreatePathDependency adds a manual dependency between two paths (admin only)
func (h *Handler) CreatePathDependency(w http.ResponseWriter, r *http.Request) {
	var req createDependencyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	sourceID, err := uuid.Parse(req.SourcePathID)
	if err != nil {
		http.Error(w, "invalid source_path_id", http.StatusBadRequest)
		return
	}
	targetID, err := uuid.Parse(req.TargetPathID)
	if err != nil {
		http.Error(w, "invalid target_path_id", http.StatusBadRequest)
		return
	}
	if sourceID == targetID {
		http.Error(w, "source and target must be different", http.StatusBadRequest)
		return
	}

	var dep model.PathDependency
	err = h.db.QueryRowxContext(r.Context(),
		`INSERT INTO path_dependencies (source_path_id, target_path_id, dep_type)
		 VALUES ($1, $2, 'manual')
		 ON CONFLICT (source_path_id, target_path_id) DO UPDATE SET dep_type = 'manual'
		 RETURNING id, source_path_id, target_path_id, dep_type, created_at`,
		sourceID, targetID).StructScan(&dep)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(dep)
}

// DeletePathDependency removes a manual dependency (admin only)
func (h *Handler) DeletePathDependency(w http.ResponseWriter, r *http.Request) {
	depID := chi.URLParam(r, "depId")
	id, err := uuid.Parse(depID)
	if err != nil {
		http.Error(w, "invalid dependency id", http.StatusBadRequest)
		return
	}

	result, err := h.db.ExecContext(r.Context(),
		`DELETE FROM path_dependencies WHERE id = $1 AND dep_type = 'manual'`, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		http.Error(w, "dependency not found or not manual", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListManualDependencies returns only manual dependencies for the admin UI
func (h *Handler) ListManualDependencies(w http.ResponseWriter, r *http.Request) {
	type depWithNames struct {
		model.PathDependency
		SourceTitle string `json:"source_title" db:"source_title"`
		TargetTitle string `json:"target_title" db:"target_title"`
	}

	var deps []depWithNames
	if err := h.db.SelectContext(r.Context(), &deps,
		`SELECT pd.id, pd.source_path_id, pd.target_path_id, pd.dep_type, pd.created_at,
		        s.title AS source_title, t.title AS target_title
		 FROM path_dependencies pd
		 JOIN learning_paths s ON s.id = pd.source_path_id
		 JOIN learning_paths t ON t.id = pd.target_path_id
		 ORDER BY pd.created_at DESC`); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if deps == nil {
		deps = []depWithNames{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(deps)
}
