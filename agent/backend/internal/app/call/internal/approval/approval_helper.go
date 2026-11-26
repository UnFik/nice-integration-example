// Package approval issues and redeems the state that binds the approval of a sensitive operation to one user and one
// call. The MCP server is stateless, so the state travels through the client: it is signed, it expires, and it is
// good for one call only.
package approval

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"
)

// TTL is how long a user has to answer an approval.
const TTL = 5 * time.Minute

// ErrInvalid means the state is not one this agent issued for this user and call, or it expired or was used.
var ErrInvalid = errors.New("the approval does not match this call")

// Book signs states with a key that lives as long as the process and remembers the nonces it redeemed until they
// expire. A restart drops every pending approval, which only means the user is asked again.
type Book struct {
	key []byte
	now func() time.Time

	mu   sync.Mutex
	used map[string]time.Time
}

// New makes a book with a fresh random key. now is the clock; nil means time.Now.
func New(now func() time.Time) *Book {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic(err)
	}
	if now == nil {
		now = time.Now
	}
	return &Book{key: key, now: now, used: map[string]time.Time{}}
}

// A state is base64url(exp || nonce || bind) "." base64url(HMAC(key, that)), where exp is 8 bytes of Unix time, the
// nonce 16 random bytes, and bind the SHA-256 of the subject and the digest of the call: nothing in it is readable.
const (
	expLen   = 8
	nonceLen = 16
	bindLen  = sha256.Size
)

func bind(subject, digest string) []byte {
	sum := sha256.Sum256([]byte(subject + "\x00" + digest))
	return sum[:]
}

// Digest is the fingerprint of a call: the JSON of v, whose maps encoding/json writes with sorted keys.
func Digest(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

// Issue returns the state for an approval of the call with digest by subject.
func (b *Book) Issue(subject, digest string) string {
	raw := make([]byte, expLen+nonceLen, expLen+nonceLen+bindLen)
	binary.BigEndian.PutUint64(raw, uint64(b.now().Add(TTL).Unix()))
	if _, err := rand.Read(raw[expLen:]); err != nil {
		panic(err)
	}
	body := base64.RawURLEncoding.EncodeToString(append(raw, bind(subject, digest)...))
	return body + "." + b.sign(body)
}

// Redeem accepts state once, for subject and the call with digest, before it expires.
func (b *Book) Redeem(state, subject, digest string) error {
	body, sig, ok := strings.Cut(state, ".")
	if !ok || !hmac.Equal([]byte(sig), []byte(b.sign(body))) {
		return ErrInvalid
	}
	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil || len(raw) != expLen+nonceLen+bindLen {
		return ErrInvalid
	}
	now := b.now()
	exp := time.Unix(int64(binary.BigEndian.Uint64(raw[:expLen])), 0)
	if !now.Before(exp) || !hmac.Equal(raw[expLen+nonceLen:], bind(subject, digest)) {
		return ErrInvalid
	}
	nonce := string(raw[expLen : expLen+nonceLen])
	b.mu.Lock()
	defer b.mu.Unlock()
	for n, e := range b.used {
		if !now.Before(e) {
			delete(b.used, n)
		}
	}
	if _, seen := b.used[nonce]; seen {
		return ErrInvalid
	}
	b.used[nonce] = exp
	return nil
}

func (b *Book) sign(body string) string {
	m := hmac.New(sha256.New, b.key)
	m.Write([]byte(body))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}
