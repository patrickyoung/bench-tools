package serve

import (
	"context"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
)

type admission struct {
	a2asrv.PassthroughCallInterceptor
	store *Store
}

func (a *admission) Before(ctx context.Context, call *a2asrv.CallContext, req *a2asrv.Request) (context.Context, any, error) {
	owner, err := principal(ctx)
	if err != nil {
		return ctx, nil, err
	}
	if call.Tenant() != "" {
		return ctx, nil, a2a.ErrUnauthorized
	}
	call.User = a2asrv.NewAuthenticatedUser(owner, nil)
	var id a2a.TaskID
	switch p := req.Payload.(type) {
	case *a2a.SendMessageRequest:
		if p.Message == nil || p.Message.ID == "" || p.Message.Role != a2a.MessageRoleUser || len(p.Message.Parts) == 0 {
			return ctx, nil, a2a.ErrInvalidParams
		}
		for _, part := range p.Message.Parts {
			if part == nil || part.Content == nil {
				return ctx, nil, a2a.ErrInvalidParams
			}
		}
		if err := a.store.CheckContext(ctx, p.Message.ContextID); err != nil {
			return ctx, nil, err
		}
		for _, reference := range p.Message.ReferenceTasks {
			if _, err := a.store.Get(ctx, reference); err != nil {
				return ctx, nil, err
			}
		}
		id = p.Message.TaskID
	case *a2a.GetTaskRequest:
		id = p.ID
	case *a2a.CancelTaskRequest:
		id = p.ID
	case *a2a.SubscribeToTaskRequest:
		id = p.ID
	case *a2a.ListTasksRequest:
		if err := a.store.CheckContext(ctx, p.ContextID); err != nil {
			return ctx, nil, err
		}
	case *a2a.GetTaskPushConfigRequest:
		id = p.TaskID
	case *a2a.ListTaskPushConfigRequest:
		id = p.TaskID
	case *a2a.DeleteTaskPushConfigRequest:
		id = p.TaskID
	case *a2a.PushConfig:
		id = p.TaskID
	}
	if id != "" {
		stored, err := a.store.Get(ctx, id)
		if err != nil {
			return ctx, nil, err
		}
		if p, ok := req.Payload.(*a2a.SendMessageRequest); ok && p.Message.ContextID != "" && p.Message.ContextID != stored.Task.ContextID {
			return ctx, nil, a2a.ErrInvalidParams
		}
	}
	return ctx, nil, nil
}
