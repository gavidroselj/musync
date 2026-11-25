package destination

import (
	"log/slog"
	"path"
	"strings"

	"github.com/studio-b12/gowebdav"
)

const (
	fileNotExistStatPrefix = "PROPFIND "
)

type WebdavDestination struct {
	logger *slog.Logger
	client *gowebdav.Client
}

func NewWebdavDestination(logger *slog.Logger, rootURL, user, password string) (*WebdavDestination, error) {
	client := gowebdav.NewClient(rootURL, user, password)
	err := client.Connect()
	if err != nil {
		return nil, err
	}

	ld := &WebdavDestination{
		logger: logger.With("destinationType", "webdav", "rootURL", rootURL, "user", user),
		client: client,
	}

	return ld, nil
}

func (wd *WebdavDestination) FileExists(name string) (bool, error) {
	_, err := wd.client.Stat(name)
	if err != nil {
		if strings.HasPrefix(err.Error(), fileNotExistStatPrefix) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func (wd *WebdavDestination) ListFiles(dir string) ([]string, error) {
	entries, err := wd.client.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	results := []string{}

	for _, entry := range entries {
		results = append(results, entry.Name())
	}

	return results, nil
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
