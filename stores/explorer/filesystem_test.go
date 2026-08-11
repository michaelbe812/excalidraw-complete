package explorer

import (
	"context"
	"encoding/json"
	"errors"
	"excalidraw-complete/core"
	"os"
	"path/filepath"
	"testing"
)

func TestFilesystemStoreProjectAndFileCRUD(t *testing.T) {
	ctx := context.Background()
	store := NewFilesystemStore(t.TempDir())

	zulu, err := store.CreateProject(ctx, "Zulu")
	if err != nil {
		t.Fatalf("CreateProject Zulu: %v", err)
	}
	alpha, err := store.CreateProject(ctx, "Alpha")
	if err != nil {
		t.Fatalf("CreateProject Alpha: %v", err)
	}

	projects, err := store.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if got, want := []string{projects[0].Name, projects[1].Name}, []string{"Alpha", "Zulu"}; got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("projects sorted by name = %v, want %v", got, want)
	}

	renamed, err := store.UpdateProject(ctx, zulu.ID, "Beta")
	if err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}
	if renamed.Name != "Beta" {
		t.Fatalf("renamed project name = %q, want Beta", renamed.Name)
	}

	data := json.RawMessage(`{"elements":[{"id":"one"}],"appState":{"viewBackgroundColor":"#fff"}}`)
	meta, err := store.CreateFile(ctx, alpha.ID, "Drawing", data)
	if err != nil {
		t.Fatalf("CreateFile: %v", err)
	}

	files, err := store.ListFiles(ctx, alpha.ID)
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	if len(files) != 1 || files[0].ID != meta.ID {
		t.Fatalf("ListFiles returned %+v, want file %s", files, meta.ID)
	}

	loadedMeta, loadedData, err := store.GetFile(ctx, meta.ID)
	if err != nil {
		t.Fatalf("GetFile: %v", err)
	}
	if loadedMeta.Name != "Drawing" || string(loadedData) != string(data) {
		t.Fatalf("GetFile = (%+v, %s), want name Drawing and data %s", loadedMeta, loadedData, data)
	}

	newData := json.RawMessage(`{"saved":true}`)
	newName := "Updated drawing"
	updated, err := store.UpdateFile(ctx, meta.ID, core.FileUpdate{Name: &newName, Data: &newData})
	if err != nil {
		t.Fatalf("UpdateFile: %v", err)
	}
	if updated.Name != newName {
		t.Fatalf("updated file name = %q, want %q", updated.Name, newName)
	}
	_, loadedData, err = store.GetFile(ctx, meta.ID)
	if err != nil {
		t.Fatalf("GetFile after update: %v", err)
	}
	if string(loadedData) != string(newData) {
		t.Fatalf("updated data = %s, want %s", loadedData, newData)
	}

	if err := store.DeleteFile(ctx, meta.ID); err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}
	if _, _, err := store.GetFile(ctx, meta.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("GetFile deleted err = %v, want ErrNotFound", err)
	}
}

func TestFilesystemStoreMoveFile(t *testing.T) {
	ctx := context.Background()
	store := NewFilesystemStore(t.TempDir())

	source, err := store.CreateProject(ctx, "Source")
	if err != nil {
		t.Fatalf("CreateProject source: %v", err)
	}
	target, err := store.CreateProject(ctx, "Target")
	if err != nil {
		t.Fatalf("CreateProject target: %v", err)
	}
	meta, err := store.CreateFile(ctx, source.ID, "Moved", nil)
	if err != nil {
		t.Fatalf("CreateFile: %v", err)
	}

	updated, err := store.UpdateFile(ctx, meta.ID, core.FileUpdate{ProjectID: &target.ID})
	if err != nil {
		t.Fatalf("UpdateFile move: %v", err)
	}
	if updated.ProjectID != target.ID {
		t.Fatalf("updated projectId = %q, want %q", updated.ProjectID, target.ID)
	}

	sourceFiles, err := store.ListFiles(ctx, source.ID)
	if err != nil {
		t.Fatalf("ListFiles source: %v", err)
	}
	targetFiles, err := store.ListFiles(ctx, target.ID)
	if err != nil {
		t.Fatalf("ListFiles target: %v", err)
	}
	if len(sourceFiles) != 0 || len(targetFiles) != 1 || targetFiles[0].ID != meta.ID {
		t.Fatalf("after move source=%+v target=%+v, want only target to contain %s", sourceFiles, targetFiles, meta.ID)
	}
}

func TestFilesystemStoreDeleteProjectCascadesFiles(t *testing.T) {
	ctx := context.Background()
	basePath := t.TempDir()
	store := NewFilesystemStore(basePath)

	project, err := store.CreateProject(ctx, "Project")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	meta, err := store.CreateFile(ctx, project.ID, "Drawing", nil)
	if err != nil {
		t.Fatalf("CreateFile: %v", err)
	}

	if err := store.DeleteProject(ctx, project.ID); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	if _, err := store.ListFiles(ctx, project.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("ListFiles deleted project err = %v, want ErrNotFound", err)
	}
	if _, _, err := store.GetFile(ctx, meta.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("GetFile cascaded err = %v, want ErrNotFound", err)
	}

	for _, path := range []string{
		filepath.Join(basePath, "files", meta.ID+".meta.json"),
		filepath.Join(basePath, "files", meta.ID+".excalidraw"),
	} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("cascaded path %s stat err = %v, want not exist", path, err)
		}
	}
}

func TestFilesystemStoreRejectsInvalidIDs(t *testing.T) {
	ctx := context.Background()
	store := NewFilesystemStore(t.TempDir())

	if _, err := store.UpdateProject(ctx, "../bad", "Name"); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("UpdateProject invalid ID err = %v, want ErrInvalidInput", err)
	}
	if err := store.DeleteProject(ctx, "../../bad"); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("DeleteProject invalid ID err = %v, want ErrInvalidInput", err)
	}
	if _, err := store.ListFiles(ctx, "not-a-ulid"); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("ListFiles invalid ID err = %v, want ErrInvalidInput", err)
	}
	if _, err := store.CreateFile(ctx, "01ARZ3NDEKTSV4RRFFQ69G5FAV/../x", "Name", nil); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("CreateFile invalid ID err = %v, want ErrInvalidInput", err)
	}
	if _, _, err := store.GetFile(ctx, "../bad"); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("GetFile invalid ID err = %v, want ErrInvalidInput", err)
	}
	if err := store.DeleteFile(ctx, "../bad"); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("DeleteFile invalid ID err = %v, want ErrInvalidInput", err)
	}
}

func TestFilesystemStoreRejectsInvalidNames(t *testing.T) {
	ctx := context.Background()
	store := NewFilesystemStore(t.TempDir())

	if _, err := store.CreateProject(ctx, "   "); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("CreateProject blank name err = %v, want ErrInvalidInput", err)
	}
	project, err := store.CreateProject(ctx, "Project")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if _, err := store.CreateFile(ctx, project.ID, "", nil); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("CreateFile blank name err = %v, want ErrInvalidInput", err)
	}
}
