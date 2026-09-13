package serve

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv/taskstore"
)

func TestStorePersistsOwnershipAndUncertainExecution(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	s, err := OpenStore(root, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	a := ownedContext(context.Background(), "a")
	b := ownedContext(context.Background(), "b")
	task := &a2a.Task{ID: "task-a", ContextID: "context-a", Status: a2a.TaskStatus{State: a2a.TaskStateWorking}}
	version, err := s.Create(a, task)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(b, task.ID); !errors.Is(err, a2a.ErrTaskNotFound) {
		t.Fatalf("foreign read: %v", err)
	}
	if err := s.CheckContext(b, task.ContextID); !errors.Is(err, a2a.ErrTaskNotFound) {
		t.Fatalf("foreign context: %v", err)
	}
	if _, err := s.Create(b, &a2a.Task{ID: "task-b", ContextID: "context-a"}); !errors.Is(err, a2a.ErrTaskNotFound) {
		t.Fatalf("foreign context creation: %v", err)
	}
	if _, err := s.Update(b, &taskstore.UpdateRequest{Task: task, PrevVersion: version}); !errors.Is(err, a2a.ErrTaskNotFound) {
		t.Fatalf("foreign update: %v", err)
	}
	if list, err := s.List(b, &a2a.ListTasksRequest{}); err != nil || len(list.Tasks) != 0 {
		t.Fatalf("foreign list: %+v %v", list, err)
	}
	if _, err := OpenStore(root, 1<<20); err == nil {
		t.Fatal("second listener obtained exclusive state")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = OpenStore(root, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	saved, err := s.Get(a, "task-a")
	if err != nil {
		t.Fatal(err)
	}
	if saved.Task.Status.State != a2a.TaskStateFailed || saved.Task.Metadata["bench/outcome"] != "unknown" {
		t.Fatalf("restart lost uncertainty: %+v", saved.Task)
	}
	if _, err := s.Get(b, "task-a"); !errors.Is(err, a2a.ErrTaskNotFound) {
		t.Fatalf("ownership lost at restart: %v", err)
	}
}

func TestStoreCompletedAndWaitingTasksSurviveRestart(t *testing.T) {
	for _, state := range []a2a.TaskState{a2a.TaskStateCompleted, a2a.TaskStateInputRequired, a2a.TaskStateAuthRequired} {
		t.Run(string(state), func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "state")
			s, err := OpenStore(root, 1<<20)
			if err != nil {
				t.Fatal(err)
			}
			ctx := ownedContext(context.Background(), "owner")
			task := &a2a.Task{ID: "task", ContextID: "context", Status: a2a.TaskStatus{State: state}}
			if _, err := s.Create(ctx, task); err != nil {
				t.Fatal(err)
			}
			s.Close()
			s, err = OpenStore(root, 1<<20)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			saved, err := s.Get(ctx, "task")
			if err != nil || saved.Task.Status.State != state {
				t.Fatalf("got %+v %v", saved, err)
			}
		})
	}
}

func TestClosedStoreCannotWriteAfterReleasingOwnership(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	store, err := OpenStore(root, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	next, err := OpenStore(root, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	task := &a2a.Task{ID: "late", ContextID: "context", Status: a2a.TaskStatus{State: a2a.TaskStateSubmitted}}
	if _, err := store.Create(ownedContext(context.Background(), "owner"), task); err == nil {
		t.Fatal("closed listener wrote a late task")
	}
	if _, err := next.Get(ownedContext(context.Background(), "owner"), task.ID); err == nil {
		t.Fatal("new owner observed an unauthorized late write")
	}
}
