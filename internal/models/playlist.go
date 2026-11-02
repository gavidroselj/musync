package models

import (
	"log/slog"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/gavidroselj/musync/internal/config"
	"github.com/gavidroselj/musync/internal/downloader"
)

var fileIDRegex = regexp.MustCompile(`.*\[(.*)\].mp3`)

type Playlist struct {
	Name       string    `json:"name"`
	URL        string    `json:"url"`
	Songs      []*Song   `json:"songs"`
	Folder     string    `json:"folder"`
	LastUpdate time.Time `json:"last_update"`

	fileManager *FileManager
	logger      *slog.Logger
}

func NewPlaylist(c *config.Config, pe config.PlaylistEntry, logger *slog.Logger) *Playlist {
	playlist := Playlist{
		Name:       pe.Name,
		URL:        pe.URL,
		Folder:     path.Join(c.MusicFolder, pe.Subfolder),
		Songs:      []*Song{},
		LastUpdate: time.UnixMilli(0),

		logger: logger.With("playlist", pe.Name),
	}

	playlist.fileManager = NewFileManager(playlist.logger, playlist.Folder)

	playlist.validateFiles()
	return &playlist
}

type ProcessResult string

const (
	ProcessResultNothing  ProcessResult = "NOTHING"
	ProcessResultRefresh  ProcessResult = "REFRESH"
	ProcessResultDownload ProcessResult = "DOWNLOAD"
	ProcessResultError    ProcessResult = "ERROR"
)

func (p *Playlist) Process() ProcessResult {
	if p.LastUpdate.Before(time.Now().Add(-time.Hour * 6)) {
		p.Refresh()
		return ProcessResultRefresh
	}

	return p.DownloadIfAvailable()
}

func (p *Playlist) Refresh() {
	p.logger.Debug("Refreshing playlist from remote...")
	newSongs, err := p.fetchPlaylistSongs()
	if err != nil {
		p.logger.Error("Failed to update playlist", "err", err)
		return
	}
	if len(newSongs) == 0 {
		p.logger.Debug("Refresh returned playlist of length 0, skipping")
		return
	}

	p.updateTrackedSongs(newSongs)
	p.LastUpdate = time.Now()
}

// Get song list from URL
func (p *Playlist) fetchPlaylistSongs() ([]Song, error) {
	dl, ctx, cancel := downloader.PlaylistDownloader(p.logger)
	defer cancel()

	result, err := dl.Run(ctx, p.URL)
	if err != nil {
		p.logger.Error("Failed to query playlist", "err", err)
		return nil, err
	}

	info, err := result.GetExtractedInfo()
	if err != nil {
		p.logger.Error("Failed to extract playlist info", "err", err)
		return nil, err
	}

	songs := []Song{}

	for _, song := range info {
		songs = append(songs, NewSongFromInfo(song, p.logger))
	}

	return songs, nil
}

func (p *Playlist) songIndex(id string) int {
	return slices.IndexFunc(p.Songs, func(s *Song) bool {
		return s.ID == id
	})
}

func (p *Playlist) updateTrackedSongs(newSongs []Song) {
	newSongCount, existingSongCount, awaitingDeletionCount, deletedSongCount := 0, 0, 0, 0

	for _, newSong := range newSongs {
		songIdx := p.songIndex(newSong.ID)
		if songIdx != -1 {
			// song is already tracked
			song := p.Songs[songIdx]

			song.resetDeletionMargin()
			song.updateMetadata(newSong)
			song.PlaylistState = SongPlaylistStateOk

			existingSongCount++
		} else {
			// song is untracked (new)
			newSong.PlaylistState = SongPlaylistStateOk
			p.Songs = append(p.Songs, &newSong)
			newSongCount++
		}
	}

	// handle deleted songs
	// TODO: Validate that this does not cause a problem with indexing
	for i, song := range p.Songs {
		if !slices.ContainsFunc(newSongs, func(a Song) bool {
			return song.ID == a.ID
		}) {
			shouldDelete := song.ShouldDelete(0.1) // TODO: Make this dynamic based on how many songs were deleted.

			if shouldDelete {
				p.fileManager.DeleteSong(song)
				slices.Delete(p.Songs, i, i+1)
				deletedSongCount++
			} else {
				song.PlaylistState = SongPlaylistStateMissing
				awaitingDeletionCount++
			}
		}
	}

	p.logger.Debug("Playlist updated", "newSongs", newSongCount, "existingSongs", existingSongCount, "awaitingDeletion", awaitingDeletionCount, "deletedSongs", deletedSongCount)
}

// getNextDownload returns the next song that should be downloaded. It returns nil if none exist.
func (p *Playlist) getNextDownload() *Song {
	for _, song := range p.Songs {
		if song.DownloadState == SongDownloadStatePending || song.DownloadState == SongDownloadStateFailedRetry {
			return song
		}
	}

	return nil
}

// Download one queue entry if available, return true if download occured
// TODO: return whether download was attempted for backoff tracking
func (p *Playlist) DownloadIfAvailable() ProcessResult {
	p.logger.Debug("Attempting download...")

	targetSong := p.getNextDownload()
	if targetSong == nil {
		p.logger.Debug("Nothing to do, skipping")
		return ProcessResultNothing
	}

	p.logger.Info("Downloading song", "id", targetSong.ID, "url", targetSong.URL, "title", targetSong.Title)
	err := p.fileManager.DownloadSong(targetSong)
	if err != nil {
		p.logger.Error("Failed to download song", "err", err)

		if targetSong.DownloadState == SongDownloadStateFailedRetry || strings.Contains(err.Error(), "Video unavailable.") || strings.Contains(err.Error(), "Private video.") {
			targetSong.DownloadState = SongDownloadStateFailedUnavailable
		} else {
			targetSong.DownloadState = SongDownloadStateFailedRetry
		}

		return ProcessResultError
	}

	p.logger.Debug("Song downloaded successfuly")
	targetSong.DownloadState = SongDownloadStateSuccess
	return ProcessResultDownload
}

func (p *Playlist) validateFiles() {
	for _, song := range p.Songs {
		if !p.fileManager.FileExists(song.FileName) {
			p.logger.Warn("Expected file is missing", "name", song.FileName, "id", song.ID)
			song.DownloadState = SongDownloadStateMissing
		}
	}
}
