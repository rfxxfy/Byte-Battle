package server

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

type CreateDuelRequest struct {
	PlayerIDs []int `json:"player_ids"`
	ProblemID int   `json:"problem_id"`
}

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
	var req CreateDuelRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request body"})
	}

	duel, err := s.duelService.CreateDuel(c.Request().Context(), req.PlayerIDs, req.ProblemID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
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
