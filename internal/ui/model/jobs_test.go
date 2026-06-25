package model

import (
	"errors"
	"testing"

	"github.com/charmbracelet/crush/internal/shell"
)

func TestFilterVisibleJobs(t *testing.T) {
	t.Parallel()

	const now int64 = 1000
	jobs := []shell.JobInfo{
		{ID: "003", Done: true, CompletedAt: now - 100},                            // old done -> hidden
		{ID: "001", Done: false},                                                   // running
		{ID: "004", Done: true, ExitErr: errors.New("boom"), CompletedAt: now - 2}, // recent failed -> shown
		{ID: "002", Done: false, Waiting: true},                                    // blocked (still running)
		{ID: "005", Done: true, CompletedAt: now},                                  // just done -> shown
	}

	got := filterVisibleJobs(jobs, now)

	var ids []string
	for _, j := range got {
		ids = append(ids, j.ID)
	}

	// Running/blocked first in launch order (001, 002), then recently-finished
	// (004, 005); the old completed job (003) is filtered out.
	want := []string{"001", "002", "004", "005"}
	if len(ids) != len(want) {
		t.Fatalf("filterVisibleJobs ids = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("filterVisibleJobs ids = %v, want %v", ids, want)
		}
	}
}
