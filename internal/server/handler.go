package server

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"bytebattle/internal/api"
	"bytebattle/internal/apierr"
	sqlcdb "bytebattle/internal/db/sqlc"
	"bytebattle/internal/executor"
	"bytebattle/internal/problems"
	"bytebattle/internal/service"
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
	if !s.executionService.IsReady() {
		http.Error(w, "executor not ready", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (s *HTTPServer) handleRoot(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("Добро пожаловать в Byte-Battle!"))
}

func (s *HTTPServer) ListGames(ctx context.Context, req api.ListGamesRequestObject) (api.ListGamesResponseObject, error) {
	userID, _ := userIDFromContext(ctx)
	limit, offset := 10, 0
	if req.Params.Limit != nil {
		limit = *req.Params.Limit
	}
	if req.Params.Offset != nil {
		offset = *req.Params.Offset
	}

	games, total, err := s.gameService.ListGames(ctx, limit, offset, userID)
	if err != nil {
		return nil, err
	}

	gameIDs := make([]int32, len(games))
	for i := range games {
		gameIDs[i] = games[i].ID
	}
	participantMap, err := s.gameService.GetParticipantsByGameIDs(ctx, gameIDs)
	if err != nil {
		return nil, err
	}
	problemIDsMap, err := s.gameService.GetGameProblemIDsByGameIDs(ctx, gameIDs)
	if err != nil {
		return nil, err
	}

	apiGames := make([]api.Game, len(games))
	for i := range games {
		showToken := games[i].IsPublic || isParticipantOf(userID, participantMap[games[i].ID])
		apiGames[i] = toAPIGame(games[i], participantMap[games[i].ID], problemIDsMap[games[i].ID], showToken)
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

	isPublic := req.Body.IsPublic == nil || *req.Body.IsPublic
	isSolo := req.Body.IsSolo != nil && *req.Body.IsSolo
	if isSolo {
		isPublic = false
	}
	var timeLimitMinutes *int16
	if req.Body.TimeLimitMinutes != nil {
		v := int16(*req.Body.TimeLimitMinutes)
		timeLimitMinutes = &v
	}
	game, err := s.gameService.CreateGame(ctx, userID, req.Body.ProblemIds, isPublic, isSolo, timeLimitMinutes)
	if err != nil {
		return nil, err
	}

	g, err := s.enrichGame(ctx, game, true)
	if err != nil {
		return nil, err
	}
	return api.CreateGame201JSONResponse{Game: g}, nil
}

func (s *HTTPServer) GetGame(ctx context.Context, req api.GetGameRequestObject) (api.GetGameResponseObject, error) {
	userID, _ := userIDFromContext(ctx)
	game, err := s.gameService.GetGame(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	if err := s.gameService.CanAccessGame(ctx, game, userID); err != nil {
		return nil, err
	}

	g, err := s.enrichGame(ctx, game, true)
	if err != nil {
		return nil, err
	}
	return api.GetGame200JSONResponse{Game: g}, nil
}

func (s *HTTPServer) DeleteGame(ctx context.Context, req api.DeleteGameRequestObject) (api.DeleteGameResponseObject, error) {
	userID, _ := userIDFromContext(ctx)
	if err := s.gameService.DeleteGame(ctx, req.Id, userID); err != nil {
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

	g, err := s.enrichGame(ctx, game, true)
	if err != nil {
		return nil, err
	}
	return api.StartGame200JSONResponse{Game: g}, nil
}

func (s *HTTPServer) CompleteGame(ctx context.Context, req api.CompleteGameRequestObject) (api.CompleteGameResponseObject, error) {
	userID, _ := userIDFromContext(ctx)
	game, err := s.gameService.CompleteGame(ctx, req.Id, userID, req.Body.WinnerId)
	if err != nil {
		return nil, err
	}

	g, err := s.enrichGame(ctx, game, true)
	if err != nil {
		return nil, err
	}
	return api.CompleteGame200JSONResponse{Game: g}, nil
}

func (s *HTTPServer) LeaveGame(ctx context.Context, req api.LeaveGameRequestObject) (api.LeaveGameResponseObject, error) {
	userID, _ := userIDFromContext(ctx)
	game, err := s.gameService.LeaveGame(ctx, req.Id, userID)
	if err != nil {
		return nil, err
	}

	g, err := s.enrichGame(ctx, game, game.IsPublic)
	if err != nil {
		return nil, err
	}
	return api.LeaveGame200JSONResponse{Game: g}, nil
}

func (s *HTTPServer) CancelGame(ctx context.Context, req api.CancelGameRequestObject) (api.CancelGameResponseObject, error) {
	userID, _ := userIDFromContext(ctx)
	game, err := s.gameService.CancelGame(ctx, req.Id, userID)
	if err != nil {
		return nil, err
	}

	g, err := s.enrichGame(ctx, game, true)
	if err != nil {
		return nil, err
	}
	return api.CancelGame200JSONResponse{Game: g}, nil
}

func (s *HTTPServer) PostExecute(ctx context.Context, request api.PostExecuteRequestObject) (api.PostExecuteResponseObject, error) {
	userID, _ := userIDFromContext(ctx)
	if !s.executionService.TryAcquireSlot(userID) {
		return nil, apierr.New(apierr.ErrExecutionInProgress, "execution already in progress")
	}
	defer s.executionService.ReleaseSlot(userID)
	if err := s.executionService.CheckRateLimit(userID); err != nil {
		return nil, err
	}
	result, err := s.executionService.Execute(ctx, executor.ExecutionRequest{
		Code:     request.Body.Code,
		Language: executor.Language(request.Body.Language),
		Stdin:    request.Body.Input,
	})
	if err != nil {
		return nil, err
	}
	return api.PostExecute200JSONResponse{
		Stdout:     result.Stdout,
		Stderr:     result.Stderr,
		ExitCode:   result.ExitCode,
		TimeUsedMs: int(result.TimeUsed.Milliseconds()),
	}, nil
}

func (s *HTTPServer) GetGameSolutions(ctx context.Context, req api.GetGameSolutionsRequestObject) (api.GetGameSolutionsResponseObject, error) {
	userID, _ := userIDFromContext(ctx)
	game, err := s.gameService.GetGame(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	if err := s.gameService.CanAccessGame(ctx, game, userID); err != nil {
		return nil, err
	}

	rows, err := s.gameService.GetGameSolutions(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	solutions := make([]api.GameSolution, len(rows))
	for i := range rows {
		r := &rows[i]
		var name *string
		if r.Name.Valid {
			name = &r.Name.String
		}
		solutions[i] = api.GameSolution{
			UserId:    r.UserID,
			Name:      name,
			ProblemId: r.ProblemID,
			Code:      r.Code,
			Language:  r.Language,
			SolvedAt:  r.CreatedAt.Time,
		}
	}

	return api.GetGameSolutions200JSONResponse{Solutions: solutions}, nil
}

func (s *HTTPServer) GetGameByToken(ctx context.Context, req api.GetGameByTokenRequestObject) (api.GetGameByTokenResponseObject, error) {
	game, err := s.gameService.GetGameByToken(ctx, req.InviteToken)
	if err != nil {
		return nil, err
	}
	g, err := s.enrichGame(ctx, game, game.IsPublic)
	if err != nil {
		return nil, err
	}
	return api.GetGameByToken200JSONResponse{Game: g}, nil
}

func (s *HTTPServer) JoinGameByToken(ctx context.Context, req api.JoinGameByTokenRequestObject) (api.JoinGameByTokenResponseObject, error) {
	userID, _ := userIDFromContext(ctx)
	game, err := s.gameService.JoinGameByToken(ctx, req.InviteToken, userID)
	if err != nil {
		return nil, err
	}
	g, err := s.enrichGame(ctx, game, true)
	if err != nil {
		return nil, err
	}
	return api.JoinGameByToken200JSONResponse{Game: g}, nil
}

func (s *HTTPServer) enrichGame(ctx context.Context, game sqlcdb.Game, showToken bool) (api.Game, error) {
	participants, err := s.gameService.GetParticipants(ctx, int(game.ID))
	if err != nil {
		return api.Game{}, err
	}
	problemIDs, err := s.gameService.GetGameProblemIDs(ctx, game.ID)
	if err != nil {
		return api.Game{}, err
	}
	return toAPIGame(game, participants, problemIDs, showToken), nil
}

func toAPIGame(g sqlcdb.Game, participants []service.Participant, problemIDs []string, showToken bool) api.Game {
	apiParticipants := make([]api.GameParticipant, len(participants))
	for i, p := range participants {
		apiParticipants[i] = api.GameParticipant{Id: p.ID, Name: p.Name}
	}
	result := api.Game{
		Id:           int(g.ID),
		IsPublic:     g.IsPublic,
		IsSolo:       g.IsSolo,
		ProblemIds:   problemIDs,
		CreatorId:    g.CreatorID,
		Status:       api.GameStatus(g.Status),
		Participants: apiParticipants,
		CreatedAt:    g.CreatedAt.Time,
		UpdatedAt:    g.UpdatedAt.Time,
	}
	if g.StartedAt.Valid {
		t := g.StartedAt.Time
		result.StartedAt = &t
	}
	if g.WinnerID.Valid {
		id := g.WinnerID.UUID
		result.WinnerId = &id
	}
	if showToken {
		result.InviteToken = &g.InviteToken
	}
	if g.TimeLimitMinutes.Valid {
		v := int(g.TimeLimitMinutes.Int16)
		result.TimeLimitMinutes = &v
	}
	return result
}

func (s *HTTPServer) TimeoutGame(ctx context.Context, req api.TimeoutGameRequestObject) (api.TimeoutGameResponseObject, error) {
	userID, _ := userIDFromContext(ctx)
	game, err := s.gameService.TimeoutGame(ctx, req.Id, userID)
	if err != nil {
		return nil, err
	}
	g, err := s.enrichGame(ctx, game, false)
	if err != nil {
		return nil, err
	}
	return api.TimeoutGame200JSONResponse{Game: g}, nil
}

func isParticipantOf(userID uuid.UUID, participants []service.Participant) bool {
	for _, p := range participants {
		if p.ID == userID {
			return true
		}
	}
	return false
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

	ok, err := s.gameService.IsParticipant(r.Context(), gameID, session.UserID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !ok {
		writeHTTPError(w, apierr.New(apierr.ErrNotParticipant, "not a participant"))
		return
	}

	// Echo the subprotocol back, required by browser WS spec
	conn, err := upgrader.Upgrade(w, r, http.Header{
		"Sec-WebSocket-Protocol": {token},
	})
	if err != nil {
		log.Printf("ws upgrade error: %v", err)
		return
	}

	client := ws.NewClient(conn, session.UserID)
	s.hub.Join(int32(gameID), client)
	defer s.hub.Leave(int32(gameID), client)
	defer client.Close() // signals WritePump to exit cleanly

	go client.WritePump()

	joinedMsg, _ := json.Marshal(ws.ServerMessage{
		Type:   ws.TypePlayerJoined,
		UserID: session.UserID,
	})
	s.hub.Broadcast(int32(gameID), joinedMsg)

	// Send initial problem state to the connecting player.
	s.sendPlayerState(r.Context(), gameID, session.UserID, client)

	connCtx, connCancel := context.WithCancel(r.Context())
	defer connCancel()

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

		go s.processSubmit(connCtx, int32(gameID), session.UserID, msg)
	}
}

func (s *HTTPServer) processSubmit(ctx context.Context, gameID int32, userID uuid.UUID, msg ws.ClientMessage) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	result, err := s.submissionService.Submit(ctx, int(gameID), userID, msg.Code, executor.Language(msg.Language))
	if err != nil {
		log.Printf("processSubmit: %v", err)
		var appErr *apierr.AppError
		if errors.As(err, &appErr) {
			s.broadcastError(gameID, userID, appErr.Message)
		} else {
			s.broadcastError(gameID, userID, "internal error")
		}
		return
	}

	if result.AlreadyAdvanced {
		return
	}

	resultMsg, _ := json.Marshal(ws.ServerMessage{
		Type:       ws.TypeSubmissionResult,
		UserID:     userID,
		Accepted:   result.Accepted,
		Stdout:     result.Stdout,
		Stderr:     result.Stderr,
		FailedTest: result.FailedTest,
	})
	s.hub.SendToUser(gameID, userID, resultMsg)

	if !result.Accepted {
		return
	}

	if result.WinnerID != uuid.Nil {
		finMsg, _ := json.Marshal(ws.ServerMessage{
			Type:     ws.TypeGameFinished,
			WinnerID: result.WinnerID,
		})
		s.hub.Broadcast(gameID, finMsg)
		return
	}

	if result.ProblemID != "" {
		progress, err := s.gameService.GetAllParticipantsProblemIndices(ctx, int(gameID))
		if err != nil {
			log.Printf("processSubmit: get progress: %v", err)
		}
		advMsg, _ := json.Marshal(ws.ServerMessage{
			Type:       ws.TypePlayerAdvanced,
			UserID:     userID,
			ProblemID:  result.ProblemID,
			ProblemIdx: result.ProblemIdx,
			Progress:   progress,
		})
		s.hub.Broadcast(gameID, advMsg)
	}
}

func (s *HTTPServer) sendPlayerState(ctx context.Context, gameID int, userID uuid.UUID, client *ws.Client) {
	playerIdx, err := s.gameService.GetParticipantProblemIndex(ctx, gameID, userID)
	if err != nil {
		return
	}
	problemID, err := s.gameService.GetGameProblemIDByIndex(ctx, int32(gameID), playerIdx)
	if err != nil {
		return
	}
	progress, _ := s.gameService.GetAllParticipantsProblemIndices(ctx, gameID)
	stateMsg, _ := json.Marshal(ws.ServerMessage{
		Type:       ws.TypePlayerState,
		ProblemID:  problemID,
		ProblemIdx: int(playerIdx),
		Progress:   progress,
	})
	client.Send(stateMsg)
}

func (s *HTTPServer) broadcastError(gameID int32, userID uuid.UUID, msg string) {
	errMsg, _ := json.Marshal(ws.ServerMessage{
		Type:    ws.TypeError,
		UserID:  userID,
		Message: msg,
	})
	s.hub.SendToUser(gameID, userID, errMsg)
}
