package store

import "harness/internal/model"

type Store interface {
	ListPrompts() ([]model.Prompt, error)
	ListRuns() ([]model.Run, error)
	GetPrompt(recordID string) (*model.Prompt, error)
	GetRun(recordID string) (*model.Run, error)
	UpsertPrompt(p *model.Prompt) (string, error)
	UpsertRun(r *model.Run) (string, error)
	UpdateRun(recordID string, fields map[string]any) error
	UpdatePrompt(recordID string, fields map[string]any) error
	FindRunsByPrompt(promptRecordID string) ([]model.Run, error)
}
