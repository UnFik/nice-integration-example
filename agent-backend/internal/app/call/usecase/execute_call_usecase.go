package callusecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	calldto "github.com/nice-platform/example/agent-backend/internal/app/call/dto"
	"github.com/nice-platform/example/agent-backend/internal/app/call/internal/failure"
	"github.com/nice-platform/example/agent-backend/internal/app/call/internal/key"
	"github.com/nice-platform/example/agent-backend/internal/domain"
)

// ExecuteCall sends one request to t on behalf of the user of c and returns the decoded JSON body on success, or a
// Failure that names the code and says what it means. Neither the token nor the raw body of a refusal ever reaches
// the Failure.
func (s *Service) ExecuteCall(ctx context.Context, c calldto.Caller, t domain.Target, where calldto.Where, method, path string, body any) (map[string]any, *calldto.Failure) {
	k, bad := key.For(c, t, where)
	if bad != nil {
		return nil, bad
	}
	req := calldto.Request{Method: method, URL: s.Bases[t] + path}
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, &calldto.Failure{Code: "invalid_argument", Message: "the arguments cannot be encoded"}
		}
		req.Body = raw
	}
	resp, err := s.Sender.Send(ctx, c.Token, k, req)
	switch {
	case errors.Is(err, domain.ErrBuildRequest):
		return nil, &calldto.Failure{Code: "internal", Message: "the request to " + string(t) + " cannot be built"}
	case errors.Is(err, domain.ErrIncompleteAnswer):
		return nil, &calldto.Failure{Code: "service_unavailable", Message: string(t) + " did not finish its answer; try again"}
	case err != nil:
		f := failure.Exchange(t, err, s.ClientVars)
		s.Log.Warn("tool call failed", slog.String("service", string(t)), slog.String("user", c.Subject), slog.String("code", f.Code))
		return nil, f
	}
	if resp.Status < 200 || resp.Status > 299 {
		f := failure.Service(t, resp.Status, resp.Body)
		s.Log.Info("service refused", slog.String("service", string(t)), slog.Int("status", resp.Status), slog.String("code", f.Code))
		return nil, f
	}
	var out map[string]any
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, &calldto.Failure{Code: "bad_answer", Message: fmt.Sprintf("%s answered something other than a JSON object", t)}
	}
	return out, nil
}
