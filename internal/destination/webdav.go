package destination

import (
	"errors"
	"log/slog"
	"os"
	"path"

	"github.com/studio-b12/gowebdav"
)

type WebdavDestination struct {
	logger *slog.Logger
	client *gowebdav.Client
}

func NewWebdavDestination(logger *slog.Logger, rootURL, username, password string) (*WebdavDestination, error) {
	client := gowebdav.NewClient(rootURL, username, password)
	err := client.Connect()
	if err != nil {
		return nil, err
	}

	ld := &WebdavDestination{
		logger: logger.With("destinationType", "webdav", "rootURL", rootURL, "username", username),
		client: client,
	}

	return ld, nil
}

func (wd *WebdavDestination) FileExists(name string) (bool, error) {
	_, err := wd.client.Stat(name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func (wd *WebdavDestination) CreateFile(name string, data []byte) error {
	// Abort if file already exists
	exists, err := wd.FileExists(name)
	if err != nil {
		return err
	}
	if exists {
		return ErrFileExists
	}

	err = wd.client.MkdirAll(path.Dir(name), dirMode)
	if err != nil {
		return err
	}

	err = wd.client.Write(name, data, fileMode)
	if err != nil {
		return err
	}

	return nil
}

func (wd *WebdavDestination) DeleteFile(name string) error {
	exists, err := wd.FileExists(name)
	if err != nil {
		return err
	}
	if !exists {
		wd.logger.Warn("File for deletion did not exist on disk, this should not be a problem", "file", name)
		return nil
	}

	err = wd.client.Remove(name)
	if err != nil {
		return err
	}

	return nil
}
