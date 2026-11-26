package nicegateway

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	sessiondto "github.com/nice-platform/example/agent/backend/internal/app/session/dto"
	sessionusecase "github.com/nice-platform/example/agent/backend/internal/app/session/usecase"
)

// UserInfo reads /oauth/userinfo with the sign-in token, whose aud is this client.
func (g *Gateway) UserInfo(ctx context.Context, signIn string) (sessiondto.UserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.Issuer+"/oauth/userinfo", nil)
	if err != nil {
		return sessiondto.UserInfo{}, err
	}
	req.Header.Set("Authorization", "Bearer "+signIn)
	resp, err := g.HTTP.Do(req)
	if err != nil {
		return sessiondto.UserInfo{}, &sessionusecase.UnavailableError{Err: err}
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return sessiondto.UserInfo{}, answerError(resp)
	}
	var body struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&body); err != nil {
		return sessiondto.UserInfo{}, &sessionusecase.UnavailableError{Err: err}
	}
	return sessiondto.UserInfo{Name: body.Name, Email: body.Email}, nil
}
