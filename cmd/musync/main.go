package main

import (
	"log/slog"
	"os"

	"github.com/gavidroselj/musync/internal/config"
	"github.com/gavidroselj/musync/internal/dependencies"
	"github.com/gavidroselj/musync/internal/processor"
	"github.com/lrstanley/go-ytdlp"
)

func main() {
	logLevel := new(slog.LevelVar)
	logLevel.Set(slog.LevelDebug)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	conf, err := config.LoadConfig()
	if err != nil {
		logger.Error("Failed to load config", "path", config.ConfigFilePath(), "err", err)
		os.Exit(1)
	}
	logLevel.Set(conf.LogLevel)
	logger.Info("Successfully loaded config", "playlists", len(conf.Playlists))

	dependencies.DownloadDependencies(logger)

	logger.Info("muSync starting", "yt-dlp", ytdlp.Version, "logLevel", logLevel)

	processor := processor.NewProcessor(conf, logger)
	err = processor.Init()
	if err != nil {
		logger.Error("Startup failed to init processor", "err", err)
		os.Exit(1)
	}

	processor.Run()
}
