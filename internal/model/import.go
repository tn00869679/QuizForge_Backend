package model

// ImportPayload is the authoritative contract for POST /admin/import. The same
// shape is produced by the future PDF importer and by seed files. See
// docs/IMPORT_FORMAT.md.
type ImportPayload struct {
	CategoryCode        string           `json:"category_code"`
	CategoryName        string           `json:"category_name"`
	CategoryDescription string           `json:"category_description"`
	Questions           []ImportQuestion `json:"questions"`
}

// ImportQuestion is a single question row inside an ImportPayload. Session and
// subject are denormalised onto each question; the import service get-or-creates
// the corresponding exam_session and subject rows.
type ImportQuestion struct {
	Year         int      `json:"year"`
	Term         int      `json:"term"`
	SessionLabel string   `json:"session_label"`
	SourceURL    string   `json:"source_url"`
	Subject      string   `json:"subject"`
	SubjectOrder int      `json:"subject_order"`
	Number       int      `json:"number"`
	Stem         string   `json:"stem"`
	Options      []Option `json:"options"`
	Answer       string   `json:"answer"`
	Explanation  string   `json:"explanation"`
	Tags         []string `json:"tags"`
	Difficulty   int      `json:"difficulty"`
}

// ImportResult reports per-entity counts after an import transaction.
type ImportResult struct {
	Categories        int `json:"categories"`
	Subjects          int `json:"subjects"`
	Sessions          int `json:"sessions"`
	QuestionsInserted int `json:"questions_inserted"`
	QuestionsUpdated  int `json:"questions_updated"`
}
