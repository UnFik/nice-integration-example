package chatusecase

import (
	"context"

	calldto "github.com/nice-platform/example/agent/backend/internal/app/call/dto"
	chatdto "github.com/nice-platform/example/agent/backend/internal/app/chat/dto"
	"github.com/nice-platform/example/agent/backend/internal/app/chat/internal/turn"
	"github.com/nice-platform/example/agent/backend/internal/domain"
)

// AnswerApproval gives the user's answer to a call of a conversation of the user of c that waits for approval. The
// call runs again with the stored state, so the call use case decides: an accepted, valid state sends the operation
// once; a refusal sends nothing; an expired or foreign state asks again. When no call of the reply waits any more,
// the turn goes on. Errors before anything runs (chat off, unknown conversation or call, a turn running) are
// returned without an event.
func (s *Service) AnswerApproval(ctx context.Context, c calldto.Caller, conversationID, callID string, approve bool, emit func(chatdto.Event)) error {
	if s.Model == nil {
		return domain.ErrChatOff
	}
	release, ok := s.locks.TryLock(conversationID)
	if !ok {
		return domain.ErrTurnInProgress
	}
	defer release()
	_, msgs, err := s.Conversations.Get(ctx, c.Subject, conversationID)
	if err != nil {
		return err
	}
	var call *chatdto.ToolCall
	pending := turn.Pending(msgs)
	for i := range pending {
		if pending[i].ID == callID {
			call = &pending[i]
		}
	}
	if call == nil {
		return domain.ErrCallNotPending
	}
	out := s.Tools.Run(ctx, c, *call, &calldto.Approval{State: call.ApprovalState, Accepted: approve})
	if out.Approval != nil {
		if err := s.awaitApproval(ctx, c, conversationID, *call, out.Approval, emit); err != nil {
			s.fail(emit, err)
			return nil
		}
		emit(chatdto.Event{Type: eventDone, Code: donePending})
		return nil
	}
	if err := s.finishCall(ctx, c, conversationID, *call, out.Content, out.IsError, emit); err != nil {
		s.fail(emit, err)
		return nil
	}
	if len(pending) > 1 {
		emit(chatdto.Event{Type: eventDone, Code: donePending})
		return nil
	}
	s.loop(ctx, c, conversationID, emit)
	return nil
}
