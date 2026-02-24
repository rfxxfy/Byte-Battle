package server

import (
	"errors"
	"net/http"
	"strconv"

	"bytebattle/internal/service"

	"github.com/labstack/echo/v4"
)

type CreateGameRequest struct {
	PlayerIDs []int `json:"player_ids"`
	ProblemID int   `json:"problem_id"`
}

type CompleteGameRequest struct {
	WinnerID int `json:"winner_id"`
}

type CreateSessionRequest struct {
	UserID int `json:"user_id"`
}

func jsonError(c echo.Context, code int, err error) error {
	return c.JSON(code, echo.Map{"error": err.Error()})
}

func jsonErrorMsg(c echo.Context, code int, msg string) error {
	return c.JSON(code, echo.Map{"error": msg})
}

func (s *HTTPServer) handleRoot(c echo.Context) error {
	return c.String(http.StatusOK, "Добро пожаловать в Byte-Battle!")
}

func (s *HTTPServer) handleHello(c echo.Context) error {
	user, err := s.users.GetOrCreateTestUser(c.Request().Context())
	if err != nil {
		return jsonError(c, http.StatusInternalServerError, err)
	}

	return c.JSON(http.StatusOK, echo.Map{
		"message": "Привет, Byte-Battle!",
		"user":    user,
	})
}

func (s *HTTPServer) handleCreateGame(c echo.Context) error {
	var req CreateGameRequest
	if err := c.Bind(&req); err != nil {
		return jsonErrorMsg(c, http.StatusBadRequest, "invalid request body")
	}

	game, err := s.gameService.CreateGame(c.Request().Context(), req.PlayerIDs, req.ProblemID)
	if err != nil {
		if errors.Is(err, service.ErrNotEnoughPlayers) ||
			errors.Is(err, service.ErrTooManyPlayers) ||
			errors.Is(err, service.ErrDuplicatePlayers) {
			return jsonError(c, http.StatusBadRequest, err)
		}
		return jsonError(c, http.StatusInternalServerError, err)
	}

	return c.JSON(http.StatusCreated, echo.Map{"game": game})
}

func (s *HTTPServer) handleGetGame(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return jsonErrorMsg(c, http.StatusBadRequest, "invalid game id")
	}

	game, err := s.gameService.GetGame(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrGameNotFound) {
			return jsonError(c, http.StatusNotFound, err)
		}
		return jsonError(c, http.StatusInternalServerError, err)
	}

	return c.JSON(http.StatusOK, echo.Map{"game": game})
}

func (s *HTTPServer) handleListGames(c echo.Context) error {
	var limit, offset int
	if raw := c.QueryParam("limit"); raw != "" {
		var err error
		limit, err = strconv.Atoi(raw)
		if err != nil {
			return jsonErrorMsg(c, http.StatusBadRequest, "invalid limit")
		}
	}
	if raw := c.QueryParam("offset"); raw != "" {
		var err error
		offset, err = strconv.Atoi(raw)
		if err != nil {
			return jsonErrorMsg(c, http.StatusBadRequest, "invalid offset")
		}
	}

	games, err := s.gameService.ListGames(c.Request().Context(), limit, offset)
	if err != nil {
		return jsonError(c, http.StatusInternalServerError, err)
	}

	return c.JSON(http.StatusOK, echo.Map{"games": games})
}

func (s *HTTPServer) handleStartGame(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return jsonErrorMsg(c, http.StatusBadRequest, "invalid game id")
	}

	game, err := s.gameService.StartGame(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrGameNotFound) {
			return jsonError(c, http.StatusNotFound, err)
		}
		if errors.Is(err, service.ErrGameAlreadyStarted) {
			return jsonError(c, http.StatusBadRequest, err)
		}
		return jsonError(c, http.StatusInternalServerError, err)
	}

	return c.JSON(http.StatusOK, echo.Map{"game": game})
}

func (s *HTTPServer) handleCompleteGame(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return jsonErrorMsg(c, http.StatusBadRequest, "invalid game id")
	}

	var req CompleteGameRequest
	if err := c.Bind(&req); err != nil {
		return jsonErrorMsg(c, http.StatusBadRequest, "invalid request body")
	}
	if req.WinnerID < 1 {
		return jsonErrorMsg(c, http.StatusBadRequest, "invalid winner_id")
	}

	game, err := s.gameService.CompleteGame(c.Request().Context(), id, req.WinnerID)
	if err != nil {
		if errors.Is(err, service.ErrGameNotFound) {
			return jsonError(c, http.StatusNotFound, err)
		}
		if errors.Is(err, service.ErrGameNotInProgress) ||
			errors.Is(err, service.ErrInvalidWinner) {
			return jsonError(c, http.StatusBadRequest, err)
		}
		return jsonError(c, http.StatusInternalServerError, err)
	}

	return c.JSON(http.StatusOK, echo.Map{"game": game})
}

