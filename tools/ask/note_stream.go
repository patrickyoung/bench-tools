package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/patrickyoung/ask/internal/event"
)

// Each stdout acknowledgement follows both fsyncs. This finite stdin filter
// holds the existing single-writer lock; it is not a background log service.
func streamNotes(path, dir, source string) (code int) {
	var log *event.Log
	defer func() {
		if log != nil {
			closeLog(log, &code)
		}
	}()
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 64<<10), maxAttachment+1)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) > maxAttachment {
			return fail(errors.New("streamed note exceeds 16 MB"))
		}
		var input struct {
			Kind string          `json:"kind"`
			Body json.RawMessage `json:"body"`
		}
		decoder := json.NewDecoder(bytes.NewReader(line))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			return fail(fmt.Errorf("streamed note: %w", err))
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			return fail(errors.New("streamed note requires one JSON object per line"))
		}
		if input.Kind == "" || strings.ContainsAny(input.Kind, " \t\r\n") || !json.Valid(input.Body) {
			return fail(errors.New("streamed note requires a one-word kind and valid JSON body"))
		}
		if log == nil {
			var err error
			if path == "" {
				path, err = event.Current(dir)
				if err != nil {
					return fail(err)
				}
			}
			log, _, err = event.Open(path)
			if err != nil {
				return fail(err)
			}
		}
		record, err := log.AppendSealed(event.Note, event.NoteData{Source: source, Kind: input.Kind, Body: input.Body})
		if err != nil {
			return fail(err)
		}
		if _, err := fmt.Fprintf(os.Stdout, "{\"seq\":%d}\n", record.Seq); err != nil {
			return fail(fmt.Errorf("acknowledge sealed note: %w", err))
		}
	}
	if err := scanner.Err(); err != nil {
		return fail(err)
	}
	if log == nil {
		return fail(errors.New("no streamed notes"))
	}
	return 0
}
