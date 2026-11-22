package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldDelete(t *testing.T) {
	t.Run("yes", func(t *testing.T) {
		song := Song{
			DeletionMargin: 1,
			PlaylistState:  SongPlaylistStateMissing,
		}

		res := song.ShouldDelete(0)
		assert.Equal(t, true, res)
	})

	t.Run("yes-increment", func(t *testing.T) {
		song := Song{
			DeletionMargin: 0.9,
			PlaylistState:  SongPlaylistStateMissing,
		}

		res := song.ShouldDelete(0.2)
		assert.Equal(t, true, res)
	})

	t.Run("no-playlist-ok", func(t *testing.T) {
		song := Song{
			DeletionMargin: 1,
			PlaylistState:  SongPlaylistStateOk,
		}

		res := song.ShouldDelete(0)
		assert.Equal(t, false, res)
	})

	t.Run("no-under-margin", func(t *testing.T) {
		song := Song{
			DeletionMargin: 0.3,
			PlaylistState:  SongPlaylistStateMissing,
		}

		res := song.ShouldDelete(0.6)
		assert.Equal(t, false, res)
	})
}
