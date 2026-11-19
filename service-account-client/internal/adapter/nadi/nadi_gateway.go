// Package nadi calls the device API of nadi as the Service Account. It sends through the *http.Client of the SDK
// (credentials.Client.HTTPClient), which proves the API key to NICE, keeps and renews the token, and adds it to every
// request; this package only reads the failures of the SDK, as the only one that knows niceauth/credentials.
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

	"github.com/nice-platform/nice/pkg/niceauth/credentials"

	"github.com/nice-platform/example/service-account-client/internal/domain"
)

const maxBody = 1 << 20

// Gateway fills the Gateway contract of the device use cases.
type Gateway struct {
	base string
	http *http.Client
}

// New makes the gateway of the nadi at base. hc must add the token of nadi to every request, as the client of
// credentials.Client.HTTPClient made for base does; that client refuses to send the token to any other host.
func New(base string, hc *http.Client) *Gateway { return &Gateway{base: base, http: hc} }

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

// send makes one call and decodes a 2xx answer into out. The client adds the Bearer token and, when nadi answers 401
// token_stale or expired, gets a new token and sends the request once more. The API key never comes near this
// request: only the SDK holds it, and only for the token endpoint of NICE.
func (g *Gateway) send(ctx context.Context, method, path string, body, out any) error {
	var rd io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		// A bytes.Reader gives the request a GetBody, so the client can send it again after token_stale.
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
	resp, err := g.http.Do(req)
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

// domainError names a failure of the SDK in the terms of the use cases. http.Client wraps every error in a
// *url.Error, a failure to get a token included, so the SDK errors are looked for first.
func domainError(err error) error {
	var grant *credentials.GrantError
	var urlErr *url.Error
	switch {
	case errors.As(err, &grant):
		return &domain.GrantError{Code: grant.Code, Description: grant.Description, Permanent: grant.Permanent}
	case credentials.IsTemporary(err):
		return domain.ErrTemporary
	case errors.As(err, &urlErr):
		return domain.ErrUnreachable
	default:
		return err
	}
}
