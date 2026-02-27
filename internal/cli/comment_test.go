package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/nuchs/tasker/internal/cli"
	"github.com/nuchs/tasker/internal/model"
)

func runComment(t *testing.T, wd string, args ...string) string {
	t.Helper()
	var buf bytes.Buffer
	if err := cli.RunComment(wd, args, &buf); err != nil {
		t.Fatalf("RunComment(%v): %v", args, err)
	}
	return strings.TrimSpace(buf.String())
}

func TestRunComment_AppearsInEventLog(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "Commented issue")

	runComment(t, wd, "1", "This is a comment")

	s, err := cli.OpenStore(wd)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer s.Close()

	events, err := s.Events(1)
	if err != nil {
		t.Fatalf("Events: %v", err)
	}

	found := false
	for _, ev := range events {
		if ev.Type == model.EventComment && ev.Body == "This is a comment" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("comment event not found in event log; events: %v", events)
	}
}

func TestRunComment_DoesNotChangeMaterialisedState(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "Stable issue", "--priority", "high")

	runComment(t, wd, "1", "Just a note")

	s, err := cli.OpenStore(wd)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer s.Close()

	issue, _, err := s.Show(1)
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if issue.Title != "Stable issue" {
		t.Errorf("title changed: got %q", issue.Title)
	}
	if issue.Priority != "high" {
		t.Errorf("priority changed: got %q", issue.Priority)
	}
	if issue.Status != "open" {
		t.Errorf("status changed: got %q", issue.Status)
	}

	// Index should also be unchanged.
	meta, err := s.GetIssueMeta(1)
	if err != nil {
		t.Fatalf("GetIssueMeta: %v", err)
	}
	if meta.Status != "open" {
		t.Errorf("index status changed: got %q", meta.Status)
	}
}

func TestRunComment_AcceptsPrefixedID(t *testing.T) {
	wd := initTracker(t, "PROJ")
	runCreate(t, wd, "--title", "Issue")

	got := runComment(t, wd, "PROJ-0001", "prefixed id comment")
	if got != "PROJ-0001" {
		t.Errorf("expected PROJ-0001, got %q", got)
	}
}

func TestRunComment_AcceptsBareID(t *testing.T) {
	wd := initTracker(t, "PROJ")
	runCreate(t, wd, "--title", "Issue")

	got := runComment(t, wd, "1", "bare id comment")
	if got != "PROJ-0001" {
		t.Errorf("expected PROJ-0001, got %q", got)
	}
}

func TestRunComment_MultipleComments(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "Multi-comment issue")

	runComment(t, wd, "1", "First comment")
	runComment(t, wd, "1", "Second comment")

	s, err := cli.OpenStore(wd)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer s.Close()

	events, err := s.Events(1)
	if err != nil {
		t.Fatalf("Events: %v", err)
	}

	var comments []string
	for _, ev := range events {
		if ev.Type == model.EventComment {
			comments = append(comments, ev.Body)
		}
	}
	if len(comments) != 2 {
		t.Fatalf("expected 2 comment events, got %d", len(comments))
	}
	if comments[0] != "First comment" || comments[1] != "Second comment" {
		t.Errorf("unexpected comments: %v", comments)
	}
}

func TestRunComment_MissingArgs(t *testing.T) {
	wd := initTracker(t, "T")
	var buf bytes.Buffer
	if err := cli.RunComment(wd, []string{"1"}, &buf); err == nil {
		t.Fatal("expected error when message is missing")
	}
}

func TestRunComment_MissingID(t *testing.T) {
	wd := initTracker(t, "T")
	var buf bytes.Buffer
	if err := cli.RunComment(wd, []string{}, &buf); err == nil {
		t.Fatal("expected error when ID is missing")
	}
}

func TestRunComment_InvalidID(t *testing.T) {
	wd := initTracker(t, "T")
	var buf bytes.Buffer
	if err := cli.RunComment(wd, []string{"not-an-id", "msg"}, &buf); err == nil {
		t.Fatal("expected error for invalid ID")
	}
}

func TestRunComment_EmptyMessage(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "Issue")
	var buf bytes.Buffer
	if err := cli.RunComment(wd, []string{"1", ""}, &buf); err == nil {
		t.Fatal("expected error for empty message")
	}
}
