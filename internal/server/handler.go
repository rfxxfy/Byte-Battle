package server

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

func (s *HTTPServer) handleRoot(c echo.Context) error {
	return c.String(http.StatusOK, "Добро пожаловать в Byte-Battle!")
}

func (s *HTTPServer) handleHello(c echo.Context) error {
	user, err := s.users.GetOrCreateTestUser(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"status": "error",
			"error":  err.Error(),
		})
	}

	return c.JSON(http.StatusOK, echo.Map{
		"status":  "success",
		"message": "Привет, Byte-Battle!",
		"user":    user,
	})
}

func (s *HTTPServer) handleCreateDuel(c echo.Context) error {
	player1ID, _ := strconv.Atoi(c.FormValue("player1_id"))
	player2ID, _ := strconv.Atoi(c.FormValue("player2_id"))
	problemID, _ := strconv.Atoi(c.FormValue("problem_id"))

	duel, err := s.duelService.CreateDuel(c.Request().Context(), player1ID, player2ID, problemID)
	if err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}

	return c.String(http.StatusCreated, fmt.Sprintf("Duel created: ID=%d", duel.ID))
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
		return c.String(http.StatusBadRequest, err.Error())
	}

	return c.String(http.StatusOK, fmt.Sprintf("Duel started: %+v", duel))
}

func (s *HTTPServer) handleCompleteDuel(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	winnerID, _ := strconv.Atoi(c.FormValue("winner_id"))

	duel, err := s.duelService.CompleteDuel(c.Request().Context(), id, winnerID)
	if err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}

	return c.String(http.StatusOK, fmt.Sprintf("Duel completed: %+v", duel))
}

func (s *HTTPServer) handleDeleteDuel(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))

	err := s.duelService.DeleteDuel(c.Request().Context(), id)
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}

	return c.NoContent(http.StatusNoContent)
}
