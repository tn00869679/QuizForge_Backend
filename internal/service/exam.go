package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/tn00869679/QuizForge_Backend/internal/model"
	"github.com/tn00869679/QuizForge_Backend/internal/store"
)

// ErrUnknownCategory is returned by Exam.Start when a category has no active
// questions to draw from (so a started exam would be empty).
var ErrUnknownCategory = errors.New("service: category has no questions")

// Default exam parameters, applied when the request omits them (zero value).
const (
	defaultPerSubjectN = 50
	defaultDurationSec  = 7200
)

// Exam serves exam/start (sampling + token issue) and exam/grade (verify +
// score). It is deliberately the only place that knows the HMAC secret.
type Exam struct {
	questions *store.Questions
	secret    string
	now       func() time.Time
}

// NewExam constructs an Exam service. secret keys the exam-token HMAC.
func NewExam(questions *store.Questions, secret string) *Exam {
	return &Exam{questions: questions, secret: secret, now: func() time.Time { return time.Now().UTC() }}
}

// Start samples up to perSubjectN active questions from each subject of the
// category, issues a signed exam token over the drawn ids, and returns the
// answer-free question set plus per-subject counts. perSubjectN/durationSec of 0
// fall back to the defaults. Returns ErrUnknownCategory when nothing was drawn.
func (s *Exam) Start(ctx context.Context, categoryID int64, perSubjectN, durationSec int) (model.ExamStartResponse, error) {
	if perSubjectN <= 0 {
		perSubjectN = defaultPerSubjectN
	}
	if durationSec <= 0 {
		durationSec = defaultDurationSec
	}

	drawn, err := s.questions.SampleByCategory(ctx, categoryID, perSubjectN)
	if err != nil {
		return model.ExamStartResponse{}, fmt.Errorf("service.Exam.Start: %w", err)
	}
	if len(drawn) == 0 {
		return model.ExamStartResponse{}, ErrUnknownCategory
	}

	ids := make([]int64, 0, len(drawn))
	public := make([]model.QuestionPublic, 0, len(drawn))
	for _, sq := range drawn {
		ids = append(ids, sq.Question.ID)
		public = append(public, sq.Question.ToPublic(false)) // exam mode: no answer/explanation
	}

	token, err := SignExamToken(s.secret, ExamPayload{
		QuestionIDs: ids,
		IssuedAt:    s.now().Unix(),
		DurationSec: durationSec,
	})
	if err != nil {
		return model.ExamStartResponse{}, fmt.Errorf("service.Exam.Start: %w", err)
	}

	return model.ExamStartResponse{
		ExamToken:   token,
		DurationSec: durationSec,
		Total:       len(drawn),
		PerSubject:  perSubjectCounts(drawn),
		Questions:   public,
	}, nil
}

// Grade verifies the exam token, loads the answer key for its question ids, and
// scores the submitted answers. A signature-valid but expired token is still
// graded, with expired=true in the response. Token errors (ErrInvalidToken /
// ErrBadSignature) are returned unwrapped for the handler to map to 400/401.
func (s *Exam) Grade(ctx context.Context, req model.ExamGradeRequest) (model.ExamGradeResponse, error) {
	payload, err := VerifyExamToken(s.secret, req.ExamToken)
	if err != nil {
		return model.ExamGradeResponse{}, err // ErrInvalidToken / ErrBadSignature
	}

	keys, err := s.questions.GradingByIDs(ctx, payload.QuestionIDs)
	if err != nil {
		return model.ExamGradeResponse{}, fmt.Errorf("service.Exam.Grade: %w", err)
	}

	resp := gradeAnswers(payload.QuestionIDs, keys, req.Answers)
	resp.Expired = s.now().Unix() > payload.IssuedAt+int64(payload.DurationSec)
	return resp, nil
}

// gradeAnswers is the pure scoring core (no IO): given the exam's ordered
// question ids, the answer key for those ids, and the submitted answers, it
// computes per-question correctness, per-subject and overall scores.
//
// Rules:
//   - total is len(questionIDs) — every issued question counts, answered or not.
//   - an unanswered or wrongly-answered question is incorrect.
//   - selected is compared verbatim to the key's answer (handler upper-cases /
//     validates input; an out-of-range selected just never matches).
//   - score = round(correct/total*100); per-subject score likewise over that
//     subject's issued count. total 0 yields score 0 (no divide-by-zero).
//   - a question id missing from keys (e.g. deleted) still counts toward total
//     and is reported with an empty answer/subject, never correct.
func gradeAnswers(questionIDs []int64, keys map[int64]store.GradingRow, answers []model.ExamAnswer) model.ExamGradeResponse {
	selectedByID := make(map[int64]string, len(answers))
	for _, a := range answers {
		selectedByID[a.QuestionID] = a.Selected
	}

	// Accumulate per-subject in first-seen order so the breakdown is stable.
	type subjAgg struct {
		id      int64
		name    string
		correct int
		total   int
	}
	subjOrder := make([]int64, 0)
	subjects := make(map[int64]*subjAgg)

	items := make([]model.ExamGradeItem, 0, len(questionIDs))
	correct := 0

	for _, qid := range questionIDs {
		key := keys[qid] // zero value when missing: Answer "", Subject "", SubjectID 0
		sel := selectedByID[qid]
		isCorrect := key.Answer != "" && sel == key.Answer
		if isCorrect {
			correct++
		}

		agg, ok := subjects[key.SubjectID]
		if !ok {
			agg = &subjAgg{id: key.SubjectID, name: key.Subject}
			subjects[key.SubjectID] = agg
			subjOrder = append(subjOrder, key.SubjectID)
		}
		agg.total++
		if isCorrect {
			agg.correct++
		}

		items = append(items, model.ExamGradeItem{
			QuestionID:  qid,
			Number:      key.Number,
			Selected:    sel,
			Answer:      key.Answer,
			IsCorrect:   isCorrect,
			Explanation: key.Explanation,
			Subject:     key.Subject,
		})
	}

	perSubject := make([]model.ExamSubjectScore, 0, len(subjOrder))
	for _, sid := range subjOrder {
		agg := subjects[sid]
		perSubject = append(perSubject, model.ExamSubjectScore{
			SubjectID: agg.id,
			Subject:   agg.name,
			Correct:   agg.correct,
			Total:     agg.total,
			Score:     pctScore(agg.correct, agg.total),
		})
	}

	total := len(questionIDs)
	return model.ExamGradeResponse{
		Score:      pctScore(correct, total),
		Correct:    correct,
		Total:      total,
		PerSubject: perSubject,
		Items:      items,
	}
}

// pctScore returns round(correct/total*100), or 0 when total is 0.
func pctScore(correct, total int) int {
	if total == 0 {
		return 0
	}
	return int(math.Round(float64(correct) / float64(total) * 100))
}

// perSubjectCounts groups drawn questions by subject, preserving first-seen
// order (the store returns rows in subject order). Each entry carries the
// subject id, name, and the number of questions drawn from it.
func perSubjectCounts(drawn []store.SampledQuestion) []model.ExamSubjectCount {
	order := make([]int64, 0)
	counts := make(map[int64]*model.ExamSubjectCount)
	for _, sq := range drawn {
		sid := sq.Question.SubjectID
		agg, ok := counts[sid]
		if !ok {
			agg = &model.ExamSubjectCount{SubjectID: sid, Subject: sq.Subject}
			counts[sid] = agg
			order = append(order, sid)
		}
		agg.Count++
	}
	out := make([]model.ExamSubjectCount, 0, len(order))
	for _, sid := range order {
		out = append(out, *counts[sid])
	}
	return out
}
