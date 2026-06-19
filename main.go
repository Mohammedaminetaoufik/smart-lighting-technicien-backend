package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"

	"technicien-mobile/internal/controllers"
	"technicien-mobile/internal/repository"
)

func main() {
	_ = godotenv.Load()

	db, err := repository.OpenDB()
	if err != nil {
		log.Fatalf("[main] connexion DB impossible: %v", err)
	}
	defer db.Close()

	if err := repository.EnsureMobileSchema(db); err != nil {
		log.Printf("[main] avertissement schema: %v", err)
	}

	repository.SeedTestDataIfEmpty(db)

	if os.Getenv("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()
	router.MaxMultipartMemory = 10 << 20 // 10 MiB

	uploadDir := os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "uploads"
	}
	router.Static("/uploads", uploadDir)

	// CORS — "*" par défaut (app mobile Expo). En production, définir
	// CORS_ORIGIN avec l'origine web autorisée (ex: https://tech.lamalif.ma).
	corsOrigin := os.Getenv("CORS_ORIGIN")
	if corsOrigin == "" {
		corsOrigin = "*"
	}
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", corsOrigin)
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, X-Test-Technician-Id")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	// ──────────────────────────────────────────────
	// Mobile API routes — /api/mobile/*
	// TODO(auth): Quand AUTH_ENABLED=true, ajouter middleware.RequireAuth() sur ces routes.
	// ──────────────────────────────────────────────
	mobile := router.Group("/api/mobile")
	{
		mobile.GET("/health", controllers.HandleHealth())
		mobile.GET("/test-context", controllers.HandleTestContext())

		// Dashboard technicien
		mobile.GET("/me/dashboard", controllers.HandleDashboard(db))

		// Bons de travail
		mobile.GET("/me/workorders", controllers.HandleListMyWorkOrders(db))
		mobile.GET("/workorders/:id", controllers.HandleGetWorkOrder(db))

		// Actions intervention
		mobile.POST("/workorders/:id/accept", controllers.HandleAcceptWorkOrder(db))
		mobile.POST("/workorders/:id/start", controllers.HandleStartWorkOrder(db))
		mobile.POST("/workorders/:id/add-note", controllers.HandleAddNote(db))
		mobile.POST("/workorders/:id/resolve", controllers.HandleResolveWorkOrder(db))
		mobile.POST("/workorders/:id/block", controllers.HandleBlockWorkOrder(db))

		// Photos interventions
		mobile.POST("/workorders/:id/photos", controllers.HandleUploadWorkOrderPhoto(db))
		mobile.GET("/workorders/:id/photos", controllers.HandleListWorkOrderPhotos(db))

		// Lampadaires (consultation terrain)
		mobile.GET("/lampadaires", controllers.HandleListLampadaires(db))
		mobile.GET("/lampadaires/:id/details", controllers.HandleLampadaireDetails(db))
		mobile.GET("/lampadaires/:id/diagnostic", controllers.HandleDiagnostic(db))
		mobile.GET("/lampadaires/:id/telemetry/latest", controllers.HandleLatestTelemetry(db))
		mobile.GET("/lampadaires/:id/alerts", controllers.HandleLampadaireAlerts(db))
		mobile.GET("/lampadaires/:id/workorders", controllers.HandleLampadaireWorkOrders(db))
		mobile.POST("/lampadaires/:id/field-note", controllers.HandleLampadaireFieldNote(db))
		mobile.POST("/lampadaires/:id/location", controllers.HandleUpdateLocation(db))

		// LCUs (consultation + diagnostic terrain)
		mobile.GET("/lcus", controllers.HandleListLCUsMobile(db))
		mobile.GET("/lcus/:id/details", controllers.HandleLCUDetails(db))
		mobile.GET("/lcus/:id/lampadaires", controllers.HandleLCULampadairesMobile(db))
		mobile.GET("/lcus/:id/diagnostic", controllers.HandleLCUDiagnostic(db))
		mobile.POST("/lcus/:id/test", controllers.HandleLCUTest(db))
		mobile.POST("/lcus/:id/sync", controllers.HandleLCUSync(db))
		mobile.POST("/lcus/:id/field-note", controllers.HandleLCUFieldNote(db))
		mobile.POST("/lcus/:id/location", controllers.HandleUpdateLCULocation(db))

		// Mise en service (commissioning terrain)
		mobile.GET("/commissioning", controllers.HandleListCommissioning(db))
		mobile.GET("/commissioning/:id", controllers.HandleGetCommissioning(db))
		mobile.POST("/commissioning/:id/update-gps", controllers.HandleCommissioningGPS(db))
		mobile.POST("/commissioning/:id/test-communication", controllers.HandleCommissioningTestComm(db))
		mobile.POST("/commissioning/:id/test-dimming", controllers.HandleCommissioningTestDimming(db))
		mobile.POST("/commissioning/:id/validate", controllers.HandleCommissioningValidate(db))
		mobile.POST("/commissioning/:id/fail", controllers.HandleCommissioningFail(db))
		mobile.POST("/commissioning/:id/add-note", controllers.HandleCommissioningNote(db))

		// Synchronisation JSON offline-first
		mobile.GET("/sync/bootstrap", controllers.HandleSyncBootstrap(db))
		mobile.GET("/sync/pull", controllers.HandleSyncPull(db))
		mobile.POST("/sync/push", controllers.HandleSyncPush(db))
		mobile.POST("/sync/full", controllers.HandleSyncFull(db))
	}

	// ──────────────────────────────────────────────
	// Map API routes — /api/map/*
	// Partagées entre la partie web et la partie technicien.
	// TODO(auth): Quand AUTH_ENABLED=true, protéger les routes sensibles.
	// ──────────────────────────────────────────────
	mapGroup := router.Group("/api/map")
	{
		mapGroup.GET("/overview", controllers.HandleMapOverview(db))
		mapGroup.GET("/lampadaires", controllers.HandleMapLampadaires(db))
		mapGroup.GET("/lcus", controllers.HandleMapLCUs(db))
		mapGroup.GET("/connections", controllers.HandleMapConnections(db))
		mapGroup.GET("/technician-context", controllers.HandleTechnicianContext(db))
		mapGroup.GET("/lampadaires/missing-location", controllers.HandleMissingLocation(db))
		mapGroup.POST("/lampadaires/:id/location", controllers.HandleUpdateLocation(db))
		mapGroup.POST("/lampadaires/:id/dimming", controllers.HandleUpdateLampadaireDimming(db))
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	go func() {
		log.Printf("[main] Serveur technicien démarré sur http://localhost:%s", port)
		log.Printf("[main] AUTH_ENABLED=%s — routes ouvertes pour les tests", os.Getenv("AUTH_ENABLED"))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[main] erreur serveur: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("[main] Arrêt en cours…")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	log.Println("[main] Serveur arrêté proprement.")
}