func (s *HTTPServer) handleCancelGame(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return jsonErrorMsg(c, http.StatusBadRequest, "invalid game id")
	}

	game, err := s.gameService.CancelGame(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrGameNotFound) {
			return jsonError(c, http.StatusNotFound, err)
		}
		if errors.Is(err, service.ErrCannotCancelFinished) ||
			errors.Is(err, service.ErrGameAlreadyCancelled) {
			return jsonError(c, http.StatusBadRequest, err)
		}
		return jsonError(c, http.StatusInternalServerError, err)
	}

	return c.JSON(http.StatusOK, echo.Map{"game": game})
}

func (s *HTTPServer) handleDeleteGame(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return jsonErrorMsg(c, http.StatusBadRequest, "invalid game id")
	}

	err = s.gameService.DeleteGame(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrGameNotFound) {
			return jsonError(c, http.StatusNotFound, err)
		}
		return jsonError(c, http.StatusInternalServerError, err)
	}

	return c.JSON(http.StatusOK, echo.Map{"deleted": true})
}

func (s *HTTPServer) handleCreateSession(c echo.Context) error {
	var req CreateSessionRequest
	if err := c.Bind(&req); err != nil {
		return jsonErrorMsg(c, http.StatusBadRequest, "invalid request body")
	}
	if req.UserID == 0 {
		return jsonErrorMsg(c, http.StatusBadRequest, "user_id is required")
	}
	if req.UserID < 0 {
		return jsonErrorMsg(c, http.StatusBadRequest, "invalid user_id")
	}

	session, err := s.sessionService.CreateSession(c.Request().Context(), req.UserID)
	if err != nil {
		return jsonError(c, http.StatusInternalServerError, err)
	}

	return c.JSON(http.StatusCreated, echo.Map{"session": session})
}

func (s *HTTPServer) handleGetSession(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return jsonErrorMsg(c, http.StatusBadRequest, "invalid session id")
	}

	session, err := s.sessionService.GetSession(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrSessionNotFound) {
			return jsonError(c, http.StatusNotFound, err)
		}
		return jsonError(c, http.StatusInternalServerError, err)
	}

	return c.JSON(http.StatusOK, echo.Map{"session": session})
}

func (s *HTTPServer) handleValidateSession(c echo.Context) error {
	token := c.Request().Header.Get("Authorization")
	if token == "" {
		token = c.QueryParam("token")
	}

	session, err := s.sessionService.ValidateToken(c.Request().Context(), token)
	if err != nil {
		if errors.Is(err, service.ErrInvalidToken) ||
			errors.Is(err, service.ErrSessionNotFound) ||
			errors.Is(err, service.ErrSessionExpired) {
			return jsonError(c, http.StatusUnauthorized, err)
		}
		return jsonError(c, http.StatusInternalServerError, err)
	}

	return c.JSON(http.StatusOK, echo.Map{
		"valid":   true,
		"session": session,
	})
}

func (s *HTTPServer) handleGetUserSessions(c echo.Context) error {
	userID, err := strconv.Atoi(c.Param("user_id"))
	if err != nil {
		return jsonErrorMsg(c, http.StatusBadRequest, "invalid user_id")
	}

	sessions, err := s.sessionService.GetUserSessions(c.Request().Context(), userID)
	if err != nil {
		return jsonError(c, http.StatusInternalServerError, err)
	}

	return c.JSON(http.StatusOK, echo.Map{"sessions": sessions})
}

func (s *HTTPServer) handleRefreshSession(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return jsonErrorMsg(c, http.StatusBadRequest, "invalid session id")
	}

	session, err := s.sessionService.RefreshSession(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrSessionExpired) {
			return jsonError(c, http.StatusUnauthorized, err)
		}
		if errors.Is(err, service.ErrSessionNotFound) {
			return jsonError(c, http.StatusNotFound, err)
		}
		return jsonError(c, http.StatusInternalServerError, err)
	}

	return c.JSON(http.StatusOK, echo.Map{"session": session})
}

func (s *HTTPServer) handleEndSession(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return jsonErrorMsg(c, http.StatusBadRequest, "invalid session id")
	}

	err = s.sessionService.EndSession(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrSessionNotFound) {
			return jsonError(c, http.StatusNotFound, err)
		}
		return jsonError(c, http.StatusInternalServerError, err)
	}

	return c.JSON(http.StatusOK, echo.Map{"ended": true})
}

func (s *HTTPServer) handleEndAllUserSessions(c echo.Context) error {
	userID, err := strconv.Atoi(c.Param("user_id"))
	if err != nil {
		return jsonErrorMsg(c, http.StatusBadRequest, "invalid user_id")
	}

	count, err := s.sessionService.EndAllUserSessions(c.Request().Context(), userID)
	if err != nil {
		return jsonError(c, http.StatusInternalServerError, err)
	}

	return c.JSON(http.StatusOK, echo.Map{"count": count})
}

func (s *HTTPServer) handleCleanupExpiredSessions(c echo.Context) error {
	count, err := s.sessionService.CleanupExpired(c.Request().Context())
	if err != nil {
		return jsonError(c, http.StatusInternalServerError, err)
	}

	return c.JSON(http.StatusOK, echo.Map{"count": count})
}
