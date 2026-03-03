package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type GitRepository struct {
	ID           uuid.UUID `json:"id" db:"id"`
	CloneURL     string    `json:"clone_url" db:"clone_url"`
	Branch       string    `json:"branch" db:"branch"`
	AuthType     string    `json:"auth_type" db:"auth_type"`
	Credentials  []byte    `json:"-" db:"credentials"`
	WebhookUUID  uuid.UUID `json:"webhook_uuid" db:"webhook_uuid"`
	SyncStatus   string    `json:"sync_status" db:"sync_status"`
	SyncError    *string   `json:"sync_error,omitempty" db:"sync_error"`
	LastSyncedAt *time.Time `json:"last_synced_at,omitempty" db:"last_synced_at"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`
}

type ProgressStatus string

const (
	ProgressNotStarted ProgressStatus = "not_started"
	ProgressInProgress ProgressStatus = "in_progress"
	ProgressCompleted  ProgressStatus = "completed"
)

type Progress struct {
	ID          uuid.UUID      `json:"id" db:"id"`
	UserID      uuid.UUID      `json:"user_id" db:"user_id"`
	StepID      uuid.UUID      `json:"step_id" db:"step_id"`
	Status      ProgressStatus `json:"status" db:"status"`
	CompletedAt *time.Time     `json:"completed_at,omitempty" db:"completed_at"`
	CreatedAt   time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at" db:"updated_at"`
}

type ExerciseAttempt struct {
	ID        uuid.UUID       `json:"id" db:"id"`
	UserID    uuid.UUID       `json:"user_id" db:"user_id"`
	StepID    uuid.UUID       `json:"step_id" db:"step_id"`
	Answers   json.RawMessage `json:"answers" db:"answers"`
	IsCorrect bool            `json:"is_correct" db:"is_correct"`
	CreatedAt time.Time       `json:"created_at" db:"created_at"`
}

type SyncJobStatus string

const (
	SyncJobPending    SyncJobStatus = "pending"
	SyncJobProcessing SyncJobStatus = "processing"
	SyncJobDone       SyncJobStatus = "done"
	SyncJobFailed     SyncJobStatus = "failed"
)

type SyncJob struct {
	ID          uuid.UUID     `json:"id" db:"id"`
	RepoID      uuid.UUID     `json:"repo_id" db:"repo_id"`
	Status      SyncJobStatus `json:"status" db:"status"`
	Error       *string       `json:"error,omitempty" db:"error"`
	Attempts    int           `json:"attempts" db:"attempts"`
	StartedAt   *time.Time    `json:"started_at,omitempty" db:"started_at"`
	CompletedAt *time.Time    `json:"completed_at,omitempty" db:"completed_at"`
	CreatedAt   time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at" db:"updated_at"`
}
