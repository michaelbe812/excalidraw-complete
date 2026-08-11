package explorer

import (
	"encoding/json"
	"excalidraw-complete/core"
	explorerstore "excalidraw-complete/stores/explorer"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestExplorerHandlersHappyPaths(t *testing.T) {
	router := newTestRouter(t)

	res := doJSON(t, router, http.MethodGet, "/api/explorer/projects", "")
	assertStatus(t, res, http.StatusOK)
	var projectList struct {
		Projects []core.Project `json:"projects"`
	}
	decodeResponse(t, res, &projectList)
	if len(projectList.Projects) != 0 {
		t.Fatalf("initial projects = %+v, want empty", projectList.Projects)
	}

	res = doJSON(t, router, http.MethodPost, "/api/explorer/projects", `{"name":"Project A"}`)
	assertStatus(t, res, http.StatusCreated)
	var projectA core.Project
	decodeResponse(t, res, &projectA)

	res = doJSON(t, router, http.MethodPost, "/api/explorer/projects", `{"name":"Project B"}`)
	assertStatus(t, res, http.StatusCreated)
	var projectB core.Project
	decodeResponse(t, res, &projectB)

	res = doJSON(t, router, http.MethodPatch, "/api/explorer/projects/"+projectA.ID, `{"name":"Renamed Project"}`)
	assertStatus(t, res, http.StatusOK)
	decodeResponse(t, res, &projectA)
	if projectA.Name != "Renamed Project" {
		t.Fatalf("patched project name = %q, want Renamed Project", projectA.Name)
	}

	res = doJSON(t, router, http.MethodGet, "/api/explorer/projects", "")
	assertStatus(t, res, http.StatusOK)
	decodeResponse(t, res, &projectList)
	if len(projectList.Projects) != 2 {
		t.Fatalf("projects count = %d, want 2", len(projectList.Projects))
	}

	res = doJSON(t, router, http.MethodGet, "/api/explorer/projects/"+projectA.ID+"/files", "")
	assertStatus(t, res, http.StatusOK)
	var fileList struct {
		Files []core.FileMeta `json:"files"`
	}
	decodeResponse(t, res, &fileList)
	if len(fileList.Files) != 0 {
		t.Fatalf("initial files = %+v, want empty", fileList.Files)
	}

	res = doJSON(t, router, http.MethodPost, "/api/explorer/projects/"+projectA.ID+"/files", `{"name":"Drawing"}`)
	assertStatus(t, res, http.StatusCreated)
	var file core.FileMeta
	decodeResponse(t, res, &file)
	if file.ProjectID != projectA.ID || file.Name != "Drawing" {
		t.Fatalf("created file = %+v, want project %s and name Drawing", file, projectA.ID)
	}

	res = doJSON(t, router, http.MethodGet, "/api/explorer/files/"+file.ID, "")
	assertStatus(t, res, http.StatusOK)
	var loaded struct {
		Meta core.FileMeta   `json:"meta"`
		Data json.RawMessage `json:"data"`
	}
	decodeResponse(t, res, &loaded)
	if loaded.Meta.ID != file.ID || !strings.Contains(string(loaded.Data), `"type":"excalidraw"`) {
		t.Fatalf("loaded file = %+v data=%s, want default Excalidraw scene", loaded.Meta, loaded.Data)
	}

	res = doJSON(t, router, http.MethodPut, "/api/explorer/files/"+file.ID, `{"name":"Saved Drawing","projectId":"`+projectB.ID+`","data":{"saved":true}}`)
	assertStatus(t, res, http.StatusOK)
	decodeResponse(t, res, &file)
	if file.Name != "Saved Drawing" || file.ProjectID != projectB.ID {
		t.Fatalf("updated file = %+v, want moved and renamed", file)
	}

	res = doJSON(t, router, http.MethodGet, "/api/explorer/projects/"+projectB.ID+"/files", "")
	assertStatus(t, res, http.StatusOK)
	decodeResponse(t, res, &fileList)
	if len(fileList.Files) != 1 || fileList.Files[0].ID != file.ID {
		t.Fatalf("target project files = %+v, want moved file %s", fileList.Files, file.ID)
	}

	res = doJSON(t, router, http.MethodDelete, "/api/explorer/files/"+file.ID, "")
	assertStatus(t, res, http.StatusNoContent)

	res = doJSON(t, router, http.MethodDelete, "/api/explorer/projects/"+projectA.ID, "")
	assertStatus(t, res, http.StatusNoContent)

	res = doJSON(t, router, http.MethodDelete, "/api/explorer/projects/"+projectB.ID, "")
	assertStatus(t, res, http.StatusNoContent)

	res = doJSON(t, router, http.MethodGet, "/api/explorer/projects", "")
	assertStatus(t, res, http.StatusOK)
	decodeResponse(t, res, &projectList)
	if len(projectList.Projects) != 0 {
		t.Fatalf("projects after delete = %+v, want empty", projectList.Projects)
	}
}

func TestExplorerHandlersErrors(t *testing.T) {
	router := newTestRouter(t)

	res := doJSON(t, router, http.MethodPost, "/api/explorer/projects", `{"name":"   "}`)
	assertJSONError(t, res, http.StatusBadRequest)

	res = doJSON(t, router, http.MethodGet, "/api/explorer/files/not-a-ulid", "")
	assertJSONError(t, res, http.StatusBadRequest)

	missingULID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	res = doJSON(t, router, http.MethodGet, "/api/explorer/projects/"+missingULID+"/files", "")
	assertJSONError(t, res, http.StatusNotFound)

	res = doJSON(t, router, http.MethodPost, "/api/explorer/projects/"+missingULID+"/files", `{"name":"Drawing"}`)
	assertJSONError(t, res, http.StatusNotFound)

	res = doJSON(t, router, http.MethodPost, "/api/explorer/projects", `{`)
	assertJSONError(t, res, http.StatusBadRequest)
}

func newTestRouter(t *testing.T) http.Handler {
	t.Helper()
	router := chi.NewRouter()
	router.Route("/api/explorer", Routes(explorerstore.NewFilesystemStore(t.TempDir())))
	return router
}

func doJSON(t *testing.T, handler http.Handler, method string, path string, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	return res
}

func assertStatus(t *testing.T, res *httptest.ResponseRecorder, status int) {
	t.Helper()
	if res.Code != status {
		t.Fatalf("status = %d body=%s, want %d", res.Code, res.Body.String(), status)
	}
}

func assertJSONError(t *testing.T, res *httptest.ResponseRecorder, status int) {
	t.Helper()
	assertStatus(t, res, status)
	var body struct {
		Error string `json:"error"`
	}
	decodeResponse(t, res, &body)
	if body.Error == "" {
		t.Fatalf("error response body = %s, want non-empty error", res.Body.String())
	}
}

func decodeResponse(t *testing.T, res *httptest.ResponseRecorder, value any) {
	t.Helper()
	if err := json.Unmarshal(res.Body.Bytes(), value); err != nil {
		t.Fatalf("decode response %s: %v", res.Body.String(), err)
	}
}
