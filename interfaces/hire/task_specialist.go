package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

type taskExpertiseMember struct {
	Name string `json:"name"`
	Mode string `json:"mode"`
}
type taskExpertiseNote struct {
	Title   string                `json:"title"`
	Why     string                `json:"why"`
	Members []taskExpertiseMember `json:"members"`
}

type taskSpecialist struct {
	Temporary  bool              `json:"temporary"`
	Expertise  taskExpertiseNote `json:"expertise"`
	References string            `json:"references"`
	Access     string            `json:"access"`
	Name       string            `json:"name"`
	Kind       string            `json:"kind"`
	About      string            `json:"about"`
	Reason     string            `json:"reason"`
	Source     string            `json:"source"`
	Members    []string          `json:"members"`
}

func plannedSpecialist(plan taskPlan, catalog []taskChoice) taskSpecialist {
	s := taskSpecialist{Name: "New specialist", Kind: "worker", Source: "Created for this conversation", Reason: truncateMessage(plan.Message, 400)}
	if plan.Target == "new:team" {
		s.Name = "New team"
		s.Kind = "team"
	}
	for _, c := range catalog {
		if c.Key != plan.Target {
			continue
		}
		s.Name, s.Kind, s.About = c.Name, c.Kind, truncateMessage(c.Description, 400)
		s.Source = "Selected from your Bench library"
		if c.Local {
			s.Source = "Reused from your local workers"
		}
		if c.NeedsBuild {
			s.Source = "Adapting an existing specialist for this request"
		}
		for role, m := range c.Members {
			s.Members = append(s.Members, role+": "+m.Worker)
		}
		sort.Strings(s.Members)
		break
	}
	if plan.Selection != nil && plan.Target == "new:team" {
		for _, role := range plan.Selection.Roles {
			name := "New specialist"
			for _, c := range catalog {
				if c.Key == role.Target {
					name = c.Name
					break
				}
			}
			s.Members = append(s.Members, role.Role+": "+name)
		}
	}
	s.Expertise = plannedExpertise(plan, catalog)
	return s
}

func (a *app) taskSpecialist(j Job, rec taskRecord, result taskResult) taskSpecialist {
	root := filepath.Dir(j.Dir)
	if rec.Resume != nil {
		root = rec.Resume.Root
	}
	var s taskSpecialist
	if raw, err := readText(root, "specialist.json", 16<<10); err == nil {
		_ = json.Unmarshal([]byte(raw), &s)
	}
	if s.Name == "" {
		// Older conversations retain the exact selected plan and catalog too.
		request, e := readText(root, "plan/request.json", 2<<20)
		response, e2 := readText(root, "plan/response.json", 128<<10)
		if e == nil && e2 == nil {
			if plan, err := decodeTaskPlan([]byte(response), []byte(request), rec.Catalog); err == nil && plan.Question == "" {
				s = plannedSpecialist(plan, rec.Catalog)
			}
		}
	}
	if s.Name == "" {
		return s
	}
	s.References = "No reference files attached"
	if files, err := taskAttachments(a.cfg.Data, rec.Thread); err == nil && len(files) > 0 {
		s.References = fmt.Sprintf("Attached references: %d", len(files))
	}
	s.Access = "Local files · web research off"
	if rec.Research {
		s.Access = "Web research enabled"
	}
	if s.Kind == "team" {
		s.Access = "Team members follow their own access settings"
	}
	// The actual frozen definition is more specific than a previous job's title.
	expert := result.Expert
	if expert == "" {
		expert = filepath.Join(root, "authoring", "expert")
	}
	if within(filepath.Join(a.cfg.Data, "workspaces"), expert) {
		rel, _ := filepath.Rel(a.cfg.Data, filepath.Join(expert, "README.md"))
		if guide, err := readText(a.cfg.Data, rel, 64<<10); err == nil {
			for _, line := range strings.Split(guide, "\n") {
				if strings.HasPrefix(line, "# ") && !s.Temporary {
					s.Name = truncateMessage(strings.TrimSpace(strings.TrimPrefix(line, "# ")), 120)
					break
				}
			}
			if s.About == "" {
				for _, paragraph := range strings.Split(guide, "\n\n") {
					text := strings.TrimSpace(paragraph)
					if text != "" && !strings.HasPrefix(text, "#") && !strings.HasPrefix(text, "```") {
						s.About = truncateMessage(text, 400)
						break
					}
				}
			}
		}
	}
	if result.Flash != nil {
		s.Name = result.Flash.Name
		s.Source = "Flash team · kept with this conversation"
		s.Temporary = true
	}
	return s
}

// An explicit target lets the planner request adaptation through public Hire.
// A selected worker is copied before adaptation; the library source is unchanged.
func withTaskAdaptations(choices []taskChoice) []taskChoice {
	out := append([]taskChoice{}, choices...)
	for _, c := range choices {
		if c.Kind != "worker" || c.NeedsBuild || strings.HasPrefix(c.Key, "adapt:") {
			continue
		}
		c.Key = "adapt:" + c.Key
		c.NeedsBuild = true
		c.Name = "Adapt " + c.Name
		c.Description = "Adapt this existing worker with Hire when this request needs capabilities its current instructions, renderer or checks cannot provide. " + c.Description
		out = append(out, c)
	}
	return out
}

// Explain preparation before Hire starts. These are selected roles, never
// claims that an unevaluated new or adapted worker has already proved itself.
func plannedExpertise(plan taskPlan, catalog []taskChoice) taskExpertiseNote {
	note := taskExpertiseNote{}
	if plan.Selection == nil || plan.Question != "" {
		return note
	}
	member := func(target, fallback string) taskExpertiseMember {
		item := taskExpertiseMember{Name: fallback, Mode: "Creating new"}
		for _, c := range catalog {
			if c.Key != target {
				continue
			}
			item.Name = strings.TrimPrefix(c.Name, "Adapt ")
			item.Mode = "Using existing"
			if c.NeedsBuild {
				item.Mode = "Adapting existing"
			}
			break
		}
		return item
	}
	switch {
	case plan.Target == "new:team":
		note.Title = "Here’s who I’m bringing in"
		for _, role := range plan.Selection.Roles {
			note.Members = append(note.Members, member(role.Target, displayName(role.Role)))
		}
	case plan.Target == "new:worker":
		note.Title = "One more skill is needed"
		note.Members = []taskExpertiseMember{member(plan.Target, "New specialist")}
	case strings.HasPrefix(plan.Target, "adapt:"):
		note.Title = "Adding a skill to your specialist"
		note.Members = []taskExpertiseMember{member(plan.Target, "Your specialist")}
	default:
		return note
	}
	note.Why = truncateMessage(plan.Selection.Gap, 600)
	if note.Why == "" {
		note.Why = truncateMessage(plan.Message, 600)
	}
	return note
}
