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

const stateFile = "state.json"

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
		// Skip init if state contains playlist
		if slices.ContainsFunc(p.playlists, func(playlist *models.Playlist) bool {
			return playlist.Name == confEntry.Name
		}) {
			continue
		}

		// Init newly added playlists to state
		newPlaylist := models.NewPlaylist(p.conf, confEntry, p.logger)

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
		skipSleep := false

		playlist := p.nextPlaylist()

		res := playlist.Process()
		switch res {
		case models.ProcessResultNothing:
			skipSleep = true

		case models.ProcessResultDownload, models.ProcessResultRefresh:
			p.failureBackoffCount = 0

		case models.ProcessResultError:
			p.failureBackoffCount++
		}

		if !skipSleep {
			time.Sleep(p.sleepDuration())
		}
	}
}

func (p *Processor) nextPlaylist() *models.Playlist {
	selected := p.playlists[p.roundRobinIndex]
	p.roundRobinIndex = (p.roundRobinIndex + 1) % len(p.playlists)

	return selected
}

func (p *Processor) sleepDuration() time.Duration {
	duration := time.Minute

	for range p.failureBackoffCount {
		duration *= 2
	}

	return min(duration, time.Hour)
}

func (p *Processor) loadState() error {
	_, err := os.Stat(stateFile)
	if err != nil {
		p.playlists = []*models.Playlist{}
	}

	data, err := os.ReadFile(stateFile)
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, p.playlists)
	if err != nil {
		return err
	}

	return nil
}

func (p *Processor) saveState() error {
	data, err := json.Marshal(p.playlists, jsontext.Multiline(true))
	if err != nil {
		return err
	}

	err = os.WriteFile(stateFile, data, 0644)
	if err != nil {
		return err
	}

	return nil
}
