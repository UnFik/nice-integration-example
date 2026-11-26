package niceexchange

import (
	"context"

	"github.com/ABA-Developer/nice-auth-sdk/credentials"

	calldto "github.com/nice-platform/example/agent/backend/internal/app/call/dto"
)

// OperationSchemas reads the full schemas of the operations keys of slug for the user of token. The SDK asks NICE
// 20 keys at a time with the delegated token of the NICE API.
func (c *Client) OperationSchemas(ctx context.Context, token, slug string, keys []string) (calldto.OperationSchemas, error) {
	s, err := c.x.OperationSchemas(ctx, token, slug, keys)
	if err != nil {
		return calldto.OperationSchemas{}, domainError(err)
	}
	out := calldto.OperationSchemas{Service: s.Service, Schemas: []calldto.OperationSchema{}, Missing: s.Missing}
	for _, o := range s.Schemas {
		out.Schemas = append(out.Schemas, calldto.OperationSchema{Key: o.Key, InputSchema: o.InputSchema, Responses: o.Responses,
			Truncated: o.Truncated})
	}
	return out, nil
}

// Redact removes from body the properties that the response schema of status marks x-secret.
func (c *Client) Redact(schema calldto.OperationSchema, status int, body []byte) ([]byte, error) {
	s := credentials.OperationSchema{Key: schema.Key, InputSchema: schema.InputSchema, Responses: schema.Responses}
	return credentials.RedactSecrets(s.ResponseSchema(status), body)
}
