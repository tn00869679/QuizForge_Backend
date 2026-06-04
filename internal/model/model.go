// Package model holds domain types and request/response DTOs shared across layers.
package model

import "time"

// Category is a certification exam category, e.g. "證券商業務員".
type Category struct {
	ID          int64     `json:"id"`
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

// Subject is an exam subject belonging to a Category.
type Subject struct {
	ID         int64     `json:"id"`
	CategoryID int64     `json:"category_id"`
	Name       string    `json:"name"`
	OrderIndex int       `json:"order_index"`
	CreatedAt  time.Time `json:"created_at"`
}

// ExamSession is a single sitting of an exam (a year+term) under a Category.
type ExamSession struct {
	ID         int64     `json:"id"`
	CategoryID int64     `json:"category_id"`
	Year       int       `json:"year"`
	Term       int       `json:"term"`
	SourceURL  string    `json:"source_url"`
	Label      string    `json:"label"`
	CreatedAt  time.Time `json:"created_at"`
}

// Option is a single answer choice on a Question. It maps to one element of the
// questions.options JSONB array.
type Option struct {
	Key  string `json:"key"`
	Text string `json:"text"`
}

// Question is a single multiple-choice question. It is the internal domain
// representation and always carries the answer/explanation; use ToPublic to
// produce the API-facing DTO that can hide them for exam mode.
type Question struct {
	ID            int64     `json:"id"`
	SubjectID     int64     `json:"subject_id"`
	ExamSessionID int64     `json:"exam_session_id"`
	Number        int       `json:"number"`
	Stem          string    `json:"stem"`
	Options       []Option  `json:"options"`
	Answer        string    `json:"answer"`
	Explanation   string    `json:"explanation"`
	Tags          []string  `json:"tags"`
	Difficulty    int       `json:"difficulty"`
	IsActive      bool      `json:"is_active"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// QuestionPublic is the API-facing representation of a Question. Answer and
// Explanation use omitempty and are populated only when answers are included
// (practice mode). In exam mode (include_answer=false) they are left empty and
// omitted from the JSON entirely so the client never receives the solution.
type QuestionPublic struct {
	ID            int64    `json:"id"`
	SubjectID     int64    `json:"subject_id"`
	ExamSessionID int64    `json:"exam_session_id"`
	Number        int      `json:"number"`
	Stem          string   `json:"stem"`
	Options       []Option `json:"options"`
	Answer        string   `json:"answer,omitempty"`
	Explanation   string   `json:"explanation,omitempty"`
	Tags          []string `json:"tags"`
	Difficulty    int      `json:"difficulty"`
}

// ToPublic converts a Question to its API DTO. When includeAnswer is false the
// answer and explanation are dropped (exam / non-revealing mode).
func (q Question) ToPublic(includeAnswer bool) QuestionPublic {
	p := QuestionPublic{
		ID:            q.ID,
		SubjectID:     q.SubjectID,
		ExamSessionID: q.ExamSessionID,
		Number:        q.Number,
		Stem:          q.Stem,
		Options:       q.Options,
		Tags:          q.Tags,
		Difficulty:    q.Difficulty,
	}
	if includeAnswer {
		p.Answer = q.Answer
		p.Explanation = q.Explanation
	}
	return p
}

// Page is the generic paginated response envelope carried inside `data`.
type Page[T any] struct {
	Items    []T `json:"items"`
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	Total    int `json:"total"`
}
