//go:build !linux

package netcfg

import "errors"

// DefaultEgress is only implemented on Linux.
func DefaultEgress() (string, error) { return "", errors.New("not supported on this platform") }
