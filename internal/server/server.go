package server

import "context"

type Server interface {
	Run(addr string) error
	Shutdown(ctx context.Context) error
}
