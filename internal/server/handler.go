package server

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"bytebattle/internal/service"

	"github.com/labstack/echo/v4"
)

type CreateDuelRequest struct {
	PlayerIDs []int `json:"player_ids"`
	ProblemID int   `json:"problem_id"`
}

const (
	StatusSuccess = "success"
	StatusError   = "error"
)

type SuccessResponse struct {
	Status string `json:"status"`
}

type ErrorResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

func (s *HTTPServer) handleRoot(c echo.Context) error {
	return c.String(http.StatusOK, "Добро пожаловать в Byte-Battle!")
}

func (s *HTTPServer) handleHello(c echo.Context) error {
	user, err := s.users.GetOrCreateTestUser(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"status": StatusError,
			"error":  err.Error(),
		})
	}

	return c.JSON(http.StatusOK, echo.Map{
		"status":  StatusSuccess,
		"message": "Привет, Byte-Battle!",
		"user":    user,
	})
}

func (s *HTTPServer) handleCreateDuel(c echo.Context) error {
	var req CreateDuelRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request body"})
	}

	duel, err := s.duelService.CreateDuel(c.Request().Context(), req.PlayerIDs, req.ProblemID)
	if err != nil {
		if errors.Is(err, service.ErrNotEnoughPlayers) ||
			errors.Is(err, service.ErrDuplicatePlayers) {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusCreated, echo.Map{"id": duel.ID})
}

func (s *HTTPServer) handleGetDuel(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))

	duel, err := s.duelService.GetDuel(c.Request().Context(), id)
	if err != nil {
		return c.String(http.StatusNotFound, "Duel not found")
	}

	return c.String(http.StatusOK, fmt.Sprintf("Duel: %+v", duel))
}

func (s *HTTPServer) handleListDuels(c echo.Context) error {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	offset, _ := strconv.Atoi(c.QueryParam("offset"))

	duels, err := s.duelService.ListDuels(c.Request().Context(), limit, offset)
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}

	return c.String(http.StatusOK, fmt.Sprintf("Duels: %+v", duels))
}

func (s *HTTPServer) handleStartDuel(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))

	duel, err := s.duelService.StartDuel(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrDuelAlreadyStarted) {
			return c.String(http.StatusBadRequest, err.Error())
		}
		return c.String(http.StatusInternalServerError, err.Error())
	}

	return c.String(http.StatusOK, fmt.Sprintf("Duel started: %+v", duel))
}

func (s *HTTPServer) handleCompleteDuel(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	winnerID, _ := strconv.Atoi(c.FormValue("winner_id"))

	duel, err := s.duelService.CompleteDuel(c.Request().Context(), id, winnerID)
	if err != nil {
		if errors.Is(err, service.ErrDuelNotInProgress) ||
			errors.Is(err, service.ErrInvalidWinner) {
			return c.String(http.StatusBadRequest, err.Error())
		}
		return c.String(http.StatusInternalServerError, err.Error())
	}

	return c.String(http.StatusOK, fmt.Sprintf("Duel completed: %+v", duel))
}

func (s *HTTPServer) handleCancelDuel(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))

	duel, err := s.duelService.CancelDuel(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrCannotCancelCompleted) {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, echo.Map{
		"status": StatusSuccess,
		"duel":   duel,
	})
}

func (s *HTTPServer) handleDeleteDuel(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))

	err := s.duelService.DeleteDuel(c.Request().Context(), id)
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, SuccessResponse{
		Status: StatusSuccess,
	})
}

func (s *HTTPServer) handleCreateSession(c echo.Context) error {
	userID, err := strconv.Atoi(c.FormValue("user_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"status": StatusError,
			"error":  "invalid user_id",
		})
	}

	session, err := s.sessionService.CreateSession(c.Request().Context(), userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"status": StatusError,
			"error":  err.Error(),
		})
	}

	return c.JSON(http.StatusCreated, echo.Map{
		"status":  StatusSuccess,
		"session": session,
	})
}

func (s *HTTPServer) handleGetSession(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"status": StatusError,
			"error":  "invalid session id",
		})
	}

	session, err := s.sessionService.GetSession(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrSessionNotFound) {
			return c.JSON(http.StatusNotFound, echo.Map{
				"status": StatusError,
				"error":  err.Error(),
			})
		}
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"status": StatusError,
			"error":  err.Error(),
		})
	}

	return c.JSON(http.StatusOK, echo.Map{
		"status":  StatusSuccess,
		"session": session,
	})
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
			return c.JSON(http.StatusUnauthorized, echo.Map{
				"status": StatusError,
				"error":  err.Error(),
			})
		}
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"status": StatusError,
			"error":  err.Error(),
		})
	}

	return c.JSON(http.StatusOK, echo.Map{
		"status":  StatusSuccess,
		"valid":   true,
		"session": session,
	})
}

func (s *HTTPServer) handleGetUserSessions(c echo.Context) error {
	userID, err := strconv.Atoi(c.Param("user_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"status": StatusError,
			"error":  "invalid user_id",
		})
	}

	sessions, err := s.sessionService.GetUserSessions(c.Request().Context(), userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"status": StatusError,
			"error":  err.Error(),
		})
	}

	return c.JSON(http.StatusOK, echo.Map{
		"status":   StatusSuccess,
		"sessions": sessions,
	})
}

func (s *HTTPServer) handleRefreshSession(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"status": StatusError,
			"error":  "invalid session id",
		})
	}

	session, err := s.sessionService.RefreshSession(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrSessionNotFound) ||
			errors.Is(err, service.ErrSessionExpired) {
			return c.JSON(http.StatusNotFound, echo.Map{
				"status": StatusError,
				"error":  err.Error(),
			})
		}
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"status": StatusError,
			"error":  err.Error(),
		})
	}

	return c.JSON(http.StatusOK, echo.Map{
		"status":  StatusSuccess,
		"session": session,
	})
}

func (s *HTTPServer) handleEndSession(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"status": StatusError,
			"error":  "invalid session id",
		})
	}

	err = s.sessionService.EndSession(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrSessionNotFound) {
			return c.JSON(http.StatusNotFound, echo.Map{
				"status": StatusError,
				"error":  err.Error(),
			})
		}
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"status": StatusError,
			"error":  err.Error(),
		})
	}

	return c.JSON(http.StatusOK, SuccessResponse{
		Status: StatusSuccess,
	})
}

func (s *HTTPServer) handleEndAllUserSessions(c echo.Context) error {
	userID, err := strconv.Atoi(c.Param("user_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"status": StatusError,
			"error":  "invalid user_id",
		})
	}

	count, err := s.sessionService.EndAllUserSessions(c.Request().Context(), userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"status": StatusError,
			"error":  err.Error(),
		})
	}

	return c.JSON(http.StatusOK, echo.Map{
		"status":  StatusSuccess,
		"deleted": count,
	})
}

func (s *HTTPServer) handleCleanupExpiredSessions(c echo.Context) error {
	count, err := s.sessionService.CleanupExpired(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"status": StatusError,
			"error":  err.Error(),
		})
	}

	return c.JSON(http.StatusOK, echo.Map{
		"status":  StatusSuccess,
		"deleted": count,
	})
}
