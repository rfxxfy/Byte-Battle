package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGameStatus_Constants(t *testing.T) {
	tests := []struct {
		status   string
		expected string
	}{
		{GameStatusPending, "pending"},
		{GameStatusActive, "active"},
		{GameStatusFinished, "finished"},
		{GameStatusCancelled, "cancelled"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status)
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

func TestNewGameRepository(t *testing.T) {
	repo := NewGameRepository(nil)

	require.NotNil(t, repo)
}
