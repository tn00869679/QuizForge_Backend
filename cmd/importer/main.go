// cmd/importer — QuizForge 匯入工具（兼任 seed loader）
//
// 子命令：
//
//	importer seed  <file.json>         — 讀 IMPORT_FORMAT JSON，呼叫 service.Import.Apply 寫入 DB
//	importer parse --questions <q.pdf> --answers <a.pdf> --out <out.json>
//	                                    — SKELETON：用 pdftotext 抽文字 → 切題 → 輸出待校對 JSON
//
// SKELETON NOTE（parse 子命令）:
//
//	本檔的 parse 功能為骨架，供說明整合流程之用。
//	真實證基會 PDF 排版因年份不同而差異甚大，正則與切題邏輯需逐案人工調整。
//	輸出 JSON 每題均標記 "needs_review": true，必須人工校對後方可透過 `seed` 子命令正式匯入。
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/tn00869679/QuizForge_Backend/internal/config"
	"github.com/tn00869679/QuizForge_Backend/internal/db"
	"github.com/tn00869679/QuizForge_Backend/internal/importer"
	"github.com/tn00869679/QuizForge_Backend/internal/model"
	"github.com/tn00869679/QuizForge_Backend/internal/service"
	"github.com/tn00869679/QuizForge_Backend/internal/store"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "seed":
		err = runSeed(os.Args[2:])
	case "parse":
		err = runParse(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %q\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		slog.Error("importer failed", "error", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  importer seed  <file.json>")
	fmt.Fprintln(os.Stderr, "  importer parse --questions <q.pdf> --answers <a.pdf> --out <out.json>")
}

// ────────────────────────────────────────────────────────────────────────────
// seed subcommand
// ────────────────────────────────────────────────────────────────────────────

// runSeed reads an IMPORT_FORMAT JSON file and applies it to the database
// in-process via service.Import.Apply. This is the canonical seed loader.
func runSeed(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("seed: expected exactly one argument <file.json>, got %d", len(args))
	}
	jsonPath := args[0]

	// Load config (reads DATABASE_URL from env, with sane dev defaults).
	cfg := config.Load()

	// Build pgxpool — same pattern as cmd/server/main.go.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	cancel()
	if err != nil {
		return fmt.Errorf("seed: connect to DB: %w", err)
	}
	defer pool.Close()

	// Read and decode the seed JSON.
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return fmt.Errorf("seed: read %q: %w", jsonPath, err)
	}
	var payload model.ImportPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("seed: decode %q: %w", jsonPath, err)
	}

	// Apply via the service layer (validation + transactional upsert).
	svc := service.NewImport(store.NewImport(pool))
	applyCtx, applyCancel := context.WithTimeout(context.Background(), 60*time.Second)
	res, err := svc.Apply(applyCtx, payload)
	applyCancel()
	if err != nil {
		var ve *service.ValidationError
		if errors.As(err, &ve) {
			return fmt.Errorf("seed: validation error: %s", ve.Msg)
		}
		return fmt.Errorf("seed: apply: %w", err)
	}

	slog.Info("seed complete",
		"categories", res.Categories,
		"subjects", res.Subjects,
		"sessions", res.Sessions,
		"questions_inserted", res.QuestionsInserted,
		"questions_updated", res.QuestionsUpdated,
	)
	fmt.Printf("categories:%d  subjects:%d  sessions:%d  questions_inserted:%d  questions_updated:%d\n",
		res.Categories, res.Subjects, res.Sessions, res.QuestionsInserted, res.QuestionsUpdated)
	return nil
}

// ────────────────────────────────────────────────────────────────────────────
// parse subcommand
// ────────────────────────────────────────────────────────────────────────────

// parseFlags holds the parsed CLI flags for the parse subcommand.
type parseFlags struct {
	questionsPath string
	answersPath   string
	outPath       string
	subject       string
	sessionLabel  string
	year          int
	term          int
	sourceURL     string
}

