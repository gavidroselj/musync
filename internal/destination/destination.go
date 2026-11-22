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
	FileExists(name string) (bool, error)
	CreateFile(name string, data []byte) error
	DeleteFile(name string) error
}
