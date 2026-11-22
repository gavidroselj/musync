package processor

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"log/slog"
	"os"
	"slices"
	"time"

	"github.com/gavidroselj/musync/internal/config"
	"github.com/gavidroselj/musync/internal/core"
	"github.com/gavidroselj/musync/internal/destination"
)

var ErrMissingDestination = errors.New("missing destination")

const (
	maxBackoffTime = time.Minute * 30
)

type Processor struct {
	playlists []*core.Playlist

	roundRobinIndex     int
	failureBackoffCount int

	destinations map[string]destination.Destination
	conf         *config.Config
	logger       *slog.Logger
}

func NewProcessor(conf *config.Config, logger *slog.Logger) *Processor {
	return &Processor{
		roundRobinIndex: 0,

		destinations: map[string]destination.Destination{},
		conf:         conf,
		logger:       logger,
	}
}

func (p *Processor) Init() error {
	err := p.loadState()
	if err != nil {
		return err
	}

	err = p.prepareDestinations()
	if err != nil {
		return err
	}

	err = p.preparePlaylists()
	if err != nil {
		return err
	}

	err = p.saveState()
	if err != nil {
		return err
	}

	return nil
}

func (p *Processor) prepareDestinations() error {
	for _, ldEntry := range p.conf.Destinations.LocalDestinations {
		ld, err := destination.NewLocalDestination(p.logger, ldEntry.Root)
		if err != nil {
			return err
		}

		p.destinations[ldEntry.Name] = ld
	}

	for _, wdEntry := range p.conf.Destinations.WebdavDestinations {
		wd, err := destination.NewWebdavDestination(p.logger, wdEntry.RootURL, wdEntry.User, wdEntry.Password)
		if err != nil {
			return err
		}

		p.destinations[wdEntry.Name] = wd
	}

	return nil
}

func (p *Processor) preparePlaylists() error {
	// Initialize configured playlists
	for _, confEntry := range p.conf.Playlists {
		dest, ok := p.destinations[confEntry.Destination]
		if !ok {
			p.logger.Error("Missing destination entry for playlist", "destination", confEntry.Destination)
			return ErrMissingDestination
		}

		// Init playlist with state tracked
		idx := slices.IndexFunc(p.playlists, func(playlist *core.Playlist) bool {
			return playlist.Name == confEntry.Name
		})
		if idx != -1 {
			playlist := p.playlists[idx]

			playlist.ApplyConfig(p.logger, confEntry, dest)
			continue
		}

		// Init newly added playlists to state
		newPlaylist := core.NewPlaylist(p.logger, confEntry, dest)

		p.playlists = append(p.playlists, newPlaylist)
	}

	// Delete state for non-configured playlists
	p.playlists = slices.DeleteFunc(p.playlists, func(playlist *core.Playlist) bool {
		return !slices.ContainsFunc(p.conf.Playlists, func(playlistEntry config.PlaylistEntry) bool {
			return playlist.Name == playlistEntry.Name
		})
	})

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
			if p.failureBackoffCount > 0 {
				p.logger.Debug("Backing off due to download failures", "failureCount", p.failureBackoffCount, "sleepDuration", p.sleepDuration())
			}
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
	case core.ProcessResultNothing:
		skipSleep = true

	case core.ProcessResultDownload, core.ProcessResultRefresh:
		p.failureBackoffCount = 0

	case core.ProcessResultErrorBackoff:
		p.failureBackoffCount++
	}

	if res != core.ProcessResultNothing {
		err := p.saveState()
		if err != nil {
			p.logger.Error("Failed to save state", "err", err)
		}
	}

}

func (p *Processor) nextPlaylist() *core.Playlist {
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

	return min(duration, maxBackoffTime)
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
