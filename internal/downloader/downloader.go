package downloader

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path"
	"time"

	"github.com/gavidroselj/musync/internal/config"
	"github.com/lrstanley/go-ytdlp"
)

const (
	playlistRefreshTimeout = time.Minute
	songDownloadTimeout    = time.Minute
	progressFuncInterval   = time.Second * 2
	tempFileName           = "temp-download"
	SongFileExtension      = "mp3"
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

// songDownloader returns a ytdlp instance for downloading a song. Output path should be set before running it.
func songDownloader(logger *slog.Logger) (*ytdlp.Command, context.Context, context.CancelFunc) {
	dl := ytdlp.New().
		AudioQuality("0").
		ExtractAudio().
		EmbedThumbnail().
		AudioFormat("mp3").
		EmbedMetadata().
		AbortOnError().
		SponsorblockRemove("sponsor,intro,outro,selfpromo,preview,interaction,music_offtopic").
		ProgressFunc(progressFuncInterval, func(update ytdlp.ProgressUpdate) {
			logger.Debug("Downloading...", "progress", fmt.Sprintf("%.1f%%", update.Percent()), "ETA", update.ETA())
		})

	ctx, cancel := getContext(songDownloadTimeout)
	return dl, ctx, cancel
}

// DownloadSong downloads a song to disk (because yt-dlp doesn't support direct to memory downloads), reads it and removes it from disk.
func DownloadSong(logger *slog.Logger, url string) ([]byte, error) {
	dl, ctx, cancel := songDownloader(logger)
	defer cancel()

	tempFilePath := path.Join(config.WorkDir, tempFileName)
	tempFileName := fmt.Sprintf("%s.%s", tempFilePath, SongFileExtension)

	_, err := dl.Output(tempFilePath).Run(ctx, url)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(tempFileName)
	if err != nil {
		return nil, err
	}

	err = os.Remove(tempFileName)
	if err != nil {
		return nil, err
	}

	return data, nil
}
