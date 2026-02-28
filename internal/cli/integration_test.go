package cli_test

// End-to-end integration tests that exercise the full CLI workflow.
// These tests use only the public RunXxx functions and the Store API;
// they do not reach into package internals beyond what the CLI itself uses.

import (
	"strings"
	"testing"

	"github.com/nuchs/tasker/internal/cli"
	"github.com/nuchs/tasker/internal/index"
	"github.com/nuchs/tasker/internal/model"
)

// TestFullLifecycle exercises the complete issue workflow end-to-end:
// init → create (with dependencies) → list → ready → claim → comment →
// update → release → show (with events) → rebuild.
func TestFullLifecycle(t *testing.T) {
	wd := initTracker(t, "PROJ")

	// --- Create several issues ---
	id1 := runCreate(t, wd, "--title", "Foundation", "--priority", "high", "--type", "task",
		"--description", "Must complete first",
		"--acceptance-criteria", "Foundation done")
	id2 := runCreate(t, wd, "--title", "Build on Foundation", "--priority", "medium")
	id3 := runCreate(t, wd, "--title", "Independent Work", "--priority", "low")

	if id1 != "PROJ-0001" || id2 != "PROJ-0002" || id3 != "PROJ-0003" {
		t.Fatalf("unexpected IDs: %s %s %s", id1, id2, id3)
	}

	// --- Set issue 2 to depend on issue 1 via the store ---
	s, err := cli.OpenStore(wd)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	if err := s.Append(2, model.Event{
		Type:    model.EventDependenciesChanged,
		Depends: []int{1},
	}); err != nil {
		t.Fatalf("set dependency: %v", err)
	}
	s.Close()

	// --- List: all three should appear (non-done, non-cancelled) ---
	listOut := runList(t, wd)
	for _, title := range []string{"Foundation", "Build on Foundation", "Independent Work"} {
		if !strings.Contains(listOut, title) {
			t.Errorf("list missing %q:\n%s", title, listOut)
		}
	}

	// --- Ready: only issues 1 and 3 — issue 2 is blocked by unresolved dep ---
	readyOut := runReady(t, wd)
	if !strings.Contains(readyOut, "Foundation") {
		t.Errorf("ready: expected issue 1 (Foundation):\n%s", readyOut)
	}
	if !strings.Contains(readyOut, "Independent Work") {
		t.Errorf("ready: expected issue 3 (Independent Work):\n%s", readyOut)
	}
	if strings.Contains(readyOut, "Build on Foundation") {
		t.Errorf("ready: issue 2 should be blocked by dep on issue 1:\n%s", readyOut)
	}

	// --- Claim issue 1 ---
	runClaim(t, wd, "1", "--agent", "agent-alpha", "--session", "sess-001")

	// Ready now excludes issue 1 (claimed) and issue 2 (dep unresolved); only 3 visible.
	readyOut = runReady(t, wd)
	if strings.Contains(readyOut, "PROJ-0001") {
		t.Errorf("ready: claimed issue 1 should not appear:\n%s", readyOut)
	}
	if strings.Contains(readyOut, "PROJ-0002") {
		t.Errorf("ready: issue 2 still blocked:\n%s", readyOut)
	}
	if !strings.Contains(readyOut, "Independent Work") {
		t.Errorf("ready: issue 3 should still appear:\n%s", readyOut)
	}

	// --- Comment on issue 1 ---
	runComment(t, wd, "1", "Working on the foundation now")

	// --- Update issue 1 status to done ---
	runUpdate(t, wd, "PROJ-0001", "--status", "done")

	// --- Release issue 1 (any user can release) ---
	runRelease(t, wd, "1")

	// --- Ready: now issue 2 is unblocked; issue 3 still there ---
	readyOut = runReady(t, wd)
	if !strings.Contains(readyOut, "Build on Foundation") {
		t.Errorf("ready: issue 2 should be unblocked after dep done:\n%s", readyOut)
	}
	if !strings.Contains(readyOut, "Independent Work") {
		t.Errorf("ready: issue 3 should still appear:\n%s", readyOut)
	}
	// Issue 1 is done — should not appear in ready.
	if strings.Contains(readyOut, "PROJ-0001") {
		t.Errorf("ready: done issue 1 should not appear:\n%s", readyOut)
	}

	// --- Show issue 1 with events, verify history ---
	showOut := runShow(t, wd, "--events", "1")
	for _, want := range []string{
		"created",
		"claimed",
		"comment",
		"status_changed",
		"released",
	} {
		if !strings.Contains(showOut, want) {
			t.Errorf("show --events: expected %q in output:\n%s", want, showOut)
		}
	}

	// --- Claim/release cycle on issue 2 ---
	runClaim(t, wd, "2", "--agent", "agent-beta", "--session", "sess-002")
	runUpdate(t, wd, "2", "--status", "in_progress")
	runRelease(t, wd, "2")

	// Issue 2 is open again (in_progress but no claim); verify in list.
	s2, err := cli.OpenStore(wd)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	issue2, _, err := s2.Show(2)
	s2.Close()
	if err != nil {
		t.Fatalf("Show issue 2: %v", err)
	}
	if issue2.Claim != nil {
		t.Errorf("issue 2 should have no claim after release, got %+v", issue2.Claim)
	}
	if issue2.Status != model.StatusInProgress {
		t.Errorf("issue 2 status: got %q, want in_progress", issue2.Status)
	}

	// --- Rebuild ---
	rebuildOut := runRebuild(t, wd)
	if !strings.Contains(rebuildOut, "3 issue(s)") {
		t.Errorf("rebuild summary unexpected: %q", rebuildOut)
	}

	// After rebuild, list and ready should remain consistent.
	listAfter := runList(t, wd)
	if !strings.Contains(listAfter, "Build on Foundation") {
		t.Errorf("list after rebuild missing issue 2:\n%s", listAfter)
	}
	if !strings.Contains(listAfter, "Independent Work") {
		t.Errorf("list after rebuild missing issue 3:\n%s", listAfter)
	}
}

