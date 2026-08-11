package stores

import (
	"excalidraw-complete/core"
	"excalidraw-complete/stores/aws"
	explorerstore "excalidraw-complete/stores/explorer"
	"excalidraw-complete/stores/filesystem"
	"excalidraw-complete/stores/memory"
	"excalidraw-complete/stores/sqlite"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

func GetStore() core.DocumentStore {
	storageType := os.Getenv("STORAGE_TYPE")
	var store core.DocumentStore

	storageField := logrus.Fields{
		"storageType": storageType,
	}

	switch storageType {
	case "filesystem":
		basePath := os.Getenv("LOCAL_STORAGE_PATH")
		storageField["basePath"] = basePath
		store = filesystem.NewDocumentStore(basePath)
	case "sqlite":
		dataSourceName := os.Getenv("DATA_SOURCE_NAME")
		storageField["dataSourceName"] = dataSourceName
		store = sqlite.NewDocumentStore(dataSourceName)
	case "s3":
		bucketName := os.Getenv("S3_BUCKET_NAME")
		storageField["bucketName"] = bucketName
		store = aws.NewDocumentStore(bucketName)
	default:
		store = memory.NewDocumentStore()
		storageField["storageType"] = "in-memory"
	}
	logrus.WithFields(storageField).Info("Use storage")
	return store
}

func GetExplorerStore() core.ExplorerStore {
	basePath := os.Getenv("EXPLORER_STORAGE_PATH")
	if basePath == "" {
		localStoragePath := os.Getenv("LOCAL_STORAGE_PATH")
		if localStoragePath != "" {
			basePath = filepath.Join(localStoragePath, "explorer")
		} else {
			basePath = filepath.Join(".", "data", "explorer")
		}
	}

	logrus.WithField("basePath", basePath).Info("Use explorer storage")
	return explorerstore.NewFilesystemStore(basePath)
}
