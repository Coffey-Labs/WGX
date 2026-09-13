//go:build !linux

package wg

import (
	"errors"
	"log/slog"
)

var errLinuxOnly = errors.New("real WireGuard interfaces are only supported on Linux; use IHASVPN_BACKEND=mock for development")

// KernelAvailable is always false off Linux.
func KernelAvailable() bool { return false }

// NewKernel is unavailable off Linux.
func NewKernel(name string, log *slog.Logger) (Backend, error) { return nil, errLinuxOnly }

// NewUserspace is unavailable off Linux.
func NewUserspace(name string, log *slog.Logger) (Backend, error) { return nil, errLinuxOnly }
