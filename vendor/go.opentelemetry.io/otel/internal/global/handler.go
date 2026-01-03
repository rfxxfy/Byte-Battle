// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package global provides the OpenTelemetry global API.
package global // import "go.opentelemetry.io/otel/internal/global"

import (
<<<<<<< HEAD
	"go.opentelemetry.io/otel/internal/errorhandler"
)

// ErrorHandler is an alias for errorhandler.ErrorHandler, kept for backward
// compatibility with existing callers of internal/global.
type ErrorHandler = errorhandler.ErrorHandler

// ErrDelegator is an alias for errorhandler.ErrDelegator, kept for backward
// compatibility with existing callers of internal/global.
type ErrDelegator = errorhandler.ErrDelegator
=======
	"log"
	"sync/atomic"
)

// ErrorHandler handles irremediable events.
type ErrorHandler interface {
	// Handle handles any error deemed irremediable by an OpenTelemetry
	// component.
	Handle(error)
}

type ErrDelegator struct {
	delegate atomic.Pointer[ErrorHandler]
}

// Compile-time check that delegator implements ErrorHandler.
var _ ErrorHandler = (*ErrDelegator)(nil)

func (d *ErrDelegator) Handle(err error) {
	if eh := d.delegate.Load(); eh != nil {
		(*eh).Handle(err)
		return
	}
	log.Print(err)
}

// setDelegate sets the ErrorHandler delegate.
func (d *ErrDelegator) setDelegate(eh ErrorHandler) {
	d.delegate.Store(&eh)
}
>>>>>>> f0895f0 (fix issues)
