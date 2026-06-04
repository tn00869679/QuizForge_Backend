// Package importer provides a SKELETON PDF-to-JSON pipeline for QuizForge.
//
// THIS IS A SKELETON — NOT A PRODUCTION-READY PARSER.
//
// Reality check:
//   - 證基會 PDF 排版因年份、科目而異，正則需逐案調整。
//   - 題目文字可能跨欄、跨頁，需人工確認合併結果。
//   - 答案 PDF 格式多樣（橫列、直行、表格），需人工核對。
//   - 輸出的 JSON 每題帶 needs_review:true，必須人工校對後方可正式匯入。
//
// Dependency: 外部指令 `pdftotext`（poppler-utils）必須已安裝。
//
// TODOS 標記（SKELETON_TODO）指出真實解析時需補強之處。
package importer

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// rawQuestion holds the uncleaned text extracted from a question PDF before
// answer matching and format normalisation.
type rawQuestion struct {
	Number  int
	Stem    string
	Options map[string]string // key -> text, e.g. "A" -> "..."
}

// ParsedQuestion is a single question ready to be serialised into the
// IMPORT_FORMAT JSON. The NeedsReview flag is always true from this pipeline.
type ParsedQuestion struct {
	Number      int                `json:"number"`
	Stem        string             `json:"stem"`
	Options     []ParsedOption     `json:"options"`
	Answer      string             `json:"answer"`      // filled by matchAnswers; may be empty
	Explanation string             `json:"explanation"` // always empty from PDF; fill manually
	Tags        []string           `json:"tags"`
	Difficulty  int                `json:"difficulty"`
	NeedsReview bool               `json:"needs_review"` // always true; must be manually verified
	Meta        ParsedQuestionMeta `json:"_meta"`
}

// ParsedOption mirrors model.Option for JSON serialisation.
type ParsedOption struct {
	Key  string `json:"key"`
	Text string `json:"text"`
}

// ParsedQuestionMeta records provenance for audit and manual review.
type ParsedQuestionMeta struct {
	SourceURL string `json:"source_url"`
	FetchedAt string `json:"fetched_at"`
	PageHint  int    `json:"page_hint"` // page the question was found on; 0 if unknown
}

// PDFToText runs the external `pdftotext` command (poppler-utils) and returns
// the extracted plain text. If pdftotext is not installed, it returns a
// descriptive error with install instructions.
func PDFToText(pdfPath string) (string, error) {
	bin, err := exec.LookPath("pdftotext")
	if err != nil {
		return "", fmt.Errorf(
			"pdftotext not found in PATH: install poppler-utils\n"+
				"  macOS:  brew install poppler\n"+
				"  Debian: apt-get install poppler-utils\n"+
				"  error:  %w", err)
	}
	// -layout preserves column alignment; output to stdout ("-")
	out, err := exec.Command(bin, "-layout", pdfPath, "-").Output()
	if err != nil {
		return "", fmt.Errorf("pdftotext %q: %w", pdfPath, err)
	}
	return string(out), nil
}

// reQuestionStart matches lines that begin a new question, e.g.:
//
//	"1."  "1、"  " 1."  "（1）"
//
// SKELETON_TODO: 證基會 PDF 題號格式依年份有差異，正則需人工確認後調整。
var reQuestionStart = regexp.MustCompile(`(?m)^\s*(\d+)[\.、）]\s+`)

// reOptionLine matches an answer option line such as:
//
//	"(A) …"  "A. …"  "（A）…"  "A、…"
//
// SKELETON_TODO: 選項縮排和括號風格需按實際 PDF 調整。
var reOptionLine = regexp.MustCompile(`(?m)^\s*[（(]?([ABCD])[）)\.、]\s+(.+)`)

