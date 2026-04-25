package service

import (
	"context"
	"sort"

	"github.com/AngheloAlva/timer/internal/domain"
)

// Projects and Projects[].Tasks are pre-sorted by Total DESC.
type ReportSummary struct {
	Total    int64
	Projects []ProjectSummary
}

type ProjectSummary struct {
	ID    string
	Name  string
	Slug  string
	Total int64
	Tasks []TaskSummary
}

type TaskSummary struct {
	ID    string
	Title string
	Total int64
}

func (s *TimerService) BuildReport(ctx context.Context, opts ListEntriesOpts) (ReportSummary, error) {
	if opts.Max == 0 {
		opts.Max = 10000
	}
	entries, err := s.ListEntries(ctx, opts)
	if err != nil {
		return ReportSummary{}, err
	}
	return AggregateEntries(entries), nil
}

func AggregateEntries(entries []domain.TimeEntry) ReportSummary {
	type projAcc struct {
		ProjectSummary
		taskByID map[string]*TaskSummary
	}

	byProj := map[string]*projAcc{}
	var total int64

	for _, e := range entries {
		p, ok := byProj[e.ProjectID]
		if !ok {
			p = &projAcc{
				ProjectSummary: ProjectSummary{
					ID:   e.ProjectID,
					Name: e.ProjectName,
					Slug: e.ProjectSlug,
				},
				taskByID: map[string]*TaskSummary{},
			}
			byProj[e.ProjectID] = p
		}
		p.Total += e.DurationSec

		t, ok := p.taskByID[e.TaskID]
		if !ok {
			t = &TaskSummary{ID: e.TaskID, Title: e.TaskTitle}
			p.taskByID[e.TaskID] = t
		}
		t.Total += e.DurationSec

		total += e.DurationSec
	}

	out := ReportSummary{Total: total, Projects: make([]ProjectSummary, 0, len(byProj))}
	for _, p := range byProj {
		tasks := make([]TaskSummary, 0, len(p.taskByID))
		for _, t := range p.taskByID {
			tasks = append(tasks, *t)
		}
		sort.SliceStable(tasks, func(i, j int) bool { return tasks[i].Total > tasks[j].Total })

		ps := p.ProjectSummary
		ps.Tasks = tasks
		out.Projects = append(out.Projects, ps)
	}
	sort.SliceStable(out.Projects, func(i, j int) bool { return out.Projects[i].Total > out.Projects[j].Total })

	return out
}
