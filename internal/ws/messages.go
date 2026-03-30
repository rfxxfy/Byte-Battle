package ws

import "github.com/google/uuid"

const (
	TypeSubmit           = "submit"
	TypeSubmissionResult = "submission_result"
	TypePlayerAdvanced   = "player_advanced"
	TypePlayerState      = "player_state"
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
	Type       string           `json:"type"`
	UserID     uuid.UUID        `json:"user_id,omitempty"`
	WinnerID   uuid.UUID        `json:"winner_id,omitempty"`
	Accepted   bool             `json:"accepted"`
	Stdout     string           `json:"stdout,omitempty"`
	Stderr     string           `json:"stderr,omitempty"`
	Message    string           `json:"message,omitempty"`
	ErrorCode  string           `json:"error_code,omitempty"`
	FailedTest *int             `json:"failed_test,omitempty"`
	ProblemID  string           `json:"problem_id,omitempty"`
	ProblemIdx int              `json:"problem_index"`
	Code       string           `json:"code,omitempty"`
	Language   string           `json:"language,omitempty"`
	Progress   map[string]int32 `json:"progress,omitempty"`
}
