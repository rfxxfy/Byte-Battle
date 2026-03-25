package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"bytebattle/internal/api"
	"bytebattle/internal/apierr"
	sqlcdb "bytebattle/internal/db/sqlc"
	"bytebattle/internal/executor"
	"bytebattle/internal/problems"
	"bytebattle/internal/ws"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	gorillaws "github.com/gorilla/websocket"
)

func (s *HTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.pool.Ping(r.Context()); err != nil {
		http.Error(w, "db unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (s *HTTPServer) handleRoot(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("Добро пожаловать в Byte-Battle!"))
}

func (s *HTTPServer) handleHello(w http.ResponseWriter, r *http.Request) {
	user, err := s.users.GetOrCreateTestUser(r.Context())
	if err != nil {
		responseErrorHandler(w, r, err)
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

	gameIDs := make([]int32, len(games))
	for i := range games {
		gameIDs[i] = games[i].ID
	}
	participantRows, err := s.gameService.GetParticipantIDsByGameIDs(ctx, gameIDs)
	if err != nil {
		return nil, err
	}
	participantMap := make(map[int32][]uuid.UUID, len(games))
	for _, row := range participantRows {
		participantMap[row.GameID] = append(participantMap[row.GameID], row.UserID)
	}

	apiGames := make([]api.Game, len(games))
	for i := range games {
		apiGames[i] = toAPIGame(games[i], participantMap[games[i].ID])
	}

	return api.ListGames200JSONResponse{Games: apiGames, Total: total}, nil
}

func (s *HTTPServer) ListProblems(_ context.Context, _ api.ListProblemsRequestObject) (api.ListProblemsResponseObject, error) {
	problemsList := s.problemService.ListProblems()

	apiProblems := make([]api.Problem, len(problemsList))
	for i := range problemsList {
		apiProblems[i] = toAPIProblem(problemsList[i])
	}

	return api.ListProblems200JSONResponse{Problems: apiProblems}, nil
}

func (s *HTTPServer) GetProblem(_ context.Context, req api.GetProblemRequestObject) (api.GetProblemResponseObject, error) {
	p, err := s.problemService.GetProblem(req.ProblemId)
	if err != nil {
		return nil, apierr.New(apierr.ErrProblemNotFound, "problem not found")
	}

	return api.GetProblem200JSONResponse{Problem: toAPIProblem(p)}, nil
}

func (s *HTTPServer) CreateGame(ctx context.Context, req api.CreateGameRequestObject) (api.CreateGameResponseObject, error) {
	userID, _ := userIDFromContext(ctx)
	game, err := s.gameService.CreateGame(ctx, userID, req.Body.ProblemId)
	if err != nil {
		return nil, err
	}

	participantIDs, err := s.gameService.GetParticipantIDs(ctx, int(game.ID))
	if err != nil {
		return nil, err
	}

	return api.CreateGame201JSONResponse{Game: toAPIGame(game, participantIDs)}, nil
}

func (s *HTTPServer) JoinGame(ctx context.Context, req api.JoinGameRequestObject) (api.JoinGameResponseObject, error) {
	userID, _ := userIDFromContext(ctx)
	game, err := s.gameService.JoinGame(ctx, req.Id, userID)
	if err != nil {
		return nil, err
	}

	participantIDs, err := s.gameService.GetParticipantIDs(ctx, int(game.ID))
	if err != nil {
		return nil, err
	}

	return api.JoinGame200JSONResponse{Game: toAPIGame(game, participantIDs)}, nil
}

func (s *HTTPServer) GetGame(ctx context.Context, req api.GetGameRequestObject) (api.GetGameResponseObject, error) {
	game, err := s.gameService.GetGame(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	participantIDs, err := s.gameService.GetParticipantIDs(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	return api.GetGame200JSONResponse{Game: toAPIGame(game, participantIDs)}, nil
}

func (s *HTTPServer) DeleteGame(ctx context.Context, req api.DeleteGameRequestObject) (api.DeleteGameResponseObject, error) {
	if err := s.gameService.DeleteGame(ctx, req.Id); err != nil {
		return nil, err
	}

	return api.DeleteGame200JSONResponse{Deleted: true}, nil
}

func (s *HTTPServer) StartGame(ctx context.Context, req api.StartGameRequestObject) (api.StartGameResponseObject, error) {
	userID, _ := userIDFromContext(ctx)
	game, err := s.gameService.StartGame(ctx, req.Id, userID)
	if err != nil {
		return nil, err
	}

	participantIDs, err := s.gameService.GetParticipantIDs(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	return api.StartGame200JSONResponse{Game: toAPIGame(game, participantIDs)}, nil
}

func (s *HTTPServer) CompleteGame(ctx context.Context, req api.CompleteGameRequestObject) (api.CompleteGameResponseObject, error) {
	game, err := s.gameService.CompleteGame(ctx, req.Id, req.Body.WinnerId)
	if err != nil {
		return nil, err
	}

	participantIDs, err := s.gameService.GetParticipantIDs(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	return api.CompleteGame200JSONResponse{Game: toAPIGame(game, participantIDs)}, nil
}

func (s *HTTPServer) CancelGame(ctx context.Context, req api.CancelGameRequestObject) (api.CancelGameResponseObject, error) {
	game, err := s.gameService.CancelGame(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	participantIDs, err := s.gameService.GetParticipantIDs(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	return api.CancelGame200JSONResponse{Game: toAPIGame(game, participantIDs)}, nil
}

func (s *HTTPServer) handleExecute(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		writeHTTPError(w, apierr.New(apierr.ErrInvalidToken, "missing or invalid Authorization header"))
		return
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if _, err := s.sessionService.ValidateToken(r.Context(), token); err != nil {
		writeHTTPError(w, err)
		return
	}

	var req struct {
		Code     string `json:"code"`
		Language string `json:"language"`
		Input    string `json:"input"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
		return
	}

	result, err := s.executionService.Execute(r.Context(), executor.ExecutionRequest{
		Code:     req.Code,
		Language: executor.Language(req.Language),
		Stdin:    req.Input,
	})
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func toAPIGame(g sqlcdb.Game, participantIDs []uuid.UUID) api.Game {
	result := api.Game{
		Id:             int(g.ID),
		ProblemId:      g.ProblemID,
		Status:         api.GameStatus(g.Status),
		ParticipantIds: participantIDs,
		CreatedAt:      g.CreatedAt.Time,
		UpdatedAt:      g.UpdatedAt.Time,
	}
	if g.WinnerID.Valid {
		id := g.WinnerID.UUID
		result.WinnerId = &id
	}
	return result
}

func toAPIProblem(p *problems.Problem) api.Problem {
	testCount := len(p.TestCases)
	return api.Problem{
		Id:            p.ID,
		Title:         p.Meta.Title,
		Description:   p.Meta.Description,
		Difficulty:    api.ProblemDifficulty(p.Meta.Difficulty),
		TimeLimitMs:   p.Meta.TimeLimitMs,
		MemoryLimitMb: p.Meta.MemoryLimitMb,
		TestCount:     &testCount,
	}
}

func (s *HTTPServer) handleGameWS(w http.ResponseWriter, r *http.Request) {
	gameID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid game id", http.StatusBadRequest)
		return
	}

	// Token is passed as the first Sec-WebSocket-Protocol value.
	// Browsers cannot set custom headers on WS connections, so this is
	// the standard workaround: new WebSocket(url, [token])
	token := ""
	if protocols := gorillaws.Subprotocols(r); len(protocols) > 0 {
		token = protocols[0]
	}

	session, err := s.sessionService.ValidateToken(r.Context(), token)
	if err != nil {
		writeHTTPError(w, err)
		return
	}

	game, err := s.gameService.GetGame(r.Context(), gameID)
	if err != nil {
		http.Error(w, "game not found", http.StatusNotFound)
		return
	}
	if game.Status != "active" {
		http.Error(w, "game is not active", http.StatusBadRequest)
		return
	}

	// Echo the subprotocol back — required by browser WS spec
	conn, err := upgrader.Upgrade(w, r, http.Header{
		"Sec-WebSocket-Protocol": {token},
	})
	if err != nil {
		log.Printf("ws upgrade error: %v", err)
		return
	}

	client := ws.NewClient(conn)
	s.hub.Join(int32(gameID), client)
	defer s.hub.Leave(int32(gameID), client)
	defer client.Close() // signals WritePump to exit cleanly

	go client.WritePump()

	conn.SetReadLimit(32 * 1024)
	conn.SetReadDeadline(time.Now().Add(ws.PongWait)) //nolint:errcheck // not actionable
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(ws.PongWait))
	})

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			if gorillaws.IsUnexpectedCloseError(err, gorillaws.CloseGoingAway, gorillaws.CloseNormalClosure) {
				log.Printf("ws read error: %v", err)
			}
			return
		}

		var msg ws.ClientMessage
		if err := json.Unmarshal(data, &msg); err != nil || msg.Type != ws.TypeSubmit {
			continue
		}

		s.processSubmit(r.Context(), int32(gameID), session.UserID, msg)
	}
}

func (s *HTTPServer) processSubmit(ctx context.Context, gameID int32, userID uuid.UUID, msg ws.ClientMessage) {
	result, execErr := s.executionService.Execute(ctx, executor.ExecutionRequest{
		Code:     msg.Code,
		Language: executor.Language(msg.Language),
		Stdin:    msg.Input,
	})

	if execErr != nil {
		log.Print("executor error during submission")
	}
	accepted := execErr == nil && result.ExitCode == 0

	resultMsg, _ := json.Marshal(ws.ServerMessage{
		Type:     ws.TypeSubmissionResult,
		UserID:   userID,
		Accepted: accepted,
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
	})
	s.hub.Broadcast(gameID, resultMsg)

	if !accepted {
		return
	}

	completed, err := s.gameService.CompleteGame(ctx, int(gameID), userID)
	if err != nil {
		// another player already won — ignore
		return
	}

	var winnerID uuid.UUID
	if completed.WinnerID.Valid {
		winnerID = completed.WinnerID.UUID
	}
	finMsg, _ := json.Marshal(ws.ServerMessage{
		Type:     ws.TypeGameFinished,
		WinnerID: winnerID,
	})
	s.hub.Broadcast(gameID, finMsg)
}
