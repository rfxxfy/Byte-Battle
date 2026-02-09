package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDuelStatus_Constants(t *testing.T) {
	tests := []struct {
		status   DuelStatus
		expected string
	}{
		{DuelStatusPending, "pending"},
		{DuelStatusActive, "active"},
		{DuelStatusFinished, "finished"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.status))
		})
	}
}

func TestPlayer_Struct(t *testing.T) {
	player := Player{ID: 42}

	assert.Equal(t, 42, player.ID)
}

func TestPlayer_ZeroValue(t *testing.T) {
	var player Player

	assert.Equal(t, 0, player.ID)
}

func TestNewDuelRepository(t *testing.T) {
	repo := NewDuelRepository(nil)

	require.NotNil(t, repo)
}
