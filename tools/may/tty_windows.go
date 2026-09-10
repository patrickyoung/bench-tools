//go:build windows

package main

import "errors"

func openControllingTTY() (readWriteCloser, error) {
	return nil, errors.New("controlling-terminal approval is unsupported on Windows")
}
