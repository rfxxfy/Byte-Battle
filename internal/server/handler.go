package server

import (
	"context"
	"encoding/json"
	"net/http"

	"bytebattle/internal/api"
	"bytebattle/internal/apierr"
	"bytebattle/internal/database/models"
)

func (s *HTTPServer) handleRoot(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("Добро пожаловать в Byte-Battle!"))
}

func (s *HTTPServer) handleHello(w http.ResponseWriter, r *http.Request) {
	user, err := s.users.GetOrCreateTestUser(r.Context())
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"message": "Привет, Byte-Battle!",
		"user":    user,
	})
}

func (s *HTTPServer) ListGames(ctx context.Context, req api.ListGamesRequestObject) (api.ListGamesResponseObject, error) {
	limit, offset := 10, 0
	if req.Params.Limit != nil {
		limit = *req.Params.Limit
	}
	if req.Params.Offset != nil {
		offset = *req.Params.Offset
	}

	games, total, err := s.gameService.ListGames(ctx, limit, offset)
	if err != nil {
		return nil, err
	}

	apiGames := make([]api.Game, len(games))
	for i, g := range games {
		apiGames[i] = toAPIGame(g)
	}

	return api.ListGames200JSONResponse{Games: apiGames, Total: total}, nil
}

func (s *HTTPServer) CreateGame(ctx context.Context, req api.CreateGameRequestObject) (api.CreateGameResponseObject, error) {
	game, err := s.gameService.CreateGame(ctx, req.Body.PlayerIds, req.Body.ProblemId)
	if err != nil {
		return nil, err
	}

	return api.CreateGame201JSONResponse{Game: toAPIGame(game)}, nil
}

func (s *HTTPServer) GetGame(ctx context.Context, req api.GetGameRequestObject) (api.GetGameResponseObject, error) {
	game, err := s.gameService.GetGame(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	return api.GetGame200JSONResponse{Game: toAPIGame(game)}, nil
}

func (s *HTTPServer) DeleteGame(ctx context.Context, req api.DeleteGameRequestObject) (api.DeleteGameResponseObject, error) {
	if err := s.gameService.DeleteGame(ctx, req.Id); err != nil {
		return nil, err
	}

	return api.DeleteGame200JSONResponse{Deleted: true}, nil
}

func (s *HTTPServer) StartGame(ctx context.Context, req api.StartGameRequestObject) (api.StartGameResponseObject, error) {
	game, err := s.gameService.StartGame(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	return api.StartGame200JSONResponse{Game: toAPIGame(game)}, nil
}

func (s *HTTPServer) CompleteGame(ctx context.Context, req api.CompleteGameRequestObject) (api.CompleteGameResponseObject, error) {
	if req.Body.WinnerId < 1 {
		return nil, apierr.New(apierr.ErrValidation, "invalid winner_id")
	}

	game, err := s.gameService.CompleteGame(ctx, req.Id, req.Body.WinnerId)
	if err != nil {
		return nil, err
	}

	return api.CompleteGame200JSONResponse{Game: toAPIGame(game)}, nil
}

func (s *HTTPServer) CancelGame(ctx context.Context, req api.CancelGameRequestObject) (api.CancelGameResponseObject, error) {
	game, err := s.gameService.CancelGame(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	return api.CancelGame200JSONResponse{Game: toAPIGame(game)}, nil
}

func (s *HTTPServer) CreateSession(ctx context.Context, req api.CreateSessionRequestObject) (api.CreateSessionResponseObject, error) {
	if req.Body.UserId < 1 {
		return nil, apierr.New(apierr.ErrValidation, "user_id must be a positive integer")
	}

	session, err := s.sessionService.CreateSession(ctx, req.Body.UserId)
	if err != nil {
		return nil, err
	}

	return api.CreateSession201JSONResponse{Session: toAPISession(session)}, nil
}

func (s *HTTPServer) GetSession(ctx context.Context, req api.GetSessionRequestObject) (api.GetSessionResponseObject, error) {
	session, err := s.sessionService.GetSession(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	return api.GetSession200JSONResponse{Session: toAPISession(session)}, nil
}

func (s *HTTPServer) ValidateSession(ctx context.Context, req api.ValidateSessionRequestObject) (api.ValidateSessionResponseObject, error) {
	token := ""
	if req.Params.Token != nil {
		token = *req.Params.Token
	}

	session, err := s.sessionService.ValidateToken(ctx, token)
	if err != nil {
		return nil, err
	}

	return api.ValidateSession200JSONResponse{Valid: true, Session: toAPISession(session)}, nil
}

func (s *HTTPServer) RefreshSession(ctx context.Context, req api.RefreshSessionRequestObject) (api.RefreshSessionResponseObject, error) {
	session, err := s.sessionService.RefreshSession(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	return api.RefreshSession200JSONResponse{Session: toAPISession(session)}, nil
}

func (s *HTTPServer) EndSession(ctx context.Context, req api.EndSessionRequestObject) (api.EndSessionResponseObject, error) {
	if err := s.sessionService.EndSession(ctx, req.Id); err != nil {
		return nil, err
	}

	return api.EndSession200JSONResponse{Ended: true}, nil
}

func (s *HTTPServer) GetUserSessions(ctx context.Context, req api.GetUserSessionsRequestObject) (api.GetUserSessionsResponseObject, error) {
	sessions, err := s.sessionService.GetUserSessions(ctx, req.UserId)
	if err != nil {
		return nil, err
	}

	apiSessions := make([]api.Session, len(sessions))
	for i, s := range sessions {
		apiSessions[i] = toAPISession(s)
	}

	return api.GetUserSessions200JSONResponse{Sessions: apiSessions}, nil
}

func (s *HTTPServer) EndAllUserSessions(ctx context.Context, req api.EndAllUserSessionsRequestObject) (api.EndAllUserSessionsResponseObject, error) {
	count, err := s.sessionService.EndAllUserSessions(ctx, req.UserId)
	if err != nil {
		return nil, err
	}

	return api.EndAllUserSessions200JSONResponse{Count: count}, nil
}

func (s *HTTPServer) CleanupExpiredSessions(ctx context.Context, _ api.CleanupExpiredSessionsRequestObject) (api.CleanupExpiredSessionsResponseObject, error) {
	count, err := s.sessionService.CleanupExpired(ctx)
	if err != nil {
		return nil, err
	}

	return api.CleanupExpiredSessions200JSONResponse{Count: count}, nil
}

func toAPIGame(g *models.Game) api.Game {
	result := api.Game{
		Id:        g.ID,
		ProblemId: g.ProblemID,
		Status:    api.GameStatus(g.Status),
		CreatedAt: g.CreatedAt,
		UpdatedAt: g.UpdatedAt,
	}
	if g.WinnerID.Valid {
		id := g.WinnerID.Int
		result.WinnerId = &id
	}
	return result
}

func toAPISession(s *models.Session) api.Session {
	return api.Session{
		Id:        s.ID,
		UserId:    s.UserID,
		Token:     s.Token,
		ExpiresAt: s.ExpiresAt,
		CreatedAt: s.CreatedAt,
	}
}
