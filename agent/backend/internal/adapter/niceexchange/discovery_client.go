package niceexchange

import (
	"container/list"
	"context"
	"sync"

	"github.com/ABA-Developer/nice-auth-sdk/credentials"

	calldto "github.com/nice-platform/example/agent/backend/internal/app/call/dto"
)

// maxKeptLists bounds the operation lists kept in memory, one per user and service.
const maxKeptLists = 1000

// Services reads the service map of the user of token. The SDK reads every page with one delegated token of the
// NICE API per subject token, and keeps nothing.
func (c *Client) Services(ctx context.Context, token string) (calldto.ServiceMap, error) {
	m, err := c.x.Services(ctx, token)
	if err != nil {
		return calldto.ServiceMap{}, domainError(err)
	}
	var out calldto.ServiceMap
	for _, s := range m.Services {
		out.Services = append(out.Services, calldto.MapService{Slug: s.Slug, Name: s.Name, Description: s.Description,
			Endpoint: s.Endpoint, BasePath: s.BasePath})
	}
	for _, o := range m.Organizations {
		org := calldto.MapOrganization{ID: o.ID, Name: o.Name, Slug: o.Slug, ProjectsTruncated: o.ProjectsTruncated}
		for _, p := range o.Projects {
			org.Projects = append(org.Projects, calldto.MapProject{ID: p.ID, Name: p.Name, Slug: p.Slug, Services: p.Services})
		}
		out.Organizations = append(out.Organizations, org)
	}
	return out, nil
}

// Operations reads the operations of the service slug whose permission the user of token holds. The last list of
// each user and service is kept with its ETag, and every read still asks NICE, which answers 304 while the list is
// the same: what the user may call is never taken from memory alone.
func (c *Client) Operations(ctx context.Context, token, user, slug string) (calldto.ServiceOperations, error) {
	key := user + "\x00" + slug
	kept, _ := c.lists.get(key)
	ops, changed, err := c.x.ServiceOperationsIfChanged(ctx, token, slug, kept.ETag)
	if err != nil {
		// A refusal, such as 404 once a permission is removed, must not leave the old list behind.
		c.lists.remove(key)
		return calldto.ServiceOperations{}, domainError(err)
	}
	if changed {
		kept = ops
		if ops.ETag != "" {
			c.lists.put(key, ops)
		} else {
			c.lists.remove(key)
		}
	}
	return operationsOf(kept), nil
}

func operationsOf(ops credentials.ServiceOperations) calldto.ServiceOperations {
	out := calldto.ServiceOperations{Service: ops.Service}
	for _, o := range ops.Operations {
		op := calldto.Operation{Key: o.Key, Method: o.Method, Path: o.Path, Summary: o.Summary, Description: o.Description,
			Permission: o.Permission, Tag: o.Tag, Sensitive: o.Sensitive}
		for _, h := range o.Headers {
			op.Headers = append(op.Headers, calldto.Header{Name: h.Name, Value: h.Value})
		}
		for _, f := range o.RequestFields {
			op.RequestFields = append(op.RequestFields, calldto.RequestField{Location: f.Location, Name: f.Name, Required: f.Required,
				Type: f.Type, Description: f.Description, Format: f.Format, Enum: f.Enum, EnumTruncated: f.EnumTruncated})
		}
		out.Operations = append(out.Operations, op)
	}
	return out
}

// keptLists is a bounded least recently used map of operation lists.
type keptLists struct {
	mu    sync.Mutex
	max   int
	order *list.List
	items map[string]*list.Element
}

type keptEntry struct {
	key string
	ops credentials.ServiceOperations
}

func newKeptLists(max int) *keptLists {
	return &keptLists{max: max, order: list.New(), items: map[string]*list.Element{}}
}

func (k *keptLists) get(key string) (credentials.ServiceOperations, bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	e, ok := k.items[key]
	if !ok {
		return credentials.ServiceOperations{}, false
	}
	k.order.MoveToFront(e)
	return e.Value.(*keptEntry).ops, true
}

func (k *keptLists) put(key string, ops credentials.ServiceOperations) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if e, ok := k.items[key]; ok {
		e.Value.(*keptEntry).ops = ops
		k.order.MoveToFront(e)
		return
	}
	k.items[key] = k.order.PushFront(&keptEntry{key: key, ops: ops})
	for k.order.Len() > k.max {
		last := k.order.Back()
		k.order.Remove(last)
		delete(k.items, last.Value.(*keptEntry).key)
	}
}

func (k *keptLists) remove(key string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if e, ok := k.items[key]; ok {
		k.order.Remove(e)
		delete(k.items, key)
	}
}

func (k *keptLists) len() int {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.order.Len()
}
