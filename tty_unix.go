//go:build !windows

package main

import "os"

func openControllingTTY() (readWriteCloser, error) {
	return os.OpenFile("/dev/tty", os.O_RDWR, 0)
}
