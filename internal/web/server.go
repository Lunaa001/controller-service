package web

import (
	"log"
	"net/http"
	"time"

	"service-controller-notebookum/internal/config"
	"service-controller-notebookum/internal/core/cache"
	"service-controller-notebookum/internal/core/orchestrator"
	"service-controller-notebookum/internal/core/resilience"
	"service-controller-notebookum/internal/transport/services"
	"service-controller-notebookum/internal/web/handlers"
	"service-controller-notebookum/internal/web/middleware"
	"service-controller-notebookum/internal/web/problem"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func NewRouter(cfg config.Config) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.CustomRecovery(func(c *gin.Context, recovered any) {
		problem.Write(c, http.StatusInternalServerError, "Internal Server Error", "An error occurred", middleware.CorrelationID(c))
	}))

	// CORS — permitir requests desde el dashboard
	router.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-User-ID", "X-Correlation-ID"},
		ExposeHeaders:    []string{"Content-Length", "X-Correlation-ID"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	router.Use(middleware.Correlation(cfg.CorrelationHeader))

	// Crear timeout para requests
	timeout := time.Duration(cfg.RequestTimeout) * time.Second

	// Inicializar Redis cache
	redisCache, err := cache.New(cfg.RedisURL, cfg.RedisPassword)
	if err != nil {
		log.Printf("warning: failed to initialize redis: %v", err)
		// Continuar sin cache si Redis falla
	}

	// Inicializar servicios con clientes upstream
	userService := services.NewUserService(cfg.UserServiceURL, timeout)
	extractService := services.NewExtractService(cfg.ExtractServiceURL, timeout)
	summaryService := services.NewSummaryService(cfg.SummaryServiceURL, timeout)
	persistenceService := services.NewPersistenceService(cfg.PersistenceURL, timeout)

	// Inicializar orquestador
	orch := orchestrator.New(userService, extractService, summaryService, persistenceService, redisCache)

	// Inicializar handlers
	registry := resilience.NewRegistry()
	health := handlers.NewHealthHandler(registry)
	users := handlers.NewUsersHandler(cfg)
	documentsHandler := handlers.NewDocumentsHandler(orch)
	summaries := handlers.NewSummariesHandler(orch)

	// Rutas públicas (sin autenticación)
	router.GET("/health", health.Health)
	router.GET("/ready", health.Ready)
	router.GET("/status/circuits", health.CircuitStatus)

	// Document upload — acepta X-User-ID header o usa "anonymous" como default
	router.POST("/api/v1/documents/upload", middleware.OptionalAuth(), documentsHandler.Upload)

	// Rutas con autenticación
	router.POST("/api/v1/users", middleware.RequireAuth(), users.Create)
	router.GET("/api/v1/users/:id", middleware.RequireAuth(), users.Get)
	router.GET("/api/v1/documents/:id", middleware.RequireAuth(), documentsHandler.Status)
	router.GET("/api/v1/summaries/document/:id", middleware.RequireAuth(), summaries.Get)

	// Ruta 404
	router.NoRoute(func(c *gin.Context) {
		problem.Write(c, http.StatusNotFound, "Not Found", "Resource not found", middleware.CorrelationID(c))
	})

	return router
}
