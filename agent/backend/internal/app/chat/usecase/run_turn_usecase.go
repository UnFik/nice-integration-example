package chatusecase

import (
	"context"
	"errors"
	"fmt"

	calldto "github.com/nice-platform/example/agent/backend/internal/app/call/dto"
	chatdto "github.com/nice-platform/example/agent/backend/internal/app/chat/dto"
	"github.com/nice-platform/example/agent/backend/internal/app/chat/internal/turn"
	"github.com/nice-platform/example/agent/backend/internal/domain"
)

// Event types of a turn.
const (
	eventAssistant  = "assistant"
	eventToolCall   = "tool_call"
	eventToolResult = "tool_result"
	eventApproval   = "approval"
	eventDone       = "done"
	eventError      = "error"
)

// Reasons a turn ends with, in the Code of its done event.
const (
	doneAnswered = "answered"
	donePending  = "awaiting_approval"
	doneLimit    = "step_limit"
)

// RunTurn stores the user's text in a conversation of the user of c and runs one turn of the model on it. An error
// before the turn starts (chat off, unknown conversation, invalid text, a turn already running) is returned and
// nothing is emitted. Once it starts, every step is stored and then emitted, and the turn ends with a done or an
// error event; RunTurn then returns nil.
func (s *Service) RunTurn(ctx context.Context, c calldto.Caller, conversationID, text string, emit func(chatdto.Event)) error {
	if s.Model == nil {
		return domain.ErrChatOff
	}
	if err := domain.CheckMessage(text); err != nil {
		return err
	}
	release, ok := s.locks.TryLock(conversationID)
	if !ok {
		return domain.ErrTurnInProgress
	}
	defer release()
	conv, msgs, err := s.Conversations.Get(ctx, c.Subject, conversationID)
	if err != nil {
		return err
	}
	// A call still waiting when the user writes again is declined: the model must see a result for every call.
	for _, call := range turn.Pending(msgs) {
		if err := s.finishCall(ctx, c, conversationID, call, notApprovedContent("the user sent a new message instead of answering"), true, emit); err != nil {
			return err
		}
	}
	if conv.Title == "" {
		if err := s.Conversations.Rename(ctx, c.Subject, conversationID, domain.TitleFrom(text), s.now()); err != nil {
			return err
		}
	}
	if _, err := s.Conversations.Append(ctx, c.Subject, conversationID, chatdto.Message{
		ID: s.id(), Role: domain.RoleUser, Parts: []chatdto.Part{{Kind: domain.PartText, Text: text}}, CreatedAt: s.now(),
	}); err != nil {
		return err
	}
	s.loop(ctx, c, conversationID, emit)
	return nil
}

