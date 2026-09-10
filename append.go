// Append admits an attributed message without asking for another model turn.
// The observation belongs in durable conversation history before its caller
// decides to continue, pause, or stop. Notes remain non-conversational records.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/patrickyoung/ask/internal/event"
)

func cmdAppend(args []string) (code int) {
	fs := flag.NewFlagSet("append", flag.ContinueOnError)
	dir := fs.String("d", askDir(), "conversation directory")
	file := fs.String("f", "", "existing session file (default: current)")
	source := fs.String("s", "", "program writing the message (required, one word)")
	quiet := fs.Bool("q", false, "no progress on stderr; errors still print")
	usage(fs, "ask append -s source [flags] [text ...]")
	if err := fs.Parse(args); err != nil {
		return usageCode(fs, err)
	}
	if *source == "" || !utf8.ValidString(*source) || strings.ContainsFunc(*source, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return fail(errors.New("append requires -s: one word naming the program writing the message"))
	}
	text, err := noteText(fs.Args())
	if err != nil {
		return fail(err)
	}
	if strings.TrimSpace(text) == "" {
		return fail(errors.New("nothing to append: pass text as arguments or on stdin"))
	}
	if len(text) > maxAttachment {
		return fail(fmt.Errorf("message is larger than %d MB", maxAttachment>>20))
	}
	if !utf8.ValidString(text) {
		return fail(errors.New("append needs UTF-8 text; encode binary observations explicitly"))
	}
	// Preserve exact message bytes. The manifest indexes the recognized
	// snapshot in those bytes, without trimming or duplicating the message.
	trimmed := strings.TrimSpace(text)
	offset := strings.Index(text, trimmed)
	evidence, claimed, err := event.ContextEvidence([]byte(trimmed), 0, offset)
	if err != nil {
		return fail(err)
	}
	if !claimed {
		evidence, err = envelopedContextEvidence(trimmed, 0)
		if err != nil {
			return fail(err)
		}
		if evidence != nil {
			evidence.Offset += offset
		}
	}
	user := event.UserData{Text: text, Source: *source, Evidence: evidence, Sealed: true}
	if err := user.CheckEvidence(); err != nil {
		return fail(err)
	}
	path := *file
	if path == "" {
		path, err = event.Current(*dir)
		if err != nil {
			return fail(fmt.Errorf("no current conversation in %s: %w", *dir, err))
		}
	}
	// Open acquires the writer lock before reading and verifying the prefix.
	log, events, err := event.Open(path)
	if err != nil {
		return fail(err)
	}
	defer closeLog(log, &code)
	if len(events) == 0 || events[0].Type != event.Session {
		return fail(errors.New("append requires an existing session header"))
	}
	if _, err := log.AppendSealed(event.User, user); err != nil {
		return fail(err)
	}
	closeLog(log, &code)
	if code == 0 && !*quiet {
		fmt.Fprintf(os.Stderr, "ask: appended %d bytes in %s as %s\n", len(text), log.ID(), *source)
	}
	return code
}
