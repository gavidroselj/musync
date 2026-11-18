package core

import (
	"log/slog"
	"testing"
	"time"

	"github.com/gavidroselj/golf"
)

func testPlaylist() *Playlist {
	return &Playlist{
		Name:       "test",
		URL:        "test/playlist",
		LastUpdate: time.Date(2026, 10, 10, 10, 10, 10, 0, time.UTC),
		Songs: []*Song{
			{
				ID:             "1",
				DeletionMargin: 0,
				PlaylistState:  SongPlaylistStateOk,
			},
			{
				ID:             "2",
				DeletionMargin: 0,
				PlaylistState:  SongPlaylistStateOk,
			},
			{
				ID:             "3",
				DeletionMargin: 0.9,
				PlaylistState:  SongPlaylistStateMissing,
			},
		},

		logger: slog.Default(),
	}
}

func newSongList(ids []string) []Song {
	songs := []Song{}

	for _, id := range ids {
		songs = append(songs, Song{
			ID: id,
		})
	}

	return songs
}

func TestUpdateTrackedSongs(t *testing.T) {
	t.Run("all-exist", func(t *testing.T) {
		p := testPlaylist()
		p.updateTrackedSongs(newSongList([]string{"1", "2", "3"}))

		golf.AssertJson(t, p.Songs, nil)
	})

	t.Run("without-3", func(t *testing.T) {
		p := testPlaylist()
		p.updateTrackedSongs(newSongList([]string{"1", "2"}))

		golf.AssertJson(t, p.Songs, nil)
	})

	t.Run("empty", func(t *testing.T) {
		p := testPlaylist()
		p.updateTrackedSongs([]Song{})

		golf.AssertJson(t, p.Songs, nil)
	})

	t.Run("with-new", func(t *testing.T) {
		p := testPlaylist()
		p.updateTrackedSongs(newSongList([]string{"1", "2", "3", "4"}))

		golf.AssertJson(t, p.Songs, nil)
	})
}
