package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Selection is controller-validated before any authoring or execution. The
// reasons remain model judgments, not proof that a candidate was inspected.
type taskSelection struct {
	Reviewed []taskCandidate `json:"reviewed"`
	Gap      string          `json:"gap"`
	Roles    []taskMember    `json:"roles"`
}
type taskCandidate struct {
	Target string `json:"target"`
	Fit    string `json:"fit"`
	Reason string `json:"reason"`
}
type taskMember struct {
	Role           string `json:"role"`
	Target         string `json:"target"`
	Responsibility string `json:"responsibility"`
}

var taskRoleName = regexp.MustCompile(`^[a-z][a-z0-9-]{0,39}$`)

func validateTaskSelection(p taskPlan, catalog []taskChoice) error {
	s := p.Selection
	if s == nil || s.Reviewed == nil || s.Roles == nil || !boundedText(s.Gap, 2000, false) {
		return fmt.Errorf("missing library selection")
	}
	if p.Question != "" {
		if len(s.Reviewed) != 0 || len(s.Roles) != 0 || s.Gap != "" {
			return fmt.Errorf("question cannot dispatch a selection")
		}
		return nil
	}
	base, choices := map[string]taskChoice{}, map[string]taskChoice{}
	for _, c := range catalog {
		choices[c.Key] = c
		if !strings.HasPrefix(c.Key, "adapt:") {
			base[c.Key] = c
		}
	}
	if len(s.Reviewed) != len(base) {
		return fmt.Errorf("library review must account for every worker and team")
	}
	reviews := map[string]string{}
	hasFit := false
	for _, r := range s.Reviewed {
		if _, ok := base[r.Target]; !ok || reviews[r.Target] != "" || !boundedText(r.Reason, 600, true) || (r.Fit != "reuse" && r.Fit != "adapt" && r.Fit != "none") {
			return fmt.Errorf("invalid library review")
		}
		reviews[r.Target] = r.Fit
		hasFit = hasFit || r.Fit != "none"
	}
	checkTarget := func(target string) error {
		_, ok := choices[target]
		fit := reviews[strings.TrimPrefix(target, "adapt:")]
		if !ok || fit == "none" || fit == "" || (strings.HasPrefix(target, "adapt:") && fit != "adapt") {
			return fmt.Errorf("selected specialist contradicts library review")
		}
		if strings.HasPrefix(target, "adapt:") && strings.TrimSpace(s.Gap) == "" {
			return fmt.Errorf("adaptation needs a specific capability gap")
		}
		// A direct worker marked as needing adaptation must go through Hire.
		c := choices[target]
		if c.Kind == "worker" && fit == "adapt" && !c.NeedsBuild {
			return fmt.Errorf("worker needs an explicit adaptation target")
		}
		return nil
	}
	switch p.Target {
	case "new:worker":
		if strings.TrimSpace(s.Gap) == "" || hasFit {
			return fmt.Errorf("reuse or adapt available expertise before creating a worker")
		}
	case "new:team":
		for _, r := range s.Reviewed {
			if base[r.Target].Kind == "team" && r.Fit == "reuse" {
				return fmt.Errorf("use the matching existing team before assembling a flash team")
			}
		}
		if len(s.Roles) < 2 || len(s.Roles) > 16 {
			return fmt.Errorf("new team needs a concrete roster")
		}
		seen := map[string]bool{}
		reused := false
		for _, role := range s.Roles {
			if !taskRoleName.MatchString(role.Role) || strings.HasSuffix(role.Role, "-") || role.Role == "state" || seen[role.Role] || !boundedText(role.Responsibility, 2000, true) {
				return fmt.Errorf("invalid team role")
			}
			seen[role.Role] = true
			if role.Target == "new:worker" {
				if strings.TrimSpace(s.Gap) == "" {
					return fmt.Errorf("new member needs a specific capability gap")
				}
				continue
			}
			reused = true
			if err := checkTarget(role.Target); err != nil {
				return err
			}
			if choices[role.Target].Kind != "worker" {
				return fmt.Errorf("team member must be a worker")
			}
		}
		if hasFit && !reused {
			return fmt.Errorf("new team must use available expertise")
		}
	default:
		if err := checkTarget(p.Target); err != nil {
			return err
		}
	}
	if p.Target != "new:team" && len(s.Roles) != 0 {
		return fmt.Errorf("only a new team may declare a roster")
	}
	return nil
}

func taskChoiceFiles(ctx context.Context, root string, rec taskRecord, c taskChoice, exportName string) (map[string]definitionFile, string, error) {
	source := c.Path
	if !c.Local {
		command := "export"
		if c.Kind == "team" {
			command = "export-team"
		}
		dest := filepath.Join(root, exportName)
		code, _, _ := taskCommand(ctx, root, nil, rec.Config.Python, filepath.Join(rec.Config.Source, "scripts", "workers"), command, c.ID, dest, "--ref", rec.Revision, "--allow-experimental")
		if code != 0 {
			return nil, "", taskSelectionExit{code}
		}
		source = filepath.Join(dest, "expert")
	}
	return definitionSnapshot(rec.Config.Data, source)
}

