// Package nadi calls the device API of nadi as the Service Account. It is the only package that knows niceclient:
// the SDK proves the API key to NICE, keeps and renews the token, and adds it to every request.
package nadi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/nice-platform/nice/pkg/niceclient"

	"github.com/nice-platform/example/service-account-client/internal/domain"
)

const maxBody = 1 << 20

// Doer is the part of niceclient.Client the gateway uses.
type Doer interface {
	Do(ctx context.Context, key niceclient.Key, req *http.Request) (*http.Response, error)
}

// Gateway fills the Gateway contract of the device use cases.
type Gateway struct {
	base string
	nice Doer
}

// New makes the gateway of the nadi at base.
func New(base string, nice Doer) *Gateway { return &Gateway{base: base, nice: nice} }

// key names the token of every call. A Service Account needs no subject, organization or project here: the SDK
// fills its id, and NICE always issues the token for the project the account belongs to.
var key = niceclient.Key{Audience: domain.ServiceSlug}

// device is a device as nadi writes it.
type device struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	SerialNumber string    `json:"serial_number"`
	Status       string    `json:"status"`
	Location     string    `json:"location"`
	CreatedBy    string    `json:"created_by"`
	CreateTime   time.Time `json:"create_time"`
	UpdateTime   time.Time `json:"update_time"`
}

func (d device) domain() domain.Device {
	return domain.Device{ID: d.ID, Name: d.Name, SerialNumber: d.SerialNumber, Status: d.Status, Location: d.Location,
		CreatedBy: d.CreatedBy, CreateTime: d.CreateTime, UpdateTime: d.UpdateTime}
}

// send makes one call and decodes a 2xx answer into out. Do adds the Bearer token and, when nadi answers 401
// token_stale or expired, gets a new token and sends the request once more. The API key never comes near this
// request: only the SDK holds it, and only for the token endpoint of NICE.
func (g *Gateway) send(ctx context.Context, method, path string, body, out any) error {
	var rd io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		// A bytes.Reader gives the request a GetBody, so Do can send it again after token_stale.
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, g.base+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := g.nice.Do(ctx, key, req)
	if err != nil {
		return domainError(err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return domain.ErrUnreachable
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		// Only the code and the description of the refusal are kept, never the raw body.
		var refusal struct {
			Error       string `json:"error"`
			Description string `json:"error_description"`
		}
		_ = json.Unmarshal(raw, &refusal)
		return &domain.ServiceError{Status: resp.StatusCode, Code: refusal.Error, Description: refusal.Description}
	}
	if json.Unmarshal(raw, out) != nil {
		return domain.ErrBadAnswer
	}
	return nil
}

// domainError names a failure of the SDK in the terms of the use cases.
func domainError(err error) error {
	var grant *niceclient.GrantError
	var urlErr *url.Error
	switch {
	case errors.As(err, &grant):
		return &domain.GrantError{Code: grant.Code, Description: grant.Description, Permanent: grant.Permanent}
	case niceclient.IsTemporary(err):
		return domain.ErrTemporary
	case errors.As(err, &urlErr):
		return domain.ErrUnreachable
	default:
		return err
	}
}
