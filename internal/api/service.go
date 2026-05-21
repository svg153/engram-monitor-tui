package api

import (
	"context"

	"github.com/svg153/engram-monitor-tui/internal/model"
)

type Service interface {
	Health(ctx context.Context) (model.Health, error)
	Stats(ctx context.Context) (model.Stats, error)
	Search(ctx context.Context, params model.SearchParams) ([]model.Observation, error)
	AllObservations(ctx context.Context) ([]model.Observation, error)
	RecentSessions(ctx context.Context, limit int) ([]model.SessionSummary, error)
	RecentPrompts(ctx context.Context, limit int) ([]model.Prompt, error)
	Timeline(ctx context.Context, observationID int64, before, after int) (model.TimelineResult, error)
	UpdateObservation(ctx context.Context, id int64, payload model.ObservationUpdate) (model.Observation, error)
	DeletePrompt(ctx context.Context, id int64) error
	DeleteSession(ctx context.Context, id string) error
	Export(ctx context.Context, project string) ([]byte, error)
	Import(ctx context.Context, data []byte) error
	ImportFile(ctx context.Context, path string) (model.ExportData, error)
	MergeProjects(ctx context.Context, from, to string) error
}
