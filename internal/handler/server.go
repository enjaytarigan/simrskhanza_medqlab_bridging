package handler

import (
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"lis-khanza-mapper/internal/auth"
	"lis-khanza-mapper/internal/config"
	"lis-khanza-mapper/internal/medqlab"
	"lis-khanza-mapper/internal/repository"
)

type Server struct {
	cfg          config.Config
	db           *sql.DB
	lisTests     *repository.LisTestRepo
	mappings     *repository.MappingRepo
	simrs        *repository.SimrsRepo
	medqlab      *medqlab.Service
	bridgingLogs *repository.BridgingLogRepo
}

func NewServer(cfg config.Config, db *sql.DB) (*Server, error) {
	if err := initTemplates(); err != nil {
		return nil, err
	}
	return &Server{
		cfg:          cfg,
		db:           db,
		lisTests:     repository.NewLisTestRepo(db),
		mappings:     repository.NewMappingRepo(db),
		simrs:        repository.NewSimrsRepo(db),
		medqlab:      medqlab.NewService(db, cfg.MedQLabBridgingNIP, cfg.Location),
		bridgingLogs: repository.NewBridgingLogRepo(db),
	}, nil
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", s.healthz)
	r.Get("/readyz", s.readyz)

	// MedQLab push webhook (API key — not Basic auth).
	r.Group(func(wr chi.Router) {
		wr.Use(auth.APIKeyMiddleware(s.cfg.MedQLabWebhookAPIKey))
		wr.Post("/api/v1/medqlab/hasil", s.medqlabHasil)
	})

	r.Group(func(pr chi.Router) {
		pr.Use(auth.Middleware(auth.Credentials{
			Username: s.cfg.AuthUsername,
			Password: s.cfg.AuthPassword,
		}))

		// Web UI
		pr.Get("/", s.dashboard)
		pr.Get("/lis-tests", s.lisTestList)
		pr.Get("/lis-tests/new", s.lisTestNewForm)
		pr.Post("/lis-tests/new", s.lisTestCreate)
		pr.Get("/lis-tests/{id}", s.lisTestEditForm)
		pr.Post("/lis-tests/{id}", s.lisTestUpdate)
		pr.Post("/lis-tests/{id}/delete", s.lisTestDelete)

		pr.Get("/templates", s.templateList)
		pr.Get("/panels", s.panelList)
		pr.Get("/map/bulk", s.bulkMapForm)
		pr.Post("/map/bulk", s.bulkMapSubmit)

		pr.Get("/mappings", s.mappingList)
		pr.Post("/mappings/{id}/delete", s.mappingDelete)

		pr.Get("/bridging-logs", s.bridgingLogList)
		pr.Get("/bridging-logs/{id}", s.bridgingLogDetail)

		// JSON API
		pr.Route("/api/v1", func(api chi.Router) {
			api.Get("/lis-tests", s.apiListLisTests)
			api.Post("/lis-tests", s.apiCreateLisTest)
			api.Get("/lis-tests/{id}", s.apiGetLisTest)
			api.Put("/lis-tests/{id}", s.apiUpdateLisTest)
			api.Delete("/lis-tests/{id}", s.apiDeleteLisTest)

			api.Get("/simrs/panels", s.apiListPanels)
			api.Get("/simrs/templates", s.apiListTemplates)

			api.Get("/mappings", s.apiListMappings)
			api.Post("/mappings/bulk", s.apiBulkMap)
			api.Delete("/mappings/{id}", s.apiDeleteMapping)
		})
	})

	return r
}