// loop calls the model and runs the tools it asks for until it answers without a call, a call waits for approval,
// or MaxSteps model calls were made. Every failure becomes an error event.
func (s *Service) loop(ctx context.Context, c calldto.Caller, conversationID string, emit func(chatdto.Event)) {
	steps := max(s.MaxSteps, 1)
	for range steps {
		_, msgs, err := s.Conversations.Get(ctx, c.Subject, conversationID)
		if err != nil {
			s.fail(emit, err)
			return
		}
		reply, err := s.Model.Next(ctx, chatdto.ModelRequest{System: turn.System(c), Messages: turn.ForModel(msgs), Tools: s.Tools.Specs()})
		if err != nil {
			s.fail(emit, err)
			return
		}
		m := chatdto.Message{ID: s.id(), Role: domain.RoleAssistant, CreatedAt: s.now()}
		if reply.Text != "" {
			m.Parts = append(m.Parts, chatdto.Part{Kind: domain.PartText, Text: reply.Text})
		}
		for _, call := range reply.Calls {
			call.Status, call.ApprovalState, call.ApprovalMessage = "", "", ""
			if call.ID == "" {
				call.ID = s.id()
			}
			m.Parts = append(m.Parts, chatdto.Part{Kind: domain.PartToolCall, Call: &call})
		}
		if len(m.Parts) == 0 {
			m.Parts = []chatdto.Part{{Kind: domain.PartText, Text: ""}}
		}
		stored, err := s.Conversations.Append(ctx, c.Subject, conversationID, m)
		if err != nil {
			s.fail(emit, err)
			return
		}
		emit(chatdto.Event{Type: eventAssistant, Message: &stored})
		if len(reply.Calls) == 0 {
			emit(chatdto.Event{Type: eventDone, Code: doneAnswered})
			return
		}
		pending := false
		for _, p := range stored.Parts {
			if p.Kind != domain.PartToolCall {
				continue
			}
			call := *p.Call
			emit(chatdto.Event{Type: eventToolCall, CallID: call.ID, Text: call.Name})
			out := s.Tools.Run(ctx, c, call, nil)
			if out.Approval != nil {
				if err := s.awaitApproval(ctx, c, conversationID, call, out.Approval, emit); err != nil {
					s.fail(emit, err)
					return
				}
				pending = true
				continue
			}
			if err := s.finishCall(ctx, c, conversationID, call, out.Content, out.IsError, emit); err != nil {
				s.fail(emit, err)
				return
			}
		}
		if pending {
			emit(chatdto.Event{Type: eventDone, Code: donePending})
			return
		}
	}
	m := chatdto.Message{ID: s.id(), Role: domain.RoleAssistant, CreatedAt: s.now(), Parts: []chatdto.Part{{Kind: domain.PartText,
		Text: fmt.Sprintf("Batas %d langkah per giliran tercapai, jadi giliran ini berhenti. Kirim pesan lagi untuk melanjutkan.", steps)}}}
	stored, err := s.Conversations.Append(ctx, c.Subject, conversationID, m)
	if err != nil {
		s.fail(emit, err)
		return
	}
	emit(chatdto.Event{Type: eventAssistant, Message: &stored})
	emit(chatdto.Event{Type: eventDone, Code: doneLimit})
}

// awaitApproval keeps the call waiting with the state and the catalog message, and tells the dashboard.
func (s *Service) awaitApproval(ctx context.Context, c calldto.Caller, conversationID string, call chatdto.ToolCall, req *calldto.ApprovalRequest, emit func(chatdto.Event)) error {
	call.Status, call.ApprovalState, call.ApprovalMessage = domain.CallAwaitingApproval, req.State, req.Message
	if err := s.Conversations.UpdateCall(ctx, c.Subject, conversationID, call); err != nil {
		return err
	}
	emit(chatdto.Event{Type: eventApproval, CallID: call.ID, Text: req.Message})
	return nil
}

// finishCall marks the call done and stores its result as a tool message.
func (s *Service) finishCall(ctx context.Context, c calldto.Caller, conversationID string, call chatdto.ToolCall, content []byte, isError bool, emit func(chatdto.Event)) error {
	call.Status, call.ApprovalState, call.ApprovalMessage = domain.CallDone, "", ""
	if err := s.Conversations.UpdateCall(ctx, c.Subject, conversationID, call); err != nil {
		return err
	}
	stored, err := s.Conversations.Append(ctx, c.Subject, conversationID, chatdto.Message{
		ID: s.id(), Role: domain.RoleTool, CreatedAt: s.now(),
		Parts: []chatdto.Part{{Kind: domain.PartToolResult, Result: &chatdto.ToolResult{CallID: call.ID, Content: content, IsError: isError}}},
	})
	if err != nil {
		return err
	}
	emit(chatdto.Event{Type: eventToolResult, CallID: call.ID, Message: &stored})
	return nil
}

func notApprovedContent(why string) []byte {
	return turn.ErrorContent("not_approved", "the user did not approve this operation; nothing was sent to the service: "+why)
}

// fail emits the error event of a turn. The text names the cause without the provider's details.
func (s *Service) fail(emit func(chatdto.Event), err error) {
	code, text := "internal", "the turn failed; try again"
	switch {
	case errors.Is(err, domain.ErrModelUnavailable):
		code, text = "model_unavailable", "the model is not available right now; try again later"
	case errors.Is(err, domain.ErrModelRefused):
		code, text = "model_refused", "the model refused the request; check the LLM settings of the agent"
	case errors.Is(err, domain.ErrConversationNotFound):
		code, text = "not_found", "the conversation no longer exists"
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		code, text = "timeout", "the turn took too long and stopped"
	}
	if code == "internal" || code == "model_refused" {
		s.log().Error("chat turn failed", "code", code, "err", err)
	}
	emit(chatdto.Event{Type: eventError, Code: code, Text: text})
}
