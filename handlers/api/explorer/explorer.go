package explorer

import (
	"encoding/json"
	"errors"
	"excalidraw-complete/core"
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/sirupsen/logrus"
)

type (
	projectRequest struct {
		Name string `json:"name"`
	}

	fileCreateRequest struct {
		Name string           `json:"name"`
		Data *json.RawMessage `json:"data"`
	}

	fileUpdateRequest struct {
		Name      *string          `json:"name"`
		ProjectID *string          `json:"projectId"`
		Data      *json.RawMessage `json:"data"`
	}

	projectsResponse struct {
		Projects []core.Project `json:"projects"`
	}

	filesResponse struct {
		Files []core.FileMeta `json:"files"`
	}

	fileResponse struct {
		Meta core.FileMeta   `json:"meta"`
		Data json.RawMessage `json:"data"`
	}

	errorResponse struct {
		Error string `json:"error"`
	}
)

func Routes(store core.ExplorerStore) func(chi.Router) {
	return func(r chi.Router) {
		r.Get("/projects", handleListProjects(store))
		r.Post("/projects", handleCreateProject(store))
		r.Patch("/projects/{projectId}", handleUpdateProject(store))
		r.Delete("/projects/{projectId}", handleDeleteProject(store))

		r.Get("/projects/{projectId}/files", handleListFiles(store))
		r.Post("/projects/{projectId}/files", handleCreateFile(store))

		r.Get("/files/{fileId}", handleGetFile(store))
		r.Put("/files/{fileId}", handleUpdateFile(store))
		r.Delete("/files/{fileId}", handleDeleteFile(store))
	}
}

func handleListProjects(store core.ExplorerStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projects, err := store.ListProjects(r.Context())
		if err != nil {
			writeError(w, r, err)
			return
		}
		render.JSON(w, r, projectsResponse{Projects: projects})
	}
}

func handleCreateProject(store core.ExplorerStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req projectRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		project, err := store.CreateProject(r.Context(), req.Name)
		if err != nil {
			writeError(w, r, err)
			return
		}
		render.Status(r, http.StatusCreated)
		render.JSON(w, r, project)
	}
}

func handleUpdateProject(store core.ExplorerStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req projectRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		project, err := store.UpdateProject(r.Context(), chi.URLParam(r, "projectId"), req.Name)
		if err != nil {
			writeError(w, r, err)
			return
		}
		render.JSON(w, r, project)
	}
}

func handleDeleteProject(store core.ExplorerStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := store.DeleteProject(r.Context(), chi.URLParam(r, "projectId")); err != nil {
			writeError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleListFiles(store core.ExplorerStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		files, err := store.ListFiles(r.Context(), chi.URLParam(r, "projectId"))
		if err != nil {
			writeError(w, r, err)
			return
		}
		render.JSON(w, r, filesResponse{Files: files})
	}
}

func handleCreateFile(store core.ExplorerStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req fileCreateRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		var data json.RawMessage
		if req.Data != nil {
			data = *req.Data
		}
		meta, err := store.CreateFile(r.Context(), chi.URLParam(r, "projectId"), req.Name, data)
		if err != nil {
			writeError(w, r, err)
			return
		}
		render.Status(r, http.StatusCreated)
		render.JSON(w, r, meta)
	}
}

func handleGetFile(store core.ExplorerStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		meta, data, err := store.GetFile(r.Context(), chi.URLParam(r, "fileId"))
		if err != nil {
			writeError(w, r, err)
			return
		}
		render.JSON(w, r, fileResponse{Meta: *meta, Data: data})
	}
}

func handleUpdateFile(store core.ExplorerStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req fileUpdateRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		meta, err := store.UpdateFile(r.Context(), chi.URLParam(r, "fileId"), core.FileUpdate{
			Name:      req.Name,
			ProjectID: req.ProjectID,
			Data:      req.Data,
		})
		if err != nil {
			writeError(w, r, err)
			return
		}
		render.JSON(w, r, meta)
	}
}

func handleDeleteFile(store core.ExplorerStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := store.DeleteFile(r.Context(), chi.URLParam(r, "fileId")); err != nil {
			writeError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, value any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(value); err != nil {
		if !errors.Is(err, io.EOF) {
			writeError(w, r, invalidInput("invalid JSON body"))
			return false
		}
		writeError(w, r, invalidInput("request body is required"))
		return false
	}
	return true
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	message := "internal server error"

	switch {
	case errors.Is(err, core.ErrInvalidInput):
		status = http.StatusBadRequest
		message = err.Error()
	case errors.Is(err, core.ErrNotFound):
		status = http.StatusNotFound
		message = err.Error()
	default:
		logrus.WithField("error", err).Error("Explorer API request failed")
	}

	render.Status(r, status)
	render.JSON(w, r, errorResponse{Error: message})
}

func invalidInput(message string) error {
	return fmt.Errorf("%w: %s", core.ErrInvalidInput, message)
}
