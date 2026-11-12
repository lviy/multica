package main

import (
	"testing"

	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/handler"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestNotificationWorkspaceReviews(t *testing.T) {
	for _, tc := range []struct {
		name                                                              string
		opted, subscribed, self, muted, removed, foreign, parent, created bool
		status                                                            string
		want                                                              int
	}{
		{name: "unsubscribed opt in", opted: true, status: "in_review", want: 1},
		{name: "default off", status: "in_review"},
		{name: "created in review", opted: true, created: true, status: "in_review", want: 1},
		{name: "ordinary subscription unchanged", subscribed: true, status: "in_review", want: 1},
		{name: "parent subscription deduplicated", opted: true, parent: true, status: "in_review", want: 1},
		{name: "subscriber deduplicated", opted: true, subscribed: true, status: "in_review", want: 1},
		{name: "own transition", opted: true, self: true, status: "in_review", want: 1},
		{name: "independent of ordinary mute", opted: true, muted: true, status: "in_review", want: 1},
		{name: "removed member", opted: true, removed: true, status: "in_review"},
		{name: "another workspace", opted: true, foreign: true, status: "in_review"},
		{name: "not a review transition", opted: true, status: "in_progress"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fx := workspaceFixture(t)
			recipient := fx.User(t, "Review watcher", "review-watcher@example.test")
			member := fx.Member(t, testWorkspaceID, recipient, "member")
			cols := testutil.Cols{"status": tc.status}
			if tc.parent {
				parent := fx.Issue(t, "Parent", nil)
				cols["parent_issue_id"] = parent
				addTestSubscriber(t, parent, "member", recipient, "manual")
			}
			issue := fx.Issue(t, "Review gate", cols)
			fx.Cleanup(t, `DELETE FROM inbox_item WHERE issue_id=$1`, issue)
			fx.Cleanup(t, `DELETE FROM notification_preference WHERE user_id=$1`, recipient)
			if tc.subscribed {
				addTestSubscriber(t, issue, "member", recipient, "manual")
			}
			prefs := `{}`
			if tc.opted {
				prefs = `{"workspace_reviews":"all"}`
			}
			if tc.muted {
				prefs = `{"workspace_reviews":"all","status_changes":"muted"}`
			}
			ws := testWorkspaceID
			if tc.foreign {
				ws = fx.Workspace(t, "Other review workspace", "review-other")
			}
			fx.Exec(t, `INSERT INTO notification_preference(workspace_id,user_id,preferences) VALUES($1,$2,$3)`, ws, recipient, prefs)
			if tc.removed {
				fx.Exec(t, `DELETE FROM member WHERE id=$1`, member)
			}
			q := db.New(testPool)
			bus := newNotificationBus(t, q)
			actor := testUserID
			if tc.self {
				actor = recipient
			}
			publish := func(from, to string, changed bool) {
				bus.Publish(events.Event{Type: protocol.EventIssueUpdated, WorkspaceID: testWorkspaceID, ActorType: "member", ActorID: actor,
					Payload: map[string]any{"status_changed": changed, "prev_status": from, "issue": handler.IssueResponse{ID: issue, WorkspaceID: testWorkspaceID, Status: to, Title: "Review gate"}}})
			}
			if tc.created {
				bus.Publish(events.Event{Type: protocol.EventIssueCreated, WorkspaceID: testWorkspaceID, ActorType: "member", ActorID: actor,
					Payload: map[string]any{"issue": handler.IssueResponse{ID: issue, WorkspaceID: testWorkspaceID, Status: tc.status, Title: "Review gate"}}})
			} else {
				publish("in_progress", tc.status, true)
			}
			count := func() int {
				n := 0
				for _, item := range inboxItemsForRecipient(t, q, recipient) {
					if item.IssueID == parseUUID(issue) && item.Type == "status_changed" {
						n++
					}
				}
				return n
			}
			if got := count(); got != tc.want {
				t.Fatalf("items=%d want=%d", got, tc.want)
			}
			if tc.want == 1 {
				publish(tc.status, tc.status, false)
				if got := count(); got != 1 {
					t.Fatalf("unchanged update duplicated notification: %d", got)
				}
				publish("in_review", "in_progress", true)
				before := count()
				publish("in_progress", "in_review", true)
				if got := count(); got != before+1 {
					t.Fatalf("resubmission items=%d want=%d", got, before+1)
				}
			}
		})
	}
}
