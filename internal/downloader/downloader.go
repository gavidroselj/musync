package downloader

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/lrstanley/go-ytdlp"
)

const (
	playlistRefreshTimeout = time.Second * 30
	songDownloadTimeout    = time.Second * 30
)

func getContext(duration time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), duration)
}

// PlaylistDownloader returns a ytdlp instance for refreshing a playlist list of songs.
func PlaylistDownloader(logger *slog.Logger) (*ytdlp.Command, context.Context, context.CancelFunc) {
	dl := ytdlp.New().
		FlatPlaylist().
		PrintJSON().
		Simulate()

	ctx, cancel := getContext(playlistRefreshTimeout)
	return dl, ctx, cancel
}

// SongDownloader returns a ytdlp instance for downloading a song. Output path should be set before running it.
func SongDownloader(logger *slog.Logger) (*ytdlp.Command, context.Context, context.CancelFunc) {
	dl := ytdlp.New().
		AudioQuality("0").
		ExtractAudio().
		EmbedThumbnail().
		AudioFormat("mp3").
		EmbedMetadata().
		AbortOnError().
		NoPart().
		SponsorblockRemove("sponsor,intro,outro,selfpromo,preview,interaction,music_offtopic").
		ProgressFunc(time.Millisecond*200, func(update ytdlp.ProgressUpdate) {
			logger.Debug("Downloading...", "progress", fmt.Sprintf("%.1f%%", update.Percent()), "ETA", update.ETA())
		})

	ctx, cancel := getContext(songDownloadTimeout)
	return dl, ctx, cancel
}
