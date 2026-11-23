package core

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gavidroselj/musync/internal/common"
	"github.com/gavidroselj/musync/internal/downloader"
	"github.com/lrstanley/go-ytdlp"
)

const (
	songMaxDownloadAttempts                = 5
	songMaxDownloadAttemptsBackoffDuration = time.Hour * 24
	songUnavailableBackoffDuration         = time.Hour * 24 * 14
)

type SongPlaylistState string

const (
	SongPlaylistStateOk      SongPlaylistState = "OK"      // Song is in playlist
	SongPlaylistStateMissing SongPlaylistState = "MISSING" // Song is no longer in playlist, it will be deleted if it does not reappear soon
)

type SongDownloadState string

const (
	SongDownloadStatePending       SongDownloadState = "PENDING"        // No download attempt has been made for this song yet
	SongDownloadStateFailedRetry   SongDownloadState = "FAILED_RETRY"   // Last download attempt failed, will retry
	SongDownloadStateFailedBackoff SongDownloadState = "FAILED_BACKOFF" // Multiple download attempts failed, will not retry for a while
	SongDownloadStateUnavailable   SongDownloadState = "UNAVAILABLE"    // Song was detected as unavailable, will not retry for a long while
	SongDownloadStateSuccess       SongDownloadState = "SUCCESS"        // Song is downloaded and exists at destination
	SongDownloadStateMissing       SongDownloadState = "MISSING"        // Song was downloaded, but file no longer exists at destination
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

	// PlaylistState stores whether the song is currently in the remote playlist or not
	PlaylistState SongPlaylistState `json:"playlist_state"`

	// DownloadState stores whether the song has been downloaded yet or not
	DownloadState SongDownloadState `json:"download_state"`

	// DownloadRetryCount counts how many times a download has failed on this song
	DownloadRetryCount int `json:"download_retry_count"`
	// DownloadBackoffTime specifies when the next download attempt will be made, when the song has hit too many retries
	DownloadBackoffTime time.Time `json:"download_backoff_time"`

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

		PlaylistState:       SongPlaylistStateOk,
		DownloadState:       SongDownloadStatePending,
		DownloadRetryCount:  0,
		DownloadBackoffTime: time.UnixMilli(0),

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

// ShouldDownload returns true if this song is currently eligible for a download attempt
func (s *Song) ShouldDownload() bool {
	// Download has already been complete
	// TODO: Missing file handling
	if s.DownloadState == SongDownloadStateSuccess || s.DownloadState == SongDownloadStateMissing {
		return false
	}

	// No issues
	if s.DownloadState == SongDownloadStatePending {
		return true
	}
	// Retry below limit
	if s.DownloadState == SongDownloadStateFailedRetry {
		return true
	}

	// Song is in backoff, check if backoff time has passed
	return time.Now().After(s.DownloadBackoffTime)
}

// RegisterFailedDownloadAttempt updates the song DownloadState and related fields
func (s *Song) RegisterFailedDownloadAttempt(markUnavailable bool) {
	if markUnavailable {
		s.DownloadState = SongDownloadStateUnavailable
		s.DownloadRetryCount = 0
		s.DownloadBackoffTime = time.Now().Add(songUnavailableBackoffDuration)
		return
	}

	s.DownloadState = SongDownloadStateFailedRetry
	s.DownloadRetryCount++

	if s.DownloadRetryCount >= songMaxDownloadAttempts {
		s.DownloadState = SongDownloadStateFailedBackoff
		s.DownloadRetryCount = 0
		s.DownloadBackoffTime = time.Now().Add(songMaxDownloadAttemptsBackoffDuration)
	}
}

// RegisterDownloadSuccess stores the fileName and updates song DownloadState and related fields.
// After running this, use s.SavedFileName for all further file operations.
func (s *Song) RegisterDownloadSuccess(fileName string) {
	s.SavedFileName = fileName
	s.DownloadState = SongDownloadStateSuccess
	s.DownloadRetryCount = 0
	s.DownloadBackoffTime = time.UnixMilli(0)
}

// GetFileName returns the suggested file name for this song
func (s *Song) GetFileName() string {
	return fmt.Sprintf("%s [%s].%s", s.sanitizedTitle(), s.ID, downloader.SongFileExtension)
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
