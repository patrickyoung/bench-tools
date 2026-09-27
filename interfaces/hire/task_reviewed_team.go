package main

import (
	"fmt"
	"regexp"
	"strings"
)

var teamFingerprint = regexp.MustCompile(`^[a-f0-9]{64}$`)

func validateReviewedTeams(value string) error {
	if value == "" {
		return nil
	}
	items := strings.Split(value, ",")
	if len(items) > 32 {
		return fmt.Errorf("too many reviewed team fingerprints")
	}
	for _, digest := range items {
		if !teamFingerprint.MatchString(digest) {
			return fmt.Errorf("reviewed team controllers require full SHA-256 fingerprints")
		}
	}
	return nil
}

// The operator selects exact reviewed source, including every member and check.
// Neither a model response nor files in the execution root can grant host access.
// New content or executable modes require a separate review and startup selection.
func reviewedTeamController(cfg config, expert string) bool {
	if cfg.ReviewedTeamControllers == "" || expert == "" || validateReviewedTeams(cfg.ReviewedTeamControllers) != nil {
		return false
	}
	_, digest, err := definitionSnapshot(cfg.Data, expert)
	if err != nil {
		return false
	}
	for _, approved := range strings.Split(cfg.ReviewedTeamControllers, ",") {
		if digest == approved {
			return true
		}
	}
	return false
}