// TestDependencyResolutionInReady verifies that the ready query correctly
// gates on dependency status, including the chain: dep must be done or
// cancelled, not merely in_progress.
func TestDependencyResolutionInReady(t *testing.T) {
	wd := initTracker(t, "T")

	runCreate(t, wd, "--title", "Blocker")
	runCreate(t, wd, "--title", "Dependent")

	// Set issue 2 to depend on issue 1.
	s, err := cli.OpenStore(wd)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	if err := s.Append(2, model.Event{
		Type:    model.EventDependenciesChanged,
		Depends: []int{1},
	}); err != nil {
		t.Fatalf("set dep: %v", err)
	}
	s.Close()

	// Issue 1 is open → issue 2 blocked.
	out := runReady(t, wd)
	if strings.Contains(out, "Dependent") {
		t.Errorf("issue 2 should be blocked while issue 1 is open:\n%s", out)
	}

	// Move issue 1 to in_progress → issue 2 still blocked.
	runUpdate(t, wd, "1", "--status", "in_progress")
	out = runReady(t, wd)
	if strings.Contains(out, "Dependent") {
		t.Errorf("issue 2 should be blocked while issue 1 is in_progress:\n%s", out)
	}

	// Complete issue 1 → issue 2 now ready.
	runUpdate(t, wd, "1", "--status", "done")
	out = runReady(t, wd)
	if !strings.Contains(out, "Dependent") {
		t.Errorf("issue 2 should be ready after dep done:\n%s", out)
	}
}

