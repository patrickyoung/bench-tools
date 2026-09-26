package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"strings"
)

type nameRecord struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Slug  string `json:"slug"`
	Theme string `json:"theme"`
}

type vocabulary struct{ adjectives, groups []string }

var animals = []string{"Otter", "Badger", "Capybara", "Fox", "Gecko", "Hedgehog", "Koala", "Lemur", "Panda", "Puffin", "Quokka", "Wombat"}

var themes = map[string]vocabulary{
	"playful": {
		[]string{"Bouncy", "Clever", "Dapper", "Fuzzy", "Jolly", "Lucky", "Merry", "Nimble", "Peppy", "Snappy", "Sprightly", "Zippy"},
		[]string{"Crew", "Club", "Collective", "Society", "Squad", "Troop"},
	},
	"space": {
		[]string{"Cosmic", "Galactic", "Lunar", "Meteor", "Nebula", "Orbital", "Rocket", "Solar", "Starlit", "Stellar", "Supernova", "Twilight"},
		[]string{"Crew", "Constellation", "Fleet", "Guild", "Squad", "Voyagers"},
	},
	"nature": {
		[]string{"Alpine", "Cedar", "Dewy", "Fern", "Forest", "Meadow", "Mossy", "River", "Summit", "Wild", "Willow", "Woodland"},
		[]string{"Grove", "Circle", "Crew", "Collective", "Trailblazers", "Rangers"},
	},
}

const baseAttempts = 32
const suffixAttempts = 16

func pickName(theme string) (string, error) {
	words, ok := themes[theme]
	if !ok {
		return "", fmt.Errorf("theme must be playful, space or nature")
	}
	parts := make([]string, 0, 3)
	for _, list := range [][]string{words.adjectives, animals, words.groups} {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(list))))
		if err != nil {
			return "", err
		}
		parts = append(parts, list[n.Int64()])
	}
	return strings.Join(parts, " "), nil
}

func randomHex(entropy io.Reader, n int) (string, error) {
	b := make([]byte, n)
	if _, err := io.ReadFull(entropy, b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func generate(dir, theme string) (nameRecord, error) {
	return reserve(dir, theme, rand.Reader, pickName)
}

// O_EXCL on the slug is the reservation, including across processes. The
// spelling rules map each generated display name to exactly one safe slug.
// Do not remove a reservation after an uncertain write or response failure.
func reserve(dir, theme string, entropy io.Reader, choose func(string) (string, error)) (nameRecord, error) {
	var zero nameRecord
	if err := registryArgument(dir); err != nil {
		return zero, err
	}
	if _, ok := themes[theme]; !ok {
		return zero, fmt.Errorf("theme must be playful, space or nature")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return zero, fmt.Errorf("create registry: %w", err)
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() {
		return zero, fmt.Errorf("registry must be a real directory, not a symlink")
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return zero, err
	}
	defer root.Close()
	info, err = root.Stat(".")
	if err != nil {
		return zero, err
	}
	if info.Mode().Perm() != 0700 {
		return zero, fmt.Errorf("registry must be private with permissions 0700")
	}
	base := ""
	for attempt := 0; attempt < baseAttempts+suffixAttempts; attempt++ {
		if attempt < baseAttempts {
			base, err = choose(theme)
			if err != nil {
				return zero, fmt.Errorf("choose name: %w", err)
			}
		}
		name := base
		if attempt >= baseAttempts {
			suffix, err := randomHex(entropy, 4)
			if err != nil {
				return zero, fmt.Errorf("choose suffix: %w", err)
			}
			name += " " + suffix
		}
		id, err := randomHex(entropy, 16)
		if err != nil {
			return zero, fmt.Errorf("choose identity: %w", err)
		}
		value := nameRecord{ID: id, Name: name, Slug: strings.ToLower(strings.ReplaceAll(name, " ", "-")), Theme: theme}
		file, err := root.OpenFile(value.Slug+".json", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return zero, fmt.Errorf("reserve name: %w", err)
		}
		err = json.NewEncoder(file).Encode(value)
		if err == nil {
			err = file.Sync()
		}
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			return zero, fmt.Errorf("save reservation: %w", err)
		}
		// Retain the directory entry before reporting a committed reservation.
		directory, err := root.Open(".")
		if err != nil {
			return zero, err
		}
		err = directory.Sync()
		closeErr = directory.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			return zero, fmt.Errorf("sync registry: %w", err)
		}
		return value, nil
	}
	return zero, fmt.Errorf("name collisions exhausted the reservation budget; try again")
}
