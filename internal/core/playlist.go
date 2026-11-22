package core

import (
	"log/slog"
	"path"
	"slices"
	"time"

	"github.com/gavidroselj/musync/internal/config"
	"github.com/gavidroselj/musync/internal/destination"
	"github.com/gavidroselj/musync/internal/downloader"
)

const (
	playlistRefreshTimeout = time.Hour * 6
)

type Playlist struct {
	Name       string    `json:"name"`
	URL        string    `json:"url"`
	Songs      []*Song   `json:"songs"`
	LastUpdate time.Time `json:"last_update"`

	destination destination.Destination
	logger      *slog.Logger
}

func NewPlaylist(logger *slog.Logger, pe config.PlaylistEntry, destination destination.Destination) *Playlist {
	playlist := &Playlist{
		Name:       pe.Name,
		Songs:      []*Song{},
		LastUpdate: time.UnixMilli(0),
	}

	playlist.ApplyConfig(logger, pe, destination)

	return playlist
}

func (p *Playlist) ApplyConfig(logger *slog.Logger, pe config.PlaylistEntry, destination destination.Destination) {
	p.URL = pe.URL

	p.logger = logger.With("playlist", p.Name)
	p.destination = destination

	p.validateFiles()
}

type ProcessResult string

const (
	ProcessResultNothing       ProcessResult = "NOTHING"
	ProcessResultRefresh       ProcessResult = "REFRESH"
	ProcessResultDownload      ProcessResult = "DOWNLOAD"
	ProcessResultErrorContinue ProcessResult = "ERROR_CONTINUE"
	ProcessResultErrorBackoff  ProcessResult = "ERROR_BACKOFF"
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
	return p.LastUpdate.Before(time.Now().Add(-playlistRefreshTimeout))
}

func (p *Playlist) Refresh() {
	p.logger.Info("Refreshing playlist from remote...")
	newList, err := p.fetchPlaylistSongs()
	if err != nil {
		p.logger.Error("Failed to update playlist", "err", err)
		return
	}

	p.updateTrackedSongs(newList)
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

			if shouldDelete {
				fileName := p.songPath(song)
				err := p.destination.DeleteFile(fileName)
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

	p.logger.Info("Playlist updated", "newSongs", newSongCount, "existingSongs", existingSongCount, "awaitingDeletion", awaitingDeletionCount, "deletedSongs", deletedSongCount)
}

// getNextDownload returns the next song that should be downloaded. It returns nil if none exist.
func (p *Playlist) getNextDownload() *Song {
	for i := range p.Songs {
		song := p.Songs[len(p.Songs)-1-i]
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
	data, err := downloader.DownloadSong(p.logger, targetSong.URL)
	if err != nil {
		p.logger.Error("Failed to download song", "err", err)

		if downloader.IsErrVideoUnavailable(err) {
			p.logger.Debug("Song identified as unavailable, will not retry")
			targetSong.DownloadState = SongDownloadStateFailedUnavailable
			return ProcessResultErrorContinue
		}

		if targetSong.DownloadState == SongDownloadStateFailedRetry {
			targetSong.DownloadState = SongDownloadStateFailedUnavailable
		} else {
			targetSong.DownloadState = SongDownloadStateFailedRetry
		}

		return ProcessResultErrorBackoff
	}
	p.logger.Debug("Song downloaded successfuly, saving...")

	targetSong.RegisterSavedFileName()
	fileName := p.songPath(targetSong)

	err = p.destination.CreateFile(fileName, data)
	if err != nil {
		p.logger.Error("Failed to write song to destination", "err", err)
		targetSong.SavedFileName = ""
		targetSong.DownloadState = SongDownloadStateFailedRetry
		return ProcessResultErrorContinue
	}
	p.logger.Debug("Song successfuly written to destination")

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

		fileName := p.songPath(song)
		exists, err := p.destination.FileExists(fileName)
		if err != nil {
			p.logger.Error("Failed to check whether file exists", "fileName", fileName, "err", err)
			continue
		}

		if exists {
			song.DownloadState = SongDownloadStateSuccess
		} else {
			p.logger.Warn("Expected file is missing", "fileName", fileName, "id", song.ID)
			song.DownloadState = SongDownloadStateMissing
			// TODO: Figure out how to handle missing files
		}
	}
}

func (p *Playlist) songPath(song *Song) string {
	return path.Join(p.Name, song.SavedFileName)
}
