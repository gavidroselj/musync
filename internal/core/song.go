package core

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/gavidroselj/musync/internal/common"
	"github.com/gavidroselj/musync/internal/downloader"
	"github.com/lrstanley/go-ytdlp"
)

type SongPlaylistState string

const (
	SongPlaylistStateOk      SongPlaylistState = "OK"      // Song is in playlist
	SongPlaylistStateMissing SongPlaylistState = "MISSING" // Song is no longer in playlist, it will be deleted if it does not reappear soon
)

type SongDownloadState string

const (
	SongDownloadStatePending           SongDownloadState = "PENDING"            // No download attempt has been made for this song yet
	SongDownloadStateFailedRetry       SongDownloadState = "FAILED_RETRY"       // Last download attempt failed, will retry
	SongDownloadStateFailedUnavailable SongDownloadState = "FAILED_UNAVAILABLE" // Last download attempt failed, song was detected as unavailable, will not retry
	SongDownloadStateSuccess           SongDownloadState = "SUCCESS"            // Song is downloaded an exists at target
	SongDownloadStateMissing           SongDownloadState = "MISSING"            // Song was downloaded, but file no longer exists at target
)

type Song struct {
	ID    string `json:"id"`
	URL   string `json:"url"`
	Title string `json:"title"`

	// DeletionMargin is used to prevent songs from being deleted from a single playlist refresh that fails to fetch the full playlist.
	// This value will increase every time a playlist refresh does not contain this song.
	// The more songs that are removed from a playlist, the less this value will increase.
	// A song will only be deleted when this value is greater or equal to 1.
	DeletionMargin float64 `json:"deletion_margin"`

	PlaylistState SongPlaylistState `json:"playlist_state"`
	DownloadState SongDownloadState `json:"download_state"`

	// SavedFileName stores the file name the song has on disk. It does not include the destination root or the playlist name path.
	SavedFileName string `json:"saved_file_name"`

	logger *slog.Logger `json:"-"`
}

func NewSong(id, url, title string, logger *slog.Logger) Song {
	return Song{
		ID:    id,
		URL:   url,
		Title: title,

		DeletionMargin: 0,

		PlaylistState: SongPlaylistStateOk,
		DownloadState: SongDownloadStatePending,

		logger: logger.With("songID", id),
	}
}

func NewSongFromInfo(info *ytdlp.ExtractedInfo, logger *slog.Logger) Song {
	url := common.SafeDeref(info.URL)
	if url == "" {
		url = info.ExtractedFormat.URL
	}
	if url == "" {
		url = common.SafeDeref(info.WebpageURL)
	}

	return NewSong(info.ID, url, common.SafeDeref(info.Title), logger)
}

func (s *Song) resetDeletionMargin() {
	s.DeletionMargin = 0
}

// Increment the song's DeletionMargin by marginIncrease, and check if it is >= 1.
// Returns true if song should be deleted.
func (s *Song) ShouldDelete(marginIncrease float64) bool {
	s.DeletionMargin += marginIncrease

	// If song has not been marked as missing yet, it should never be deleted
	if s.PlaylistState != SongPlaylistStateMissing {
		return false
	}

	if s.DeletionMargin < 1 {
		return false
	}

	return true
}

// updateFromNewEntry is used to update only the song metadata from the new song entry.
func (s *Song) updateMetadata(newSong Song) {
	s.URL = newSong.URL
	s.Title = newSong.Title
}

// sanitizedTitle returns the song title with all potentially problematic characters for filename removed
func (s *Song) sanitizedTitle() string {
	const replacementChar = "-"

	title := strings.ReplaceAll(s.Title, "<", replacementChar)
	title = strings.ReplaceAll(title, ">", replacementChar)
	title = strings.ReplaceAll(title, ":", replacementChar)
	title = strings.ReplaceAll(title, "\"", replacementChar)
	title = strings.ReplaceAll(title, "/", replacementChar)
	title = strings.ReplaceAll(title, "\\", replacementChar)
	title = strings.ReplaceAll(title, "|", replacementChar)
	title = strings.ReplaceAll(title, "?", replacementChar)
	title = strings.ReplaceAll(title, "*", replacementChar)

	return title
}

// RegisterSavedFileName creates the song file name and stores it in the s.SavedFileName field.
func (s *Song) RegisterSavedFileName() {
	s.SavedFileName = fmt.Sprintf("%s [%s].%s", s.sanitizedTitle(), s.ID, downloader.SongFileExtension)
}
