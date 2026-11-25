package destination

import (
	"errors"
	"log/slog"
	"os"
	"path"
)

type LocalDestination struct {
	logger *slog.Logger
	root   *os.Root
}

func NewLocalDestination(logger *slog.Logger, rootPath string) (*LocalDestination, error) {
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return nil, err
	}

	ld := &LocalDestination{
		logger: logger.With("destinationType", "local", "rootPath", rootPath),
		root:   root,
	}

	return ld, nil
}

func (ld *LocalDestination) ensureDestination(dir string) error {
	err := ld.root.MkdirAll(dir, dirMode)
	if err != nil {
		return err
	}

	return nil
}

func (ld *LocalDestination) FileExists(fileName string) (bool, error) {
	err := ld.ensureDestination(path.Dir(fileName))
	if err != nil {
		return false, err
	}

	_, err = ld.root.Stat(fileName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func (ld *LocalDestination) ListFiles(dir string) ([]string, error) {
	err := ld.ensureDestination(dir)
	if err != nil {
		return nil, err
	}

	dirFile, err := ld.root.Open(dir)
	if err != nil {
		return nil, err
	}

	entries, err := dirFile.ReadDir(0)
	if err != nil {
		return nil, err
	}

	results := []string{}
	for _, entry := range entries {
		results = append(results, entry.Name())
	}

	return results, nil
}

func (ld *LocalDestination) CreateFile(fileName string, data []byte) error {
	err := ld.ensureDestination(path.Dir(fileName))
	if err != nil {
		return err
	}

	// Abort if file already exists
	exists, err := ld.FileExists(fileName)
	if err != nil {
		return err
	}
	if exists {
		return ErrFileExists
	}

	err = ld.root.WriteFile(fileName, data, fileMode)
	if err != nil {
		return err
	}

	return nil
}

func (ld *LocalDestination) DeleteFile(fileName string) error {
	exists, err := ld.FileExists(fileName)
	if err != nil {
		return err
	}
	if !exists {
		ld.logger.Warn("File for deletion did not exist on disk, this should not be a problem", "file", fileName)
		return nil
	}

	err = ld.root.Remove(fileName)
	if err != nil {
		return err
	}

	return nil
}
