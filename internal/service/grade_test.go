package service

import (
	"testing"

	"github.com/tn00869679/QuizForge_Backend/internal/model"
	"github.com/tn00869679/QuizForge_Backend/internal/store"
)

// gradeFixture builds a small two-subject answer key. Subject 10 ("Law") owns
// questions 1,2; subject 20 ("Ethics") owns question 3.
func gradeFixture() ([]int64, map[int64]store.GradingRow) {
	ids := []int64{1, 2, 3}
	keys := map[int64]store.GradingRow{
		1: {ID: 1, Number: 1, SubjectID: 10, Subject: "Law", Answer: "A"},
		2: {ID: 2, Number: 2, SubjectID: 10, Subject: "Law", Answer: "B"},
		3: {ID: 3, Number: 3, SubjectID: 20, Subject: "Ethics", Answer: "C"},
	}
	return ids, keys
}

func ans(pairs ...any) []model.ExamAnswer {
	out := make([]model.ExamAnswer, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, model.ExamAnswer{QuestionID: int64(pairs[i].(int)), Selected: pairs[i+1].(string)})
	}
	return out
}

// TestGradeAnswers_ScoringMatrix verifies the core scoring rules that the exam
// result depends on: every issued question counts toward the total, only an
// exact answer-key match is correct, unanswered/illegal selections are wrong,
// and the score is round(correct/total*100). A regression in any of these would
// silently mis-grade real exams.
func TestGradeAnswers_ScoringMatrix(t *testing.T) {
	ids, keys := gradeFixture()

	tests := []struct {
		name        string
		answers     []model.ExamAnswer
		wantCorrect int
		wantScore   int
	}{
		{
			name:        "all correct -> 100",
			answers:     ans(1, "A", 2, "B", 3, "C"),
			wantCorrect: 3,
			wantScore:   100,
		},
		{
			name:        "all wrong -> 0",
			answers:     ans(1, "B", 2, "C", 3, "A"),
			wantCorrect: 0,
			wantScore:   0,
		},
		{
			name:        "partial 2 of 3 -> round(66.67)=67",
			answers:     ans(1, "A", 2, "B", 3, "A"),
			wantCorrect: 2,
			wantScore:   67,
		},
		{
			name:        "partial 1 of 3 -> round(33.33)=33",
			answers:     ans(1, "A"),
			wantCorrect: 1,
			wantScore:   33,
		},
		{
			name:        "unanswered questions count as wrong",
			answers:     nil,
			wantCorrect: 0,
			wantScore:   0,
		},
		{
			name:        "illegal selected never matches the key",
			answers:     ans(1, "Z", 2, "", 3, "X"),
			wantCorrect: 0,
			wantScore:   0,
		},
		{
			name:        "extra answer for an unissued question is ignored",
			answers:     ans(1, "A", 999, "A"),
			wantCorrect: 1,
			wantScore:   33,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gradeAnswers(ids, keys, tt.answers)

			if got.Total != len(ids) {
				t.Fatalf("Total = %d, want %d (every issued question counts)", got.Total, len(ids))
			}
			if got.Correct != tt.wantCorrect {
				t.Fatalf("Correct = %d, want %d", got.Correct, tt.wantCorrect)
			}
			if got.Score != tt.wantScore {
				t.Fatalf("Score = %d, want %d", got.Score, tt.wantScore)
			}
			if len(got.Items) != len(ids) {
				t.Fatalf("Items len = %d, want %d", len(got.Items), len(ids))
			}
		})
	}
}

// TestGradeAnswers_PerSubjectBreakdown checks that results split by subject in a
// stable order with per-subject scores, since the UI shows a per-subject result
// card. Subject 10 (2 questions) and 20 (1 question) must each report their own
// correct/total/score.
func TestGradeAnswers_PerSubjectBreakdown(t *testing.T) {
	ids, keys := gradeFixture()
	// q1 correct, q2 wrong (Law 1/2), q3 correct (Ethics 1/1).
	got := gradeAnswers(ids, keys, ans(1, "A", 2, "Z", 3, "C"))

	if len(got.PerSubject) != 2 {
		t.Fatalf("PerSubject len = %d, want 2", len(got.PerSubject))
	}

	law := got.PerSubject[0]
	if law.SubjectID != 10 || law.Subject != "Law" {
		t.Fatalf("first subject = %+v, want id 10 Law", law)
	}
	if law.Correct != 1 || law.Total != 2 || law.Score != 50 {
		t.Fatalf("Law breakdown = %+v, want correct 1 total 2 score 50", law)
	}

	ethics := got.PerSubject[1]
	if ethics.SubjectID != 20 || ethics.Subject != "Ethics" {
		t.Fatalf("second subject = %+v, want id 20 Ethics", ethics)
	}
	if ethics.Correct != 1 || ethics.Total != 1 || ethics.Score != 100 {
		t.Fatalf("Ethics breakdown = %+v, want correct 1 total 1 score 100", ethics)
	}
}

// TestGradeAnswers_MissingKeyCountsButNeverCorrect covers a question id whose key
// is absent (e.g. the question was deleted after the exam was issued): it must
// still count toward the total, report an empty answer, and never be correct, so
// a stale exam can still be graded deterministically rather than crashing.
func TestGradeAnswers_MissingKeyCountsButNeverCorrect(t *testing.T) {
	ids := []int64{1, 42}
	keys := map[int64]store.GradingRow{
		1: {ID: 1, Number: 1, SubjectID: 10, Subject: "Law", Answer: "A"},
		// 42 intentionally absent.
	}

	got := gradeAnswers(ids, keys, ans(1, "A", 42, "A"))

	if got.Total != 2 {
		t.Fatalf("Total = %d, want 2", got.Total)
	}
	if got.Correct != 1 {
		t.Fatalf("Correct = %d, want 1 (missing-key question cannot be correct)", got.Correct)
	}

	var missing model.ExamGradeItem
	for _, it := range got.Items {
		if it.QuestionID == 42 {
			missing = it
		}
	}
	if missing.QuestionID != 42 {
		t.Fatal("item for question 42 not found")
	}
	if missing.IsCorrect {
		t.Fatal("missing-key question must not be correct")
	}
	if missing.Answer != "" {
		t.Fatalf("missing-key answer = %q, want empty", missing.Answer)
	}
}
