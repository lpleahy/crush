package model

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/shell"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// jobsDoneVisibleFor is how many seconds a finished background job stays in the
// sidebar after it completes before it ages out of view.
const jobsDoneVisibleFor = 8

// jobsTickMsg drives re-evaluation of the jobs panel so finished jobs age out
// of the visible window without waiting for another job event.
type jobsTickMsg struct{}

// jobsTickCmd schedules the next jobs-panel re-evaluation.
func (m *UI) jobsTickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return jobsTickMsg{} })
}

// visibleJobs returns the jobs to show in the sidebar: running/blocked jobs
// always, finished jobs only for a short window after completion. Running
// (incl. blocked) jobs sort before finished ones, each in launch order.
func (m *UI) visibleJobs() []shell.JobInfo {
	return filterVisibleJobs(m.jobStates, time.Now().Unix())
}

// filterVisibleJobs keeps running/blocked jobs plus finished jobs still inside
// the visible window, sorted running-first then by launch order (ID).
func filterVisibleJobs(jobs []shell.JobInfo, now int64) []shell.JobInfo {
	out := make([]shell.JobInfo, 0, len(jobs))
	for _, j := range jobs {
		if !j.Done || (j.CompletedAt > 0 && now-j.CompletedAt <= jobsDoneVisibleFor) {
			out = append(out, j)
		}
	}
	sort.SliceStable(out, func(i, k int) bool {
		if out[i].Done != out[k].Done {
			return !out[i].Done // running/blocked before finished
		}
		return out[i].ID < out[k].ID
	})
	return out
}

// jobsInfo renders the background-tasks sidebar section.
func (m *UI) jobsInfo(width, maxItems int, isSection bool) string {
	t := m.com.Styles
	jobs := m.visibleJobs()

	title := t.Resource.Heading.Render("Tasks")
	if isSection {
		title = common.Section(t, title, width)
	}
	list := t.Resource.AdditionalText.Render("None")
	if len(jobs) > 0 {
		list = jobsList(t, jobs, width, maxItems)
	}
	return lipgloss.NewStyle().Width(width).Render(fmt.Sprintf("%s\n\n%s", title, list))
}

func jobsList(t *styles.Styles, jobs []shell.JobInfo, width, maxItems int) string {
	if maxItems <= 0 {
		return ""
	}
	var rendered []string
	for _, j := range jobs {
		label := j.Description
		if label == "" {
			label = j.Command
		}
		// Collapse to one line and truncate; Status only truncates the
		// description, so leave room for the icon, status, and "#ID".
		label = strings.Join(strings.Fields(label), " ")
		label = truncateLabel(label, max(8, width-22))
		title := t.Resource.Name.Render(label)

		var icon, description string
		switch {
		case j.Waiting:
			icon = t.Resource.BusyIcon.String()
			description = t.Resource.StatusText.Render("agent waiting")
		case !j.Done:
			icon = t.Resource.BusyIcon.String()
			description = t.Resource.StatusText.Render("running")
		case j.ExitErr != nil:
			icon = t.Resource.ErrorIcon.String()
			description = t.Resource.StatusText.Render("failed")
		default:
			icon = t.Resource.OnlineIcon.String()
			description = t.Resource.StatusText.Render("done")
		}

		rendered = append(rendered, common.Status(t, common.StatusOpts{
			Icon:         icon,
			Title:        title,
			Description:  description,
			ExtraContent: t.Resource.CapabilityCount.Render("#" + j.ID),
		}, width))
	}

	if len(rendered) > maxItems {
		visible := rendered[:maxItems-1]
		remaining := len(rendered) - maxItems
		visible = append(visible, t.Resource.AdditionalText.Render(fmt.Sprintf("…and %d more", remaining)))
		return lipgloss.JoinVertical(lipgloss.Left, visible...)
	}
	return lipgloss.JoinVertical(lipgloss.Left, rendered...)
}

func truncateLabel(s string, maxWidth int) string {
	r := []rune(s)
	if len(r) <= maxWidth {
		return s
	}
	if maxWidth <= 1 {
		return "…"
	}
	return string(r[:maxWidth-1]) + "…"
}
