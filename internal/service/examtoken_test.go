package service

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// TestExamToken_SignVerifyRoundTrip ensures a freshly signed token verifies back
// to the exact payload — the happy path the whole stateless-exam design rests on.
func TestExamToken_SignVerifyRoundTrip(t *testing.T) {
	const secret = "unit-secret"
	want := ExamPayload{
		QuestionIDs: []int64{7, 3, 99, 1},
		IssuedAt:    time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC).Unix(),
		DurationSec: 7200,
	}

	token, err := SignExamToken(secret, want)
	if err != nil {
		t.Fatalf("SignExamToken: %v", err)
	}

	got, err := VerifyExamToken(secret, token)
	if err != nil {
		t.Fatalf("VerifyExamToken: %v", err)
	}
	if got.IssuedAt != want.IssuedAt || got.DurationSec != want.DurationSec {
		t.Fatalf("scalar mismatch: got %+v want %+v", got, want)
	}
	if len(got.QuestionIDs) != len(want.QuestionIDs) {
		t.Fatalf("question_ids len: got %d want %d", len(got.QuestionIDs), len(want.QuestionIDs))
	}
	for i := range want.QuestionIDs {
		if got.QuestionIDs[i] != want.QuestionIDs[i] {
			t.Fatalf("question_ids[%d]: got %d want %d", i, got.QuestionIDs[i], want.QuestionIDs[i])
		}
	}
}

// TestExamToken_RejectsTampering proves the HMAC actually protects the payload:
// any modification to the body or signature, a wrong key, or structural damage
// must fail verification. This is the security-critical property — a client must
// not be able to forge which questions an exam covers or replay under another
// key.
func TestExamToken_RejectsTampering(t *testing.T) {
	const secret = "unit-secret"
	base, err := SignExamToken(secret, ExamPayload{
		QuestionIDs: []int64{1, 2, 3},
		IssuedAt:    1000,
		DurationSec: 60,
	})
	if err != nil {
		t.Fatalf("SignExamToken: %v", err)
	}
	body, sig, _ := strings.Cut(base, ".")

	tests := []struct {
		name    string
		token   string
		secret  string
		wantErr error
	}{
		{
			name:    "tampered payload keeps old signature -> bad signature",
			token:   flipFirstByte(body) + "." + sig,
			secret:  secret,
			wantErr: ErrBadSignature,
		},
		{
			name:    "tampered signature -> bad signature",
			token:   body + "." + flipFirstByte(sig),
			secret:  secret,
			wantErr: ErrBadSignature,
		},
		{
			name:    "wrong secret -> bad signature",
			token:   base,
			secret:  "other-secret",
			wantErr: ErrBadSignature,
		},
		{
			name:    "missing dot separator -> invalid",
			token:   body + sig,
			secret:  secret,
			wantErr: ErrInvalidToken,
		},
		{
			name:    "empty signature segment -> invalid",
			token:   body + ".",
			secret:  secret,
			wantErr: ErrInvalidToken,
		},
		{
			name:    "non-hex signature -> invalid",
			token:   body + ".zzzz",
			secret:  secret,
			wantErr: ErrInvalidToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := VerifyExamToken(tt.secret, tt.token)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("got err %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// TestExamToken_ExpiredStillVerifies documents the deliberate split of concerns:
// Verify only authenticates; it must NOT reject an expired-but-authentic token.
// Expiry is grade's decision (it still grades and flags expired=true). If Verify
// started rejecting on time, expired exams could never be graded.
func TestExamToken_ExpiredStillVerifies(t *testing.T) {
	const secret = "unit-secret"
	token, err := SignExamToken(secret, ExamPayload{
		QuestionIDs: []int64{1},
		IssuedAt:    1, // far in the past
		DurationSec: 1,
	})
	if err != nil {
		t.Fatalf("SignExamToken: %v", err)
	}

	got, err := VerifyExamToken(secret, token)
	if err != nil {
		t.Fatalf("expired token must still verify, got err: %v", err)
	}
	if got.IssuedAt != 1 || got.DurationSec != 1 {
		t.Fatalf("payload not preserved: %+v", got)
	}
}

// flipFirstByte returns s with its first character changed, yielding a string of
// equal length that differs from the original. Used to corrupt a token segment.
func flipFirstByte(s string) string {
	if s == "" {
		return "x"
	}
	first := s[0]
	repl := byte('A')
	if first == 'A' {
		repl = 'B'
	}
	return string(repl) + s[1:]
}
