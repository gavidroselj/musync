package destination

import "errors"

const (
	fileMode = 0644
	dirMode  = 0755
)

var (
	ErrFileExists = errors.New("file exists")
)

type Destination interface {
	// FileExists returns whether a file with the provided fileName exists.
	FileExists(fileName string) (bool, error)

	// ListFiles returns the filenames at the provided directory.
	ListFiles(dir string) ([]string, error)

	// CreateFile writes the provided data to the provided fileName.
	CreateFile(fileName string, data []byte) error

	// DeleteFile deletes the file at the provided fileName.
	// If no file exists, deletion is treated as successful.
	DeleteFile(fileName string) error
}

type TestDestination struct{}

func (td *TestDestination) FileExists(fileName string) (bool, error) {
	return true, nil
}

func (td *TestDestination) ListFiles(dir string) ([]string, error) {
	return []string{}, nil
}

func (td *TestDestination) CreateFile(fileName string, data []byte) error {
	return nil
}

func (td *TestDestination) DeleteFile(fileName string) error {
	return nil
}
