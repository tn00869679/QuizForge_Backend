package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tn00869679/QuizForge_Backend/internal/config"
	"github.com/tn00869679/QuizForge_Backend/internal/service"
	"github.com/tn00869679/QuizForge_Backend/internal/store"
)

// Deps are the external dependencies needed to build the API handlers.
type Deps struct {
	Pool *pgxpool.Pool
	Cfg  config.Config
}

// Handlers holds the constructed services and serves the HTTP layer. New
// segments add their services here and register their routes in RegisterRoutes.
type Handlers struct {
	cfg      config.Config
	catalog  *service.Catalog
	question *service.Questions
	imports  *service.Import
	practice *service.Practice
	exam     *service.Exam
	attempts *service.Attempts
}

// New wires stores and services from Deps and returns the Handlers.
func New(deps Deps) *Handlers {
	categories := store.NewCategories(deps.Pool)
	subjects := store.NewSubjects(deps.Pool)
	examSessions := store.NewExamSessions(deps.Pool)
	questions := store.NewQuestions(deps.Pool)
	imports := store.NewImport(deps.Pool)
	attempts := store.NewAttempts(deps.Pool)

	return &Handlers{
		cfg:      deps.Cfg,
		catalog:  service.NewCatalog(categories, subjects, examSessions),
		question: service.NewQuestions(questions),
		imports:  service.NewImport(imports),
		practice: service.NewPractice(questions),
		exam:     service.NewExam(questions, deps.Cfg.ExamHMACSecret),
		attempts: service.NewAttempts(attempts, questions),
	}
}

// RegisterRoutes mounts all routes onto rg (the /api/v1 group).
//
// Middleware is injected by the caller to keep this package free of any
// dependency on internal/middleware (middleware already depends on this package
// for the response helpers, so importing it here would create a cycle):
//   - userStub is applied to the whole group so status-filtered queries and the
//     practice/exam endpoints can read the optional X-User-Id.
//   - requireUser guards the per-user subgroup (attempts/stats): it 401s when no
//     X-User-Id is present.
//   - adminMW are applied only to the /admin subgroup (admin auth + stricter
//     rate limit).
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup, userStub, requireUser gin.HandlerFunc, adminMW ...gin.HandlerFunc) {
	rg.Use(userStub)

	rg.GET("/categories", h.ListCategories)
	rg.GET("/categories/:id/subjects", h.ListSubjects)
	rg.GET("/exam-sessions", h.ListExamSessions)
	rg.GET("/questions", h.ListQuestions)
	rg.GET("/questions/:id", h.GetQuestion)

	// Practice + exam are public (no login): a status filter on practice still
	// requires X-User-Id, enforced in the handler.
	rg.POST("/practice/generate", h.GeneratePractice)
	rg.POST("/exam/start", h.StartExam)
	rg.POST("/exam/grade", h.GradeExam)

	// Per-user state: requires a valid X-User-Id.
	authed := rg.Group("")
	authed.Use(requireUser)
	authed.POST("/attempts", h.AnswerAttempt)
	authed.PATCH("/attempts/:question_id", h.PatchAttempt)
	authed.GET("/attempts", h.ListAttempts)
	authed.GET("/stats", h.GetStats)

	admin := rg.Group("/admin")
	admin.Use(adminMW...)
	admin.POST("/import", h.Import)
}
