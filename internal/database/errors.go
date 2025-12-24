package database

import "errors"

var (
	ErrNotFound             = errors.New("record not found")
	ErrGameNotPending       = errors.New("game is not pending")
	ErrGameNotActive        = errors.New("game is not active")
	ErrGameFinished         = errors.New("game is already finished")
	ErrGameAlreadyCancelled = errors.New("game is already cancelled")
	ErrNotParticipant       = errors.New("user is not a participant")
)