// parseParseArgs parses the flag-style arguments for `importer parse`.
// Supports: --questions, --answers, --out, --subject, --session-label,
//
//	--year, --term, --source-url
func parseParseArgs(args []string) (parseFlags, error) {
	var f parseFlags
	f.year = 0
	f.term = 0
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--questions":
			i++
			if i >= len(args) {
				return f, fmt.Errorf("--questions requires a value")
			}
			f.questionsPath = args[i]
		case "--answers":
			i++
			if i >= len(args) {
				return f, fmt.Errorf("--answers requires a value")
			}
			f.answersPath = args[i]
		case "--out":
			i++
			if i >= len(args) {
				return f, fmt.Errorf("--out requires a value")
			}
			f.outPath = args[i]
		case "--subject":
			i++
			if i >= len(args) {
				return f, fmt.Errorf("--subject requires a value")
			}
			f.subject = args[i]
		case "--session-label":
			i++
			if i >= len(args) {
				return f, fmt.Errorf("--session-label requires a value")
			}
			f.sessionLabel = args[i]
		case "--year":
			i++
			if i >= len(args) {
				return f, fmt.Errorf("--year requires a value")
			}
			if _, err := fmt.Sscanf(args[i], "%d", &f.year); err != nil {
				return f, fmt.Errorf("--year: invalid integer %q", args[i])
			}
		case "--term":
			i++
			if i >= len(args) {
				return f, fmt.Errorf("--term requires a value")
			}
			if _, err := fmt.Sscanf(args[i], "%d", &f.term); err != nil {
				return f, fmt.Errorf("--term: invalid integer %q", args[i])
			}
		case "--source-url":
			i++
			if i >= len(args) {
				return f, fmt.Errorf("--source-url requires a value")
			}
			f.sourceURL = args[i]
		default:
			return f, fmt.Errorf("unknown flag: %q", args[i])
		}
	}
	return f, nil
}

// parsedOutput is the top-level JSON envelope written by `importer parse`.
// It deliberately does NOT match ImportPayload so that downstream tooling
// (human reviewer) is reminded to fill in category/session metadata and
// remove/address the needs_review flag before calling `importer seed`.
type parsedOutput struct {
	// Note is a human-readable reminder that this file requires review.
	Note string `json:"_note"`
	// GeneratedAt records when this file was produced.
	GeneratedAt string `json:"_generated_at"`
	// CategoryCode / CategoryName / SessionLabel are placeholders;
	// fill them in before running `importer seed`.
	CategoryCode string `json:"category_code"`
	CategoryName string `json:"category_name"`
	SessionLabel string `json:"session_label"`
	Year         int    `json:"year"`
	Term         int    `json:"term"`
	SourceURL    string `json:"source_url"`
	Subject      string `json:"subject"`
	// Questions is the list of extracted questions, each with needs_review:true.
	Questions []importer.ParsedQuestion `json:"questions"`
}

// runParse is the skeleton PDF-parse subcommand.
// It calls pdftotext, splits the text into questions, matches answers,
// and writes the result to --out as a JSON file marked for human review.
func runParse(args []string) error {
	f, err := parseParseArgs(args)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	if f.questionsPath == "" {
		return fmt.Errorf("parse: --questions is required")
	}
	if f.answersPath == "" {
		return fmt.Errorf("parse: --answers is required")
	}
	if f.outPath == "" {
		return fmt.Errorf("parse: --out is required")
	}

	slog.Info("extracting text from questions PDF", "path", f.questionsPath)
	questionText, err := importer.PDFToText(f.questionsPath)
	if err != nil {
		return fmt.Errorf("parse: extract questions PDF: %w", err)
	}

	slog.Info("extracting text from answers PDF", "path", f.answersPath)
	answerText, err := importer.PDFToText(f.answersPath)
	if err != nil {
		return fmt.Errorf("parse: extract answers PDF: %w", err)
	}

	questions := importer.BuildParsedQuestions(questionText, answerText, f.sourceURL)
	slog.Info("extracted questions", "count", len(questions))

	out := parsedOutput{
		Note:         "SKELETON OUTPUT — 每題 needs_review:true，請人工校對後再以 `importer seed` 匯入",
		GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
		CategoryCode: "02",    // SKELETON_TODO: fill in correct category
		CategoryName: "（待填寫）", // SKELETON_TODO: fill in correct category name
		SessionLabel: f.sessionLabel,
		Year:         f.year,
		Term:         f.term,
		SourceURL:    f.sourceURL,
		Subject:      f.subject,
		Questions:    questions,
	}

	outData, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("parse: marshal output: %w", err)
	}
	if err := os.WriteFile(f.outPath, outData, 0o644); err != nil {
		return fmt.Errorf("parse: write %q: %w", f.outPath, err)
	}

	slog.Info("parse complete — REQUIRES HUMAN REVIEW before importing",
		"out", f.outPath,
		"questions", len(questions),
	)
	fmt.Printf("parse complete: %d questions written to %s (all marked needs_review:true)\n",
		len(questions), f.outPath)
	fmt.Println("NOTE: review and correct stem/options/answer/explanation before running `importer seed`")
	return nil
}
