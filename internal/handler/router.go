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
}

// New wires stores and services from Deps and returns the Handlers.
func New(deps Deps) *Handlers {
	categories := store.NewCategories(deps.Pool)
	subjects := store.NewSubjects(deps.Pool)
	examSessions := store.NewExamSessions(deps.Pool)
	questions := store.NewQuestions(deps.Pool)
	imports := store.NewImport(deps.Pool)

	return &Handlers{
		cfg:      deps.Cfg,
		catalog:  service.NewCatalog(categories, subjects, examSessions),
		question: service.NewQuestions(questions),
		imports:  service.NewImport(imports),
	}
}

// RegisterRoutes mounts all M1 (query + import) routes onto rg (the /api/v1
// group).
//
// Middleware is injected by the caller to keep this package free of any
// dependency on internal/middleware (middleware already depends on this package
// for the response helpers, so importing it here would create a cycle):
//   - userStub is applied to the whole group so status-filtered queries can read
//     the optional X-User-Id.
//   - adminMW are applied only to the /admin subgroup (admin auth + stricter
//     rate limit).
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup, userStub gin.HandlerFunc, adminMW ...gin.HandlerFunc) {
	rg.Use(userStub)

	rg.GET("/categories", h.ListCategories)
	rg.GET("/categories/:id/subjects", h.ListSubjects)
	rg.GET("/exam-sessions", h.ListExamSessions)
	rg.GET("/questions", h.ListQuestions)
	rg.GET("/questions/:id", h.GetQuestion)

	admin := rg.Group("/admin")
	admin.Use(adminMW...)
	admin.POST("/import", h.Import)
}
