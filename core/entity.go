package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"time"
)

type (
	Document struct {
		Data bytes.Buffer
	}

	DocumentStore interface {
		FindID(ctx context.Context, id string) (*Document, error)
		Create(ctx context.Context, document *Document) (string, error)
	}

	Project struct {
		ID        string    `json:"id"`
		Name      string    `json:"name"`
		CreatedAt time.Time `json:"createdAt"`
		UpdatedAt time.Time `json:"updatedAt"`
	}

	FileMeta struct {
		ID        string    `json:"id"`
		ProjectID string    `json:"projectId"`
		Name      string    `json:"name"`
		CreatedAt time.Time `json:"createdAt"`
		UpdatedAt time.Time `json:"updatedAt"`
	}

	FileUpdate struct {
		Name      *string
		ProjectID *string
		Data      *json.RawMessage
	}

	ExplorerStore interface {
		ListProjects(ctx context.Context) ([]Project, error)
		CreateProject(ctx context.Context, name string) (*Project, error)
		UpdateProject(ctx context.Context, id string, name string) (*Project, error)
		DeleteProject(ctx context.Context, id string) error

		ListFiles(ctx context.Context, projectID string) ([]FileMeta, error)
		CreateFile(ctx context.Context, projectID string, name string, data json.RawMessage) (*FileMeta, error)
		GetFile(ctx context.Context, fileID string) (*FileMeta, json.RawMessage, error)
		UpdateFile(ctx context.Context, fileID string, update FileUpdate) (*FileMeta, error)
		DeleteFile(ctx context.Context, fileID string) error
	}
)

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrNotFound     = errors.New("not found")
)