// TestClaimReleaseCycle verifies the full claim/release flow including
// event recording, index updates, and re-claimability after release.
func TestClaimReleaseCycle(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "Work item")

	// Claim the issue.
	runClaim(t, wd, "1", "--agent", "agent-1", "--session", "sess-1")

	s, err := cli.OpenStore(wd)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	issue, _, err := s.Show(1)
	s.Close()
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if issue.Claim == nil || issue.Claim.AgentID != "agent-1" {
		t.Fatalf("expected claim by agent-1, got %+v", issue.Claim)
	}

	// A second claim must fail.
	var buf strings.Builder
	if err := cli.RunClaim(wd, []string{"1", "--agent", "agent-2", "--session", "sess-2"}, &buf); err == nil {
		t.Error("expected error when claiming an already-claimed issue")
	}

	// Release the issue.
	runRelease(t, wd, "1")

	s2, err := cli.OpenStore(wd)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	issue, _, err = s2.Show(1)
	s2.Close()
	if err != nil {
		t.Fatalf("Show after release: %v", err)
	}
	if issue.Claim != nil {
		t.Errorf("expected no claim after release, got %+v", issue.Claim)
	}

	// Verify the released event records the previous claimant.
	s3, err := cli.OpenStore(wd)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	events, err := s3.Events(1)
	s3.Close()
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	var releaseEv *model.Event
	for i := range events {
		if events[i].Type == model.EventReleased {
			releaseEv = &events[i]
			break
		}
	}
	if releaseEv == nil {
		t.Fatal("no released event found")
	}
	if releaseEv.PreviousClaimant != "agent-1" {
		t.Errorf("PreviousClaimant: got %q, want agent-1", releaseEv.PreviousClaimant)
	}

	// Issue can be re-claimed after release.
	runClaim(t, wd, "1", "--agent", "agent-3", "--session", "sess-3")

	s4, err := cli.OpenStore(wd)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	issue, _, err = s4.Show(1)
	s4.Close()
	if err != nil {
		t.Fatalf("Show after re-claim: %v", err)
	}
	if issue.Claim == nil || issue.Claim.AgentID != "agent-3" {
		t.Errorf("expected re-claim by agent-3, got %+v", issue.Claim)
	}
}

// TestRebuildConsistency verifies that rebuild produces the same index state
// as incremental operations, across a variety of event types.
func TestRebuildConsistency(t *testing.T) {
	wd := initTracker(t, "T")

	// Build a rich state incrementally.
	runCreate(t, wd, "--title", "Alpha", "--priority", "high")
	runCreate(t, wd, "--title", "Beta", "--priority", "low", "--type", "issue")
	runCreate(t, wd, "--title", "Gamma", "--priority", "medium")

	runUpdate(t, wd, "1", "--status", "in_progress", "--title", "Alpha revised")
	runClaim(t, wd, "2", "--agent", "agent-x", "--session", "sess-x")
	runUpdate(t, wd, "3", "--status", "done")
	runComment(t, wd, "1", "Making progress")

	s, err := cli.OpenStore(wd)
	if err != nil {
		t.Fatalf("OpenStore (before): %v", err)
	}
	before, err := s.ListIssues(index.Filters{})
	issue2Before, _, err2 := s.Show(2)
	s.Close()
	if err != nil || err2 != nil {
		t.Fatalf("pre-rebuild reads: %v %v", err, err2)
	}

	runRebuild(t, wd)

	sAfter, err := cli.OpenStore(wd)
	if err != nil {
		t.Fatalf("OpenStore (after): %v", err)
	}
	after, err := sAfter.ListIssues(index.Filters{})
	issue1After, _, err1 := sAfter.Show(1)
	issue2After, _, err2 := sAfter.Show(2)
	sAfter.Close()
	if err != nil || err1 != nil || err2 != nil {
		t.Fatalf("post-rebuild reads: %v %v %v", err, err1, err2)
	}

	// Index row counts match.
	if len(before) != len(after) {
		t.Fatalf("issue count: before %d, after %d", len(before), len(after))
	}

	// Spot-check indexed fields.
	for i := range before {
		b, a := before[i], after[i]
		if b.ID != a.ID || b.Title != a.Title || b.Status != a.Status || b.Priority != a.Priority {
			t.Errorf("issue %d differs: before %+v after %+v", b.ID, b, a)
		}
	}

	// Claim state is preserved.
	if issue2Before.Claim == nil || issue2After.Claim == nil {
		t.Fatalf("claim missing: before %+v after %+v", issue2Before.Claim, issue2After.Claim)
	}
	if issue2Before.Claim.AgentID != issue2After.Claim.AgentID {
		t.Errorf("claim AgentID: before %q after %q", issue2Before.Claim.AgentID, issue2After.Claim.AgentID)
	}

	// Title change survived rebuild.
	if issue1After.Title != "Alpha revised" {
		t.Errorf("issue 1 title after rebuild: got %q, want 'Alpha revised'", issue1After.Title)
	}
}
