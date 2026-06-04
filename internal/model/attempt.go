package model

import "time"

// Attempt is the per-(user, question) state row. selected is nullable: a row may
// exist with only a flag set (favorited/marked) and no answer yet.
type Attempt struct {
	ID                int64     `json:"id"`
	UserID            int64     `json:"user_id"`
	QuestionID        int64     `json:"question_id"`
	Selected          *string   `json:"selected"`
	IsCorrect         bool      `json:"is_correct"`
	IsMarkedUncertain bool      `json:"is_marked_uncertain"`
	IsFavorite        bool      `json:"is_favorite"`
	Mode              string    `json:"mode"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// AttemptAnswerRequest is the body of POST /attempts.
type AttemptAnswerRequest struct {
	QuestionID int64  `json:"question_id"`
	Selected   string `json:"selected"`
	Mode       string `json:"mode"`
}

// AttemptPatchRequest is the body of PATCH /attempts/:question_id. Both flags are
// pointers so an absent field is left unchanged (partial update).
type AttemptPatchRequest struct {
	IsFavorite        *bool `json:"is_favorite"`
	IsMarkedUncertain *bool `json:"is_marked_uncertain"`
}

// StatsResponse is the body of GET /stats: aggregate counts for one user.
// Accuracy is correct/answered (0 when answered is 0).
type StatsResponse struct {
	Answered  int     `json:"answered"`
	Correct   int     `json:"correct"`
	Accuracy  float64 `json:"accuracy"`
	Wrong     int     `json:"wrong"`
	Favorites int     `json:"favorites"`
	Uncertain int     `json:"uncertain"`
}

// AttemptStatus is the compact attempt view embedded in a review list item.
type AttemptStatus struct {
	Selected          *string `json:"selected"`
	IsCorrect         bool    `json:"is_correct"`
	IsMarkedUncertain bool    `json:"is_marked_uncertain"`
	IsFavorite        bool    `json:"is_favorite"`
	Mode              string  `json:"mode"`
}

// ReviewItem pairs a full question (answer/explanation included — review mode)
// with the user's attempt state, for GET /attempts?status=.
type ReviewItem struct {
	Question QuestionPublic `json:"question"`
	Attempt  AttemptStatus  `json:"attempt"`
}
