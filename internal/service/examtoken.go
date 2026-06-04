package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidToken is returned by VerifyExamToken when the token is malformed
// (bad shape / base64 / json). Handlers map it to 400 VALIDATION.
var ErrInvalidToken = errors.New("service: invalid exam token")

// ErrBadSignature is returned by VerifyExamToken when the token is well-formed
// but its HMAC signature does not match. Handlers map it to 401 UNAUTHORIZED.
var ErrBadSignature = errors.New("service: exam token signature mismatch")

// ExamPayload is the stateless exam-token body. It carries the question ids the
// exam was issued for plus the issue time and duration so grade can compute
// expiry without any server-side session. issued_at is unix seconds (UTC).
type ExamPayload struct {
	QuestionIDs []int64 `json:"question_ids"`
	IssuedAt    int64   `json:"issued_at"`
	DurationSec int     `json:"duration_sec"`
}

// SignExamToken serialises payload to JSON and returns
// base64url(json) + "." + hex(HMAC_SHA256(secret, base64url(json))).
//
// The MAC is computed over the base64url text (not the raw json) so verify can
// authenticate the exact bytes it received before any decode, avoiding any
// canonicalisation ambiguity.
func SignExamToken(secret string, payload ExamPayload) (string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("service.SignExamToken: marshal: %w", err)
	}
	body := base64.RawURLEncoding.EncodeToString(raw)
	mac := signBody(secret, body)
	return body + "." + mac, nil
}

// VerifyExamToken validates token's signature against secret and returns the
// decoded payload. It returns ErrInvalidToken for malformed input and
// ErrBadSignature when the signature does not match. Expiry is NOT checked here:
// a signature-valid token always decodes, and the caller (grade) decides how to
// treat an expired-but-authentic exam.
func VerifyExamToken(secret, token string) (ExamPayload, error) {
	body, sig, ok := strings.Cut(token, ".")
	if !ok || body == "" || sig == "" {
		return ExamPayload{}, ErrInvalidToken
	}

	want := signBody(secret, body)
	gotMAC, err := hex.DecodeString(sig)
	if err != nil {
		return ExamPayload{}, ErrInvalidToken
	}
	wantMAC, err := hex.DecodeString(want)
	if err != nil {
		return ExamPayload{}, fmt.Errorf("service.VerifyExamToken: decode want: %w", err)
	}
	// hmac.Equal is constant-time; guards against timing side channels.
	if !hmac.Equal(gotMAC, wantMAC) {
		return ExamPayload{}, ErrBadSignature
	}

	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return ExamPayload{}, ErrInvalidToken
	}
	var payload ExamPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ExamPayload{}, ErrInvalidToken
	}
	return payload, nil
}

// signBody returns the hex HMAC-SHA256 of body keyed by secret.
func signBody(secret, body string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(body))
	return hex.EncodeToString(h.Sum(nil))
}
