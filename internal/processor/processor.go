package processor

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"log/slog"
	"os"
	"slices"
	"time"

	"github.com/gavidroselj/musync/internal/config"
	"github.com/gavidroselj/musync/internal/models"
)

type Processor struct {
	playlists []*models.Playlist

	roundRobinIndex     int
	failureBackoffCount int

	conf   *config.Config
	logger *slog.Logger
}

func NewProcessor(conf *config.Config, logger *slog.Logger) *Processor {
	return &Processor{
		roundRobinIndex: 0,

		conf:   conf,
		logger: logger,
	}
}

func (p *Processor) Init() error {
	err := p.loadState()
	if err != nil {
		return err
	}

	// Delete state for non-configured playlists
	p.playlists = slices.DeleteFunc(p.playlists, func(playlist *models.Playlist) bool {
		return !slices.ContainsFunc(p.conf.Playlists, func(playlistEntry config.PlaylistEntry) bool {
			return playlist.Name == playlistEntry.Name
		})
	})

	for _, confEntry := range p.conf.Playlists {
		// Init playlist with state tracked
		idx := slices.IndexFunc(p.playlists, func(playlist *models.Playlist) bool {
			return playlist.Name == confEntry.Name
		})
		if idx != -1 {
			playlist := p.playlists[idx]
			playlist.ApplyConfig(p.logger, confEntry)
			continue
		}

		// Init newly added playlists to state
		newPlaylist := models.NewPlaylist(p.logger, confEntry)

		p.playlists = append(p.playlists, newPlaylist)
	}

	err = p.saveState()
	if err != nil {
		return err
	}

	return nil
}

func (p *Processor) Run() {
	for {
		p.runOnce()
	}
}

func (p *Processor) runOnce() {
	skipSleep := false

	defer func() {
		if !skipSleep {
			time.Sleep(p.sleepDuration())
		}
	}()

	playlist := p.nextPlaylist()
	if playlist == nil {
		p.logger.Debug("No playlists need actions, skipping")
		return
	}

	res := playlist.Process()
	switch res {
	case models.ProcessResultNothing:
		skipSleep = true

	case models.ProcessResultDownload, models.ProcessResultRefresh:
		p.failureBackoffCount = 0

	case models.ProcessResultError:
		p.failureBackoffCount++
	}

	if res != models.ProcessResultNothing {
		err := p.saveState()
		if err != nil {
			p.logger.Error("Failed to save state", "err", err)
		}
	}

}

func (p *Processor) nextPlaylist() *models.Playlist {
	// Rotate through all playlists from p.roundRobinIndex
	for range p.playlists {
		selected := p.playlists[p.roundRobinIndex]
		p.roundRobinIndex = (p.roundRobinIndex + 1) % len(p.playlists)

		if selected.NeedsProcessing() {
			return selected
		}
	}

	return nil
}

func (p *Processor) sleepDuration() time.Duration {
	duration := time.Minute

	for range p.failureBackoffCount {
		duration *= 2
	}

	return min(duration, time.Hour)
}

func (p *Processor) loadState() error {
	filePath := config.StateFilePath()

	_, err := os.Stat(filePath)
	if err != nil {
		return nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, &p.playlists)
	if err != nil {
		return err
	}

	return nil
}

func (p *Processor) saveState() error {
	filePath := config.StateFilePath()

	data, err := json.Marshal(p.playlists, jsontext.Multiline(true))
	if err != nil {
		return err
	}

	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		return err
	}

	return nil
}
