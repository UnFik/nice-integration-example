package callusecase

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	calldto "github.com/nice-platform/example/agent-backend/internal/app/call/dto"
	"github.com/nice-platform/example/agent-backend/internal/app/call/internal/approval"
	"github.com/nice-platform/example/agent-backend/internal/app/call/internal/failure"
	"github.com/nice-platform/example/agent-backend/internal/app/call/internal/key"
	"github.com/nice-platform/example/agent-backend/internal/domain"
)

const maxOperationKey = 200

// CallOperation runs one call of an operation for the user of c. The service and the operation are looked up in what
// NICE lists for the user at this moment, never taken from in. A sensitive operation returns an ApprovalRequest
// until ap carries the state this agent issued for the same user and call, and the user's acceptance. Exactly one of
// the three results is set.
func (s *Service) CallOperation(ctx context.Context, c calldto.Caller, in calldto.OperationInput, ap calldto.Approval) (*calldto.CallView, *calldto.ApprovalRequest, *calldto.Failure) {
	if !domain.ValidServiceSlug(in.Service) {
		return nil, nil, failure.Invalid("service must be the slug of a service from list_services")
	}
	if in.Operation == "" || len(in.Operation) > maxOperationKey {
		return nil, nil, failure.Invalid("operation must be the key of an operation from list_operations")
	}
	t := domain.Target(in.Service)
	k, bad := key.For(c, t, in.Where)
	if bad != nil {
		return nil, nil, bad
	}

	m, err := s.Discoverer.Services(ctx, c.Token)
	if err != nil {
		return nil, nil, failure.Discovery("", err, s.ClientVars)
	}
	svc, ok := findService(m, in.Service)
	if !ok {
		return nil, nil, &calldto.Failure{Code: "service_not_allowed",
			Message: fmt.Sprintf("the user may not call %s in any project, or it is not a nice_token service; see list_services", in.Service)}
	}
	org, prj, f := locate(m, c, in.Service, k)
	if f != nil {
		return nil, nil, f
	}
	if !domain.SecureEndpoint(svc.Endpoint) {
		return nil, nil, &calldto.Failure{Code: "insecure_endpoint",
			Message: fmt.Sprintf("the catalog endpoint of %s is plain http to a host that is not a loopback, so no token is sent to it; the operator must register an https endpoint", in.Service)}
	}

	ops, err := s.Discoverer.Operations(ctx, c.Token, in.Service)
	if err != nil {
		return nil, nil, failure.Discovery(t, err, s.ClientVars)
	}
	i := slices.IndexFunc(ops.Operations, func(o calldto.Operation) bool { return o.Key == in.Operation })
	if i < 0 {
		return nil, nil, failure.Invalid(fmt.Sprintf("%s has no operation %q the user may call; see list_operations", in.Service, in.Operation))
	}
	op := ops.Operations[i]
	query := map[string][]string{}
	for name, v := range in.Query {
		if !slices.ContainsFunc(op.RequestFields, func(f calldto.RequestField) bool { return f.Location == "query" && f.Name == name }) {
			return nil, nil, failure.Invalid(fmt.Sprintf("query %q is not a query field of %s", name, op.Key))
		}
		query[name] = []string{v}
	}
	var body []byte
	if in.Body != nil {
		if body, err = json.Marshal(in.Body); err != nil {
			return nil, nil, failure.Invalid("the body cannot be encoded as JSON")
		}
	}
	call := calldto.OperationCall{Service: svc, Operation: op, Org: k.Org, Project: k.Project,
		PathParams: in.PathParams, Query: query, Body: body}

	if op.Sensitive {
		ask, f := s.approve(c, in, ap, svc, op, org, prj)
		if ask != nil || f != nil {
			return nil, ask, f
		}
	}

	res, err := s.Operator.CallOperation(ctx, c.Token, call)
	if err != nil {
		f := failure.Exchange(t, err, s.ClientVars)
		s.Log.Warn("tool call failed", slog.String("service", in.Service), slog.String("operation", op.Key),
			slog.String("user", c.Subject), slog.String("code", f.Code))
		return nil, nil, f
	}
	if res.Status < 200 || res.Status > 299 {
		f := failure.Service(t, res.Status, res.Body)
		s.Log.Info("service refused", slog.String("service", in.Service), slog.String("operation", op.Key),
			slog.Int("status", res.Status), slog.String("code", f.Code))
		return nil, nil, f
	}
	out := &calldto.CallView{Status: res.Status}
	if len(res.Body) > 0 {
		if err := json.Unmarshal(res.Body, &out.Body); err != nil {
			return nil, nil, &calldto.Failure{Code: "bad_answer", Message: in.Service + " answered something other than JSON"}
		}
	}
	return out, nil, nil
}

