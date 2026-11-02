package models

import (
	"fmt"
	"log/slog"
	"os"
	"path"

	"github.com/gavidroselj/musync/internal/downloader"
)

// FileManager manages the actual song files on disk, as directed by playlist it's tied to.
// It also manages files on remote filesystems, such as WebDAV.
type FileManager struct {
	logger *slog.Logger
	folder string
}

func NewFileManager(logger *slog.Logger, folder string) *FileManager {
	return &FileManager{
		logger: logger.With("module", "downloader"),
		folder: folder,
	}
}

func (fm *FileManager) DownloadSong(song *Song) error {
	fileName := fmt.Sprintf("%s [%s]", song.SanitizedTitle(), song.ID)

	dl, ctx, cancel := downloader.SongDownloader(fm.logger)
	defer cancel()
	dl.Output(path.Join(fm.folder, fileName))

	_, err := dl.Run(ctx, song.URL)
	if err != nil {
		return err
	}

	// TODO: upload to WebDAV if configured

	song.FileName = fileName

	return nil
}

func (fm *FileManager) FileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	if err != nil {
		return false
	}

	return true
}

func (fm *FileManager) DeleteSong(song *Song) error {
	filePath := path.Join(fm.folder, song.FileName)
	exists := fm.FileExists(filePath)
	if !exists {
		fm.logger.Warn("Song for deletion did not exist on disk, this should be fine", "song", song, "filePath", filePath)
		return nil
	}

	err := os.Remove(filePath)
	if err != nil {
		return err
	}

	return nil
}
