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
	return api.PostAuthConfirm200JSONResponse{
		Token:     session.Token,
		ExpiresAt: session.ExpiresAt.Time,
	}, nil
}

func (s *HTTPServer) GetAuthMe(ctx context.Context, _ api.GetAuthMeRequestObject) (api.GetAuthMeResponseObject, error) {
	userID, ok := userIDFromContext(ctx)
	if !ok {
		return nil, apierr.New(apierr.ErrInvalidToken, "unauthorized")
	}
	return api.GetAuthMe200JSONResponse{UserId: userID}, nil
}

func (s *HTTPServer) PostAuthLogout(ctx context.Context, _ api.PostAuthLogoutRequestObject) (api.PostAuthLogoutResponseObject, error) {
	if session, ok := sessionFromContext(ctx); ok {
		_ = s.sessionService.EndSession(ctx, int(session.ID))
	}
	return api.PostAuthLogout200JSONResponse{Status: "ok"}, nil
}

func entranceAppErr(err error) *apierr.AppError {
	switch {
	case errors.Is(err, service.ErrInvalidEmail):
		return apierr.New(apierr.ErrInvalidEmail, err.Error())
	case errors.Is(err, service.ErrInvalidCode):
		return apierr.New(apierr.ErrInvalidCode, err.Error())
	case errors.Is(err, service.ErrTooManyAttempts):
		return apierr.New(apierr.ErrTooManyAttempts, err.Error())
	default:
		return apierr.New(apierr.ErrInternal, "internal server error")
	}
}
