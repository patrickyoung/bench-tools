//go:build darwin

package main

import "testing"

func TestSeatbeltQuote(t *testing.T) {
	got := seatbeltQuote("/tmp/a\\b\"c\n")
	want := `/tmp/a\\b\"c\n`
	if got != want {
		t.Errorf("quote = %q, want %q", got, want)
	}
}
