package database

import (
	"testing"
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
			if string(tt.status) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, string(tt.status))
			}
		})
	}
}

func TestPlayer_Struct(t *testing.T) {
	player := Player{ID: 42}

	if player.ID != 42 {
		t.Errorf("expected ID 42, got %d", player.ID)
	}
}

func TestPlayer_ZeroValue(t *testing.T) {
	var player Player

	if player.ID != 0 {
		t.Errorf("expected ID 0, got %d", player.ID)
	}
}

func TestNewDuelRepository(t *testing.T) {
	repo := NewDuelRepository(nil)

	if repo == nil {
		t.Fatal("expected repository, got nil")
	}
}
