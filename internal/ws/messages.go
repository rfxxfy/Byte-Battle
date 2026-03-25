package ws

import "github.com/google/uuid"

const (
	TypeSubmit           = "submit"
	TypeSubmissionResult = "submission_result"
	TypeGameFinished     = "game_finished"
	TypePlayerJoined     = "player_joined"
	TypeError            = "error"
)

type ClientMessage struct {
	Type     string `json:"type"`
	Code     string `json:"code,omitempty"`
	Language string `json:"language,omitempty"`
}

type ServerMessage struct {
	Type       string    `json:"type"`
	UserID     uuid.UUID `json:"user_id,omitempty"`
	WinnerID   uuid.UUID `json:"winner_id,omitempty"`
	Accepted   bool      `json:"accepted,omitempty"`
	Stdout     string    `json:"stdout,omitempty"`
	Stderr     string    `json:"stderr,omitempty"`
	Message    string    `json:"message,omitempty"`
	FailedTest *int      `json:"failed_test,omitempty"`
}
