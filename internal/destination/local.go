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

func (ld *LocalDestination) FileExists(fileName string) (bool, error) {
	_, err := ld.root.Stat(fileName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func (ld *LocalDestination) CreateFile(fileName string, data []byte) error {
	// Abort if file already exists
	exists, err := ld.FileExists(fileName)
	if err != nil {
		return err
	}
	if exists {
		return ErrFileExists
	}

	err = ld.root.MkdirAll(path.Dir(fileName), dirMode)
	if err != nil {
		return err
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
