package apierr

import "net/http"

const (
	ErrInternal = "INTERNAL_ERROR"

	ErrGameNotFound             = "GAME_NOT_FOUND"
	ErrNotEnoughPlayers         = "NOT_ENOUGH_PLAYERS"
	ErrAlreadyParticipant       = "ALREADY_PARTICIPANT"
	ErrGameAlreadyStarted       = "GAME_ALREADY_STARTED"
	ErrGameNotInProgress        = "GAME_NOT_IN_PROGRESS"
	ErrInvalidWinner            = "INVALID_WINNER"
	ErrNotGameCreator           = "NOT_GAME_CREATOR"
	ErrCannotCancelFinishedGame = "CANNOT_CANCEL_FINISHED_GAME"
	ErrGameAlreadyCancelled     = "GAME_ALREADY_CANCELLED"
	ErrNotParticipant           = "NOT_PARTICIPANT"
	ErrCreatorCannotLeave       = "CREATOR_CANNOT_LEAVE"

	ErrSessionNotFound = "SESSION_NOT_FOUND"
	ErrSessionExpired  = "SESSION_EXPIRED"
	ErrInvalidToken    = "INVALID_TOKEN"
	ErrProblemNotFound = "PROBLEM_NOT_FOUND"

	ErrInvalidEmail         = "INVALID_EMAIL"
	ErrInvalidCode          = "INVALID_CODE"
	ErrTooManyAttempts      = "TOO_MANY_ATTEMPTS"
	ErrCodeRecentlySent     = "CODE_RECENTLY_SENT"
	ErrExecutionRateLimited = "EXECUTION_RATE_LIMITED"
	ErrExecutionInProgress  = "EXECUTION_IN_PROGRESS"
	ErrRoundAlreadyAdvanced = "ROUND_ALREADY_ADVANCED"
	ErrUserNotFound         = "USER_NOT_FOUND"

	ErrValidation = "VALIDATION_ERROR"
)

type AppError struct {
	ErrorCode  string `json:"error_code"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"-"`
}

func (e *AppError) Error() string { return e.Message }

func httpStatusCode(code string) int {
	switch code {
	case ErrValidation, ErrNotEnoughPlayers, ErrInvalidWinner:
		return http.StatusBadRequest
	case ErrNotGameCreator, ErrCreatorCannotLeave:
		return http.StatusForbidden
	case ErrAlreadyParticipant, ErrGameAlreadyStarted, ErrGameNotInProgress,
		ErrCannotCancelFinishedGame, ErrGameAlreadyCancelled, ErrRoundAlreadyAdvanced:
		return http.StatusConflict
	case ErrInvalidToken, ErrSessionExpired:
		return http.StatusUnauthorized
	case ErrGameNotFound, ErrSessionNotFound, ErrProblemNotFound, ErrUserNotFound, ErrNotParticipant:
		return http.StatusNotFound
	case ErrTooManyAttempts, ErrCodeRecentlySent, ErrExecutionRateLimited, ErrExecutionInProgress:
		return http.StatusTooManyRequests
	case ErrInvalidEmail, ErrInvalidCode:
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func New(code, message string) *AppError {
	return &AppError{
		ErrorCode:  code,
		Message:    message,
		HTTPStatus: httpStatusCode(code),
	}
}
