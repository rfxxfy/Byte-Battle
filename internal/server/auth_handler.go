package server

import (
	"context"
	"errors"

	"bytebattle/internal/api"
	"bytebattle/internal/apierr"
	"bytebattle/internal/service"
)

func (s *HTTPServer) PostAuthEnter(ctx context.Context, req api.PostAuthEnterRequestObject) (api.PostAuthEnterResponseObject, error) {
	if err := s.entrance.SendCode(ctx, req.Body.Email); err != nil {
		return nil, entranceAppErr(err)
	}
	return api.PostAuthEnter200JSONResponse{Status: "ok"}, nil
}

func (s *HTTPServer) PostAuthConfirm(ctx context.Context, req api.PostAuthConfirmRequestObject) (api.PostAuthConfirmResponseObject, error) {
	session, err := s.entrance.VerifyCode(ctx, req.Body.Email, req.Body.Code)
	if err != nil {
		return nil, entranceAppErr(err)
	}
	resp := api.PostAuthConfirm200JSONResponse{
		Token:     session.Token,
		ExpiresAt: session.ExpiresAt.Time,
	}
	user, err := s.users.GetByID(ctx, session.UserID)
	if err == nil && user.Name.Valid {
		resp.Name = &user.Name.String
	}
	return resp, nil
}

func (s *HTTPServer) GetAuthMe(ctx context.Context, _ api.GetAuthMeRequestObject) (api.GetAuthMeResponseObject, error) {
	userID, ok := userIDFromContext(ctx)
	if !ok {
		return nil, apierr.New(apierr.ErrInvalidToken, "unauthorized")
	}
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, apierr.New(apierr.ErrInternal, "internal server error")
	}
	resp := api.GetAuthMe200JSONResponse{UserId: userID, Email: user.Email}
	if user.Name.Valid {
		resp.Name = &user.Name.String
	}
	return resp, nil
}

func (s *HTTPServer) PostAuthLogout(ctx context.Context, _ api.PostAuthLogoutRequestObject) (api.PostAuthLogoutResponseObject, error) {
	if session, ok := sessionFromContext(ctx); ok {
		if err := s.sessionService.EndSession(ctx, int(session.ID)); err != nil {
			var appErr *apierr.AppError
			if !errors.As(err, &appErr) || appErr.ErrorCode != apierr.ErrSessionNotFound {
				return nil, apierr.New(apierr.ErrInternal, "internal server error")
			}
		}
	}
	return api.PostAuthLogout200JSONResponse{Status: "ok"}, nil
}

func (s *HTTPServer) PatchAuthMe(ctx context.Context, req api.PatchAuthMeRequestObject) (api.PatchAuthMeResponseObject, error) {
	userID, ok := userIDFromContext(ctx)
	if !ok {
		return nil, apierr.New(apierr.ErrInvalidToken, "unauthorized")
	}
	user, err := s.users.UpdateName(ctx, userID, req.Body.Name)
	if errors.Is(err, service.ErrInvalidName) {
		return nil, apierr.New(apierr.ErrValidation, err.Error())
	}
	if err != nil {
		return nil, apierr.New(apierr.ErrInternal, "internal server error")
	}
	resp := api.PatchAuthMe200JSONResponse{UserId: userID}
	if user.Name.Valid {
		resp.Name = &user.Name.String
	}
	return resp, nil
}

func entranceAppErr(err error) *apierr.AppError {
	switch {
	case errors.Is(err, service.ErrInvalidEmail):
		return apierr.New(apierr.ErrInvalidEmail, err.Error())
	case errors.Is(err, service.ErrInvalidCode):
		return apierr.New(apierr.ErrInvalidCode, err.Error())
	case errors.Is(err, service.ErrTooManyAttempts):
		return apierr.New(apierr.ErrTooManyAttempts, err.Error())
	case errors.Is(err, service.ErrCodeRecentlySent):
		return apierr.New(apierr.ErrCodeRecentlySent, err.Error())
	default:
		return apierr.New(apierr.ErrInternal, "internal server error")
	}
}