// splitQuestions splits the extracted PDF text into rawQuestion slices by
// detecting question-start lines and collecting option lines beneath each.
//
// Known limitations (SKELETON_TODO):
//  1. Questions spanning multiple pages are not automatically merged — the
//     cross-page text may appear as separate "questions". Manual inspection
//     of the output JSON is mandatory.
//  2. Long stems with embedded line breaks are naively joined with a space;
//     this may concatenate continuation lines from the next question.
//  3. Tables, formulas, and images in the PDF become garbled text.
func splitQuestions(text string) []rawQuestion {
	lines := strings.Split(text, "\n")
	var questions []rawQuestion
	var current *rawQuestion

	for _, line := range lines {
		// Check if this line starts a new question.
		if m := reQuestionStart.FindStringSubmatch(line); m != nil {
			num := 0
			fmt.Sscanf(m[1], "%d", &num)
			if current != nil {
				questions = append(questions, *current)
			}
			stem := strings.TrimSpace(reQuestionStart.ReplaceAllString(line, ""))
			current = &rawQuestion{
				Number:  num,
				Stem:    stem,
				Options: make(map[string]string),
			}
			continue
		}

		if current == nil {
			continue
		}

		// Check if this line is an option.
		if m := reOptionLine.FindStringSubmatch(line); m != nil {
			key := m[1]
			text := strings.TrimSpace(m[2])
			current.Options[key] = text
			continue
		}

		// Non-empty, non-option line: treat as stem continuation.
		// SKELETON_TODO: distinguish stem continuation from page headers/footers.
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && len(current.Options) == 0 {
			current.Stem += " " + trimmed
		}
	}

	if current != nil {
		questions = append(questions, *current)
	}

	return questions
}

// detectOptions converts rawQuestion.Options (map) into the ordered
// []ParsedOption slice expected by IMPORT_FORMAT, preserving A→B→C→D order.
func detectOptions(raw rawQuestion) []ParsedOption {
	keys := []string{"A", "B", "C", "D"}
	var opts []ParsedOption
	for _, k := range keys {
		if text, ok := raw.Options[k]; ok {
			opts = append(opts, ParsedOption{Key: k, Text: text})
		}
	}
	// SKELETON_TODO: Some PDFs use 5+ options or skip a letter; add validation.
	return opts
}

// reAnswerLine matches a line in the answers PDF such as:
//
//	"1 A"  "1. B"  "1、C"  "第1題 D"
//
// SKELETON_TODO: 答案 PDF 格式多樣，可能為表格或橫向排列，需人工確認後調整。
var reAnswerLine = regexp.MustCompile(`(?m)^\s*(?:第\s*)?(\d+)\s*[題\.、]?\s+([ABCD])`)

// parseAnswers extracts a question-number → answer-key map from the
// answers PDF text.
//
// SKELETON_TODO: If the answer sheet uses a grid layout (multiple answers per
// line), this simple per-line approach will miss them. Inspect the raw text
// and adjust the regex accordingly.
func parseAnswers(text string) map[int]string {
	answers := make(map[int]string)
	matches := reAnswerLine.FindAllStringSubmatch(text, -1)
	for _, m := range matches {
		num := 0
		fmt.Sscanf(m[1], "%d", &num)
		if num > 0 {
			answers[num] = m[2]
		}
	}
	return answers
}

// BuildParsedQuestions combines extracted questions and answers into
// []ParsedQuestion ready for JSON serialisation.
//
// sourceURL is recorded for provenance. All output questions have
// NeedsReview=true; Explanation is left empty for manual fill-in.
func BuildParsedQuestions(questionText, answerText, sourceURL string) []ParsedQuestion {
	raws := splitQuestions(questionText)
	answers := parseAnswers(answerText)
	fetchedAt := time.Now().UTC().Format(time.RFC3339)

	out := make([]ParsedQuestion, 0, len(raws))
	for _, raw := range raws {
		ans := answers[raw.Number] // empty string if not found
		pq := ParsedQuestion{
			Number:      raw.Number,
			Stem:        strings.TrimSpace(raw.Stem),
			Options:     detectOptions(raw),
			Answer:      ans,
			Explanation: "", // SKELETON_TODO: fill manually after review
			Tags:        []string{},
			Difficulty:  0, // SKELETON_TODO: assign manually
			NeedsReview: true,
			Meta: ParsedQuestionMeta{
				SourceURL: sourceURL,
				FetchedAt: fetchedAt,
				PageHint:  0, // SKELETON_TODO: track page number during split
			},
		}
		out = append(out, pq)
	}
	return out
}
