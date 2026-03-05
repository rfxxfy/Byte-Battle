package apierr

import "net/http"

const (
	ErrInternal = "INTERNAL_ERROR"

	ErrGameNotFound         = "GAME_NOT_FOUND"
	ErrNotEnoughPlayers     = "NOT_ENOUGH_PLAYERS"
	ErrDuplicatePlayers     = "DUPLICATE_PLAYERS"
	ErrGameAlreadyStarted   = "GAME_ALREADY_STARTED"
	ErrGameNotInProgress    = "GAME_NOT_IN_PROGRESS"
	ErrInvalidWinner        = "INVALID_WINNER"
	ErrCannotCancelFinished = "CANNOT_CANCEL_FINISHED"
	ErrGameAlreadyCancelled = "GAME_ALREADY_CANCELLED"

	ErrSessionNotFound = "SESSION_NOT_FOUND"
	ErrSessionExpired  = "SESSION_EXPIRED"
	ErrInvalidToken    = "INVALID_TOKEN"

	ErrValidation = "VALIDATION_ERROR"
)

type AppError struct {
	ErrorCode  string `json:"error_code"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"-"`
}

func (e *AppError) Error() string { return e.Message }

func HTTPStatusCode(code string) int {
	switch code {
	case ErrValidation, ErrNotEnoughPlayers, ErrDuplicatePlayers,
		ErrGameAlreadyStarted, ErrGameNotInProgress, ErrInvalidWinner,
		ErrCannotCancelFinished, ErrGameAlreadyCancelled:
		return http.StatusBadRequest
	case ErrInvalidToken, ErrSessionExpired:
		return http.StatusUnauthorized
	case ErrGameNotFound, ErrSessionNotFound:
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}

func New(code, message string) *AppError {
	return &AppError{
		ErrorCode:  code,
		Message:    message,
		HTTPStatus: HTTPStatusCode(code),
	}
}
