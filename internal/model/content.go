package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type LearningPath struct {
	ID                uuid.UUID      `json:"id" db:"id"`
	RepoID            uuid.UUID      `json:"repo_id" db:"repo_id"`
	Title             string         `json:"title" db:"title"`
	Description       string         `json:"description" db:"description"`
	Icon              *string        `json:"icon,omitempty" db:"icon"`
	Tags              pq.StringArray `json:"tags" db:"tags"`
	EstimatedDuration *string        `json:"estimated_duration,omitempty" db:"estimated_duration"`
	Prerequisites     pq.StringArray `json:"prerequisites,omitempty" db:"prerequisites"`
	Enabled           bool           `json:"enabled" db:"enabled"`
	CreatedAt         time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at" db:"updated_at"`
}

type Module struct {
	ID             uuid.UUID      `json:"id" db:"id"`
	LearningPathID uuid.UUID      `json:"learning_path_id" db:"learning_path_id"`
	Title          string         `json:"title" db:"title"`
	Description    string         `json:"description" db:"description"`
	Competencies   pq.StringArray `json:"competencies" db:"competencies"`
	Position       int            `json:"position" db:"position"`
	FilePath       string         `json:"-" db:"file_path"`
	CreatedAt      time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at" db:"updated_at"`
}

type StepType string

const (
	StepTypeLesson           StepType = "lesson"
	StepTypeQuiz             StepType = "quiz"
	StepTypeTerminalExercise StepType = "terminal-exercise"
	StepTypeCodeExercise     StepType = "code-exercise"
)

type Step struct {
	ID           uuid.UUID       `json:"id" db:"id"`
	ModuleID     uuid.UUID       `json:"module_id" db:"module_id"`
	Title        string          `json:"title" db:"title"`
	Type         StepType        `json:"type" db:"type"`
	Duration     *string         `json:"estimated_duration,omitempty" db:"estimated_duration"`
	ContentMD    string          `json:"-" db:"content_md"`
	ExerciseData json.RawMessage `json:"-" db:"exercise_data"`
	Position     int             `json:"position" db:"position"`
	FilePath     string          `json:"-" db:"file_path"`
	DeletedAt    *time.Time      `json:"-" db:"deleted_at"`
	CreatedAt    time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at" db:"updated_at"`
}

type CodebaseFile struct {
	ID       uuid.UUID `json:"id" db:"id"`
	StepID   uuid.UUID `json:"step_id" db:"step_id"`
	FilePath string    `json:"file_path" db:"file_path"`
	Content  string    `json:"content" db:"content"`
	Language string    `json:"language" db:"language"`
	Position int       `json:"position" db:"position"`
}
