package explorer

import (
	"context"
	"encoding/json"
	"errors"
	"excalidraw-complete/core"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/oklog/ulid/v2"
	"github.com/sirupsen/logrus"
)

const (
	defaultSceneJSON = `{"type":"excalidraw","version":2,"elements":[],"appState":{},"files":{}}`
)

var ulidPattern = regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}$`)

type FilesystemStore struct {
	basePath string
}

func NewFilesystemStore(basePath string) core.ExplorerStore {
	if err := os.MkdirAll(filepath.Join(basePath, "projects"), 0755); err != nil {
		logrus.WithField("error", err).Fatal("failed to create explorer projects directory")
	}
	if err := os.MkdirAll(filepath.Join(basePath, "files"), 0755); err != nil {
		logrus.WithField("error", err).Fatal("failed to create explorer files directory")
	}

	return &FilesystemStore{basePath: basePath}
}

func IsValidULID(id string) bool {
	return ulidPattern.MatchString(id)
}

func ValidateName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if utf8.RuneCountInString(trimmed) < 1 || utf8.RuneCountInString(trimmed) > 200 {
		return "", fmt.Errorf("%w: name must be between 1 and 200 characters", core.ErrInvalidInput)
	}
	return trimmed, nil
}

func (s *FilesystemStore) ListProjects(ctx context.Context) ([]core.Project, error) {
	_ = ctx
	entries, err := os.ReadDir(s.projectsPath())
	if err != nil {
		return nil, err
	}

	projects := make([]core.Project, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		var project core.Project
		if err := readJSON(filepath.Join(s.projectsPath(), entry.Name()), &project); err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}

	sort.Slice(projects, func(i, j int) bool {
		return lessByNameID(projects[i].Name, projects[i].ID, projects[j].Name, projects[j].ID)
	})
	return projects, nil
}

func (s *FilesystemStore) CreateProject(ctx context.Context, name string) (*core.Project, error) {
	_ = ctx
	name, err := ValidateName(name)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	project := core.Project{
		ID:        ulid.Make().String(),
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	}

	logrus.WithFields(logrus.Fields{
		"project_id": project.ID,
		"base_path":  s.basePath,
	}).Info("Creating explorer project")

	if err := writeJSONAtomic(s.projectPath(project.ID), project); err != nil {
		return nil, err
	}
	return &project, nil
}

func (s *FilesystemStore) UpdateProject(ctx context.Context, id string, name string) (*core.Project, error) {
	_ = ctx
	if err := validateID(id); err != nil {
		return nil, err
	}
	name, err := ValidateName(name)
	if err != nil {
		return nil, err
	}

	project, err := s.readProject(id)
	if err != nil {
		return nil, err
	}
	project.Name = name
	project.UpdatedAt = time.Now().UTC()

	if err := writeJSONAtomic(s.projectPath(id), project); err != nil {
		return nil, err
	}
	return project, nil
}

func (s *FilesystemStore) DeleteProject(ctx context.Context, id string) error {
	_ = ctx
	if err := validateID(id); err != nil {
		return err
	}
	if _, err := s.readProject(id); err != nil {
		return err
	}

	files, err := s.listAllFiles()
	if err != nil {
		return err
	}
	for _, file := range files {
		if file.ProjectID != id {
			continue
		}
		if err := removeIfExists(s.fileMetaPath(file.ID)); err != nil {
			return err
		}
		if err := removeIfExists(s.fileDataPath(file.ID)); err != nil {
			return err
		}
	}
	return os.Remove(s.projectPath(id))
}

func (s *FilesystemStore) ListFiles(ctx context.Context, projectID string) ([]core.FileMeta, error) {
	_ = ctx
	if err := validateID(projectID); err != nil {
		return nil, err
	}
	if _, err := s.readProject(projectID); err != nil {
		return nil, err
	}

	allFiles, err := s.listAllFiles()
	if err != nil {
		return nil, err
	}

	files := make([]core.FileMeta, 0)
	for _, file := range allFiles {
		if file.ProjectID == projectID {
			files = append(files, file)
		}
	}
	sort.Slice(files, func(i, j int) bool {
		return lessByNameID(files[i].Name, files[i].ID, files[j].Name, files[j].ID)
	})
	return files, nil
}

func (s *FilesystemStore) CreateFile(ctx context.Context, projectID string, name string, data json.RawMessage) (*core.FileMeta, error) {
	_ = ctx
	if err := validateID(projectID); err != nil {
		return nil, err
	}
	if _, err := s.readProject(projectID); err != nil {
		return nil, err
	}
	name, err := ValidateName(name)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		data = json.RawMessage(defaultSceneJSON)
	}

	now := time.Now().UTC()
	meta := core.FileMeta{
		ID:        ulid.Make().String(),
		ProjectID: projectID,
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	}

	logrus.WithFields(logrus.Fields{
		"file_id":    meta.ID,
		"project_id": projectID,
		"base_path":  s.basePath,
	}).Info("Creating explorer file")

	if err := writeJSONAtomic(s.fileMetaPath(meta.ID), meta); err != nil {
		return nil, err
	}
	if err := writeFileAtomic(s.fileDataPath(meta.ID), data, 0644); err != nil {
		_ = removeIfExists(s.fileMetaPath(meta.ID))
		return nil, err
	}
	return &meta, nil
}

func (s *FilesystemStore) GetFile(ctx context.Context, fileID string) (*core.FileMeta, json.RawMessage, error) {
	_ = ctx
	if err := validateID(fileID); err != nil {
		return nil, nil, err
	}

	meta, err := s.readFileMeta(fileID)
	if err != nil {
		return nil, nil, err
	}
	data, err := os.ReadFile(s.fileDataPath(fileID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("%w: file data %s", core.ErrNotFound, fileID)
		}
		return nil, nil, err
	}
	return meta, json.RawMessage(data), nil
}

func (s *FilesystemStore) UpdateFile(ctx context.Context, fileID string, update core.FileUpdate) (*core.FileMeta, error) {
	_ = ctx
	if err := validateID(fileID); err != nil {
		return nil, err
	}

	meta, err := s.readFileMeta(fileID)
	if err != nil {
		return nil, err
	}

	if update.Name != nil {
		name, err := ValidateName(*update.Name)
		if err != nil {
			return nil, err
		}
		meta.Name = name
	}
	if update.ProjectID != nil {
		if err := validateID(*update.ProjectID); err != nil {
			return nil, err
		}
		if _, err := s.readProject(*update.ProjectID); err != nil {
			return nil, err
		}
		meta.ProjectID = *update.ProjectID
	}
	if update.Data != nil && len(*update.Data) == 0 {
		return nil, fmt.Errorf("%w: data must not be empty", core.ErrInvalidInput)
	}

	meta.UpdatedAt = time.Now().UTC()
	if err := writeJSONAtomic(s.fileMetaPath(fileID), meta); err != nil {
		return nil, err
	}
	if update.Data != nil {
		if err := writeFileAtomic(s.fileDataPath(fileID), *update.Data, 0644); err != nil {
			return nil, err
		}
	}
	return meta, nil
}

func (s *FilesystemStore) DeleteFile(ctx context.Context, fileID string) error {
	_ = ctx
	if err := validateID(fileID); err != nil {
		return err
	}
	if _, err := s.readFileMeta(fileID); err != nil {
		return err
	}
	if err := removeIfExists(s.fileMetaPath(fileID)); err != nil {
		return err
	}
	return removeIfExists(s.fileDataPath(fileID))
}

func (s *FilesystemStore) readProject(id string) (*core.Project, error) {
	var project core.Project
	if err := readJSON(s.projectPath(id), &project); err != nil {
		return nil, err
	}
	return &project, nil
}

func (s *FilesystemStore) readFileMeta(id string) (*core.FileMeta, error) {
	var meta core.FileMeta
	if err := readJSON(s.fileMetaPath(id), &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

func (s *FilesystemStore) listAllFiles() ([]core.FileMeta, error) {
	entries, err := os.ReadDir(s.filesPath())
	if err != nil {
		return nil, err
	}

	files := make([]core.FileMeta, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".meta.json") {
			continue
		}
		fileID := strings.TrimSuffix(entry.Name(), ".meta.json")
		if !IsValidULID(fileID) {
			return nil, fmt.Errorf("%w: invalid file metadata name %s", core.ErrInvalidInput, entry.Name())
		}
		var meta core.FileMeta
		if err := readJSON(filepath.Join(s.filesPath(), entry.Name()), &meta); err != nil {
			return nil, err
		}
		if meta.ID != fileID {
			return nil, fmt.Errorf("%w: file metadata ID does not match filename", core.ErrInvalidInput)
		}
		files = append(files, meta)
	}
	return files, nil
}

func (s *FilesystemStore) projectsPath() string {
	return filepath.Join(s.basePath, "projects")
}

func (s *FilesystemStore) filesPath() string {
	return filepath.Join(s.basePath, "files")
}

func (s *FilesystemStore) projectPath(id string) string {
	return filepath.Join(s.projectsPath(), id+".json")
}

func (s *FilesystemStore) fileMetaPath(id string) string {
	return filepath.Join(s.filesPath(), id+".meta.json")
}

func (s *FilesystemStore) fileDataPath(id string) string {
	return filepath.Join(s.filesPath(), id+".excalidraw")
}

func validateID(id string) error {
	if !IsValidULID(id) {
		return fmt.Errorf("%w: invalid ULID", core.ErrInvalidInput)
	}
	return nil
}

func readJSON(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", core.ErrNotFound, filepath.Base(path))
		}
		return err
	}
	if err := json.Unmarshal(data, value); err != nil {
		return err
	}
	return nil
}

func writeJSONAtomic(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data, 0644)
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func removeIfExists(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func lessByNameID(leftName string, leftID string, rightName string, rightID string) bool {
	left := strings.ToLower(leftName)
	right := strings.ToLower(rightName)
	if left == right {
		return leftID < rightID
	}
	return left < right
}