// approve asks for an approval, or redeems the one ap carries. It returns neither when the call may go ahead.
func (s *Service) approve(c calldto.Caller, in calldto.OperationInput, ap calldto.Approval, svc calldto.MapService, op calldto.Operation, org, prj string) (*calldto.ApprovalRequest, *calldto.Failure) {
	// The fingerprint covers everything that decides what the call does, as the model gave it.
	digest, err := approval.Digest(map[string]any{"service": in.Service, "operation": in.Operation, "org": in.Where.Org,
		"project": in.Where.Project, "path": in.PathParams, "query": in.Query, "body": in.Body})
	if err != nil {
		return nil, failure.Invalid("the arguments cannot be encoded")
	}
	ask := func() *calldto.ApprovalRequest {
		return &calldto.ApprovalRequest{State: s.book().Issue(c.Subject, digest), Message: approvalMessage(svc, op, in.PathParams, org, prj)}
	}
	if ap.State == "" {
		return ask(), nil
	}
	// A state for another user or call, an old one, or one already used: ask again rather than act on it.
	if s.book().Redeem(ap.State, c.Subject, digest) != nil {
		return ask(), nil
	}
	if !ap.Accepted {
		return nil, failure.NotApproved()
	}
	return nil, nil
}

// approvalMessage is what the user reads. Names and the summary come from NICE and are cut to plain printable text.
func approvalMessage(svc calldto.MapService, op calldto.Operation, params map[string]string, org, prj string) string {
	path := op.Path
	for name, v := range params {
		path = strings.ReplaceAll(path, "{"+name+"}", v)
	}
	what := failure.Plain(op.Summary)
	if what == "" {
		what = op.Key
	}
	name := failure.Plain(svc.Name)
	if name == "" {
		name = svc.Slug
	}
	return fmt.Sprintf("Approve a sensitive operation of %s (%s): %s. %s %s in %s / %s.",
		name, svc.Slug, what, op.Method, failure.Plain(path), failure.Plain(org), failure.Plain(prj))
}

func findService(m calldto.ServiceMap, slug string) (calldto.MapService, bool) {
	for _, s := range m.Services {
		if s.Slug == slug {
			return s, true
		}
	}
	return calldto.MapService{}, false
}

// locate checks that a named context is one the map lists for the service, and returns the names of the
// organization and project for the approval message. The context of the user's token is left to NICE.
func locate(m calldto.ServiceMap, c calldto.Caller, slug string, k calldto.Key) (org, prj string, f *calldto.Failure) {
	orgID, prjID := k.Org, k.Project
	if orgID == "" {
		orgID = c.OrgID
		if prjID == "" {
			prjID = c.ProjectID
		}
	}
	org, prj = orgID, prjID
	if prj == "" {
		prj = "the project of the user's token"
	}
	for _, o := range m.Organizations {
		if o.ID != orgID {
			continue
		}
		if o.Name != "" {
			org = o.Name
		}
		for _, p := range o.Projects {
			if p.ID == prjID && slices.Contains(p.Services, slug) {
				if p.Name != "" {
					prj = p.Name
				}
				return org, prj, nil
			}
		}
		// The context of the token is left to NICE; and when NICE listed only part of the projects, the map cannot
		// refuse: NICE decides on the exchange.
		if k.Project == "" || o.ProjectsTruncated {
			return org, prj, nil
		}
	}
	if k.Project == "" {
		return org, prj, nil
	}
	return "", "", failure.Invalid(fmt.Sprintf("project %s is not listed for %s in organization %s; see list_services", k.Project, slug, orgID))
}
