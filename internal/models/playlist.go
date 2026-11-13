package models

import (
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/gavidroselj/musync/internal/config"
	"github.com/gavidroselj/musync/internal/downloader"
)

type Playlist struct {
	Name       string    `json:"name"`
	URL        string    `json:"url"`
	Songs      []*Song   `json:"songs"`
	Path       string    `json:"path"`
	LastUpdate time.Time `json:"last_update"`

	fileManager *FileManager
	logger      *slog.Logger
}

func NewPlaylist(logger *slog.Logger, pe config.PlaylistEntry) *Playlist {
	playlist := &Playlist{
		Name:       pe.Name,
		Songs:      []*Song{},
		LastUpdate: time.UnixMilli(0),
	}

	playlist.ApplyConfig(logger, pe)

	return playlist
}

type ProcessResult string

const (
	ProcessResultNothing  ProcessResult = "NOTHING"
	ProcessResultRefresh  ProcessResult = "REFRESH"
	ProcessResultDownload ProcessResult = "DOWNLOAD"
	ProcessResultError    ProcessResult = "ERROR"
)

func (p *Playlist) Process() ProcessResult {
	if p.needsRefresh() {
		p.Refresh()
		return ProcessResultRefresh
	}

	return p.downloadIfAvailable()
}

func (p *Playlist) NeedsProcessing() bool {
	if p.needsRefresh() {
		return true
	}

	if p.getNextDownload() != nil {
		return true
	}

	return false
}

func (p *Playlist) needsRefresh() bool {
	return p.LastUpdate.Before(time.Now().Add(-time.Hour * 6))
}

func (p *Playlist) Refresh() {
	p.logger.Debug("Refreshing playlist from remote...")
	newList, err := p.fetchPlaylistSongs()
	if err != nil {
		p.logger.Error("Failed to update playlist", "err", err)
		return
	}

	p.updateTrackedSongs(newList)
	p.LastUpdate = time.Now()
}

func (p *Playlist) ApplyConfig(logger *slog.Logger, pe config.PlaylistEntry) {
	p.URL = pe.URL
	p.Path = pe.Path

	p.logger = logger.With("playlist", p.Name)

	p.fileManager = NewFileManager(p.logger, p.Path)

	p.validateFiles()
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

func (p *Playlist) updateTrackedSongs(newList []Song) {
	// Only allow 0 length playlists, if it had less than 5 songs before
	if len(newList) == 0 && len(p.Songs) > 5 {
		p.logger.Warn("Updating songs from list of length 0, rejecting")
		return
	}

	newSongCount, existingSongCount, awaitingDeletionCount, deletedSongCount := 0, 0, 0, 0

	for _, newSong := range newList {
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

	// Delete songs on disk
	for _, song := range p.Songs {
		if !slices.ContainsFunc(newList, func(a Song) bool {
			return song.ID == a.ID
		}) {
			shouldDelete := song.ShouldDelete(0.1) // TODO: Make this dynamic based on how many songs were deleted.

			if shouldDelete && p.fileManager != nil {
				err := p.fileManager.DeleteSong(song)
				if err != nil {
					p.logger.Error("Failed to delete song", "song", song, "err", err)
				}

				deletedSongCount++
			} else {
				song.PlaylistState = SongPlaylistStateMissing
				awaitingDeletionCount++
			}
		}
	}

	// And remove them from tracking
	p.Songs = slices.DeleteFunc(p.Songs, func(song *Song) bool {
		return song.ShouldDelete(0)
	})

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
func (p *Playlist) downloadIfAvailable() ProcessResult {
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
		if song.DownloadState == SongDownloadStatePending ||
			song.DownloadState == SongDownloadStateFailedRetry ||
			song.DownloadState == SongDownloadStateFailedUnavailable {
			// Song is not downloaded yet, skip
			continue
		}

		path := p.fileManager.GetSongPath(song)
		if p.fileManager.FileExists(path) {
			song.DownloadState = SongDownloadStateSuccess
		} else {
			p.logger.Warn("Expected file is missing", "name", song.FileName, "id", song.ID, "path", path)
			song.DownloadState = SongDownloadStateMissing
			// TODO: Figure out how to handle missing files
		}
	}
}
