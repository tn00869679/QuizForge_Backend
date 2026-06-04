package model

// PracticeGenerateResponse is the body of POST /practice/generate: just the
// ordered ids of questions matching the supplied filter, for the client to then
// fetch / step through.
type PracticeGenerateResponse struct {
	QuestionIDs []int64 `json:"question_ids"`
}

// ExamStartRequest is the body of POST /exam/start. PerSubjectN and DurationSec
// are optional; handlers default them (50 / 7200) when zero.
type ExamStartRequest struct {
	CategoryID  int64 `json:"category_id"`
	PerSubjectN int   `json:"per_subject_n"`
	DurationSec int   `json:"duration_sec"`
}

// ExamSubjectCount reports how many questions a started exam drew from one
// subject (used in exam/start and, with scores, in exam/grade).
type ExamSubjectCount struct {
	SubjectID int64  `json:"subject_id"`
	Subject   string `json:"subject"`
	Count     int    `json:"count"`
}

// ExamStartResponse is the body returned by POST /exam/start. Questions carry no
// answer/explanation (exam mode); ExamToken is the signed, stateless ticket the
// client returns to exam/grade.
type ExamStartResponse struct {
	ExamToken   string             `json:"exam_token"`
	DurationSec int                `json:"duration_sec"`
	Total       int                `json:"total"`
	PerSubject  []ExamSubjectCount `json:"per_subject"`
	Questions   []QuestionPublic   `json:"questions"`
}

// ExamAnswer is one submitted answer in an exam/grade request.
type ExamAnswer struct {
	QuestionID int64  `json:"question_id"`
	Selected   string `json:"selected"`
}

// ExamGradeRequest is the body of POST /exam/grade.
type ExamGradeRequest struct {
	ExamToken string       `json:"exam_token"`
	Answers   []ExamAnswer `json:"answers"`
}

// ExamSubjectScore is the per-subject breakdown in an exam/grade response.
type ExamSubjectScore struct {
	SubjectID int64  `json:"subject_id"`
	Subject   string `json:"subject"`
	Correct   int    `json:"correct"`
	Total     int    `json:"total"`
	Score     int    `json:"score"`
}

// ExamGradeItem is the per-question result in an exam/grade response. Selected
// is the (possibly empty) submitted choice; Answer/Explanation are revealed at
// grade time.
type ExamGradeItem struct {
	QuestionID  int64  `json:"question_id"`
	Number      int    `json:"number"`
	Selected    string `json:"selected"`
	Answer      string `json:"answer"`
	IsCorrect   bool   `json:"is_correct"`
	Explanation string `json:"explanation"`
	Subject     string `json:"subject"`
}

// ExamGradeResponse is the body returned by POST /exam/grade. Expired is true
// when the (still authentic) token was submitted after issued_at+duration_sec;
// the exam is graded regardless.
type ExamGradeResponse struct {
	Score      int                `json:"score"`
	Correct    int                `json:"correct"`
	Total      int                `json:"total"`
	Expired    bool               `json:"expired"`
	PerSubject []ExamSubjectScore `json:"per_subject"`
	Items      []ExamGradeItem    `json:"items"`
}