type preparedTaskMember struct {
	Role, Target, Hash string
	Adapt              bool
}

// Materialize selected members ourselves, instead of asking Hire to choose its
// own replacement roster. Snapshots and pins stay outside the authoring root.
func prepareTaskMembers(ctx context.Context, root string, rec taskRecord, plan taskPlan) error {
	if plan.Selection == nil {
		return nil
	}
	if plan.Target != "new:team" {
		return snapshotExistingTaskMembers(root, rec, plan)
	}
	dest := filepath.Join(root, "authoring", "expert", "agents")
	if err := os.MkdirAll(dest, 0700); err != nil {
		return err
	}
	members := []preparedTaskMember{}
	for _, role := range plan.Selection.Roles {
		m := preparedTaskMember{Role: role.Role, Target: role.Target}
		if role.Target == "new:worker" {
			m.Adapt = true
		} else {
			var choice taskChoice
			for _, c := range rec.Catalog {
				if c.Key == role.Target {
					choice = c
				}
			}
			files, hash, err := taskChoiceFiles(ctx, root, rec, choice, "member-"+role.Role)
			if err != nil {
				return err
			}
			m.Hash, m.Adapt = hash, choice.NeedsBuild
			if err := writeDefinition(filepath.Join(dest, role.Role), files); err != nil {
				return err
			}
		}
		members = append(members, m)
	}
	return saveTaskJSON(filepath.Join(root, "selected-members.json"), members)
}

func validatePreparedTaskMembers(root string, rec taskRecord, plan taskPlan) error {
	if plan.Selection == nil {
		return nil
	}
	raw, err := os.ReadFile(filepath.Join(root, "selected-members.json"))
	if os.IsNotExist(err) && plan.Target != "new:team" {
		return nil
	}
	if err != nil {
		return err
	}
	var members []preparedTaskMember
	if err := json.Unmarshal(raw, &members); err != nil {
		return err
	}
	if plan.Target == "new:team" && len(members) != len(plan.Selection.Roles) {
		return fmt.Errorf("selected roster unavailable")
	}
	agents := filepath.Join(root, "authoring", "expert", "agents")
	entries, err := os.ReadDir(agents)
	if err != nil || len(entries) != len(members) {
		return fmt.Errorf("built team does not match selected roster")
	}
	for _, m := range members {
		files, hash, err := definitionSnapshot(rec.Config.Data, filepath.Join(agents, m.Role))
		if err != nil {
			return err
		}
		for _, name := range []string{"AGENTS.md", "README.md", "bin/check"} {
			f, ok := files[name]
			if !ok || strings.TrimSpace(f.Text) == "" || (name == "bin/check" && f.Mode&0111 == 0) {
				return fmt.Errorf("team member %s is missing %s", m.Role, name)
			}
		}
		if !m.Adapt && hash != m.Hash {
			return fmt.Errorf("Hire replaced or changed selected member %s", m.Role)
		}
	}
	return nil
}

func taskSelectionShape(v any) error {
	s, ok := v.(map[string]any)
	if !ok || len(s) != 3 {
		return fmt.Errorf("incomplete selection")
	}
	for _, field := range []string{"reviewed", "roles"} {
		entries, ok := s[field].([]any)
		if !ok {
			return fmt.Errorf("incomplete selection %s", field)
		}
		for _, item := range entries {
			entry, ok := item.(map[string]any)
			if !ok || len(entry) != 3 {
				return fmt.Errorf("incomplete selection entry")
			}
		}
	}
	if _, ok := s["gap"].(string); !ok {
		return fmt.Errorf("missing capability gap field")
	}
	return nil
}

// A selected existing team keeps its tested members while Hire adapts only the
// UI entry adapter. The actual exported roster may be richer than catalog text.
func snapshotExistingTaskMembers(root string, rec taskRecord, plan taskPlan) error {
	team := false
	for _, c := range rec.Catalog {
		if c.Key == plan.Target {
			team = c.Kind == "team"
		}
	}
	if !team {
		return nil
	}
	agents := filepath.Join(root, "authoring", "expert", "agents")
	entries, err := os.ReadDir(agents)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	members := []preparedTaskMember{}
	for _, entry := range entries {
		if !entry.IsDir() {
			return fmt.Errorf("invalid existing team member")
		}
		_, hash, err := definitionSnapshot(rec.Config.Data, filepath.Join(agents, entry.Name()))
		if err != nil {
			return err
		}
		members = append(members, preparedTaskMember{Role: entry.Name(), Target: plan.Target, Hash: hash})
	}
	return saveTaskJSON(filepath.Join(root, "selected-members.json"), members)
}

type taskSelectionExit struct{ Code int }

func (e taskSelectionExit) Error() string { return fmt.Sprintf("library export stopped (%d)", e.Code) }
func taskSelectionStatus(err error) int {
	var failure taskSelectionExit
	if errors.As(err, &failure) {
		return failure.Code
	}
	return 2
}
