package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"

	"ngac-document-platform/internal/api"
	"ngac-document-platform/internal/auth"
	"ngac-document-platform/internal/ngac"
	"ngac-document-platform/internal/seed"
)

func main() {
	// Config from env
	dbURL := getEnv("DATABASE_URL", "postgres://ngac:ngac_secret@localhost:5432/ngac?sslmode=disable")
	jwtSecret := getEnv("JWT_SECRET", "ngac-super-secret-key-change-in-production")
	port := getEnv("PORT", "8080")
	dataDir := getEnv("DATA_DIR", "/data/documents")

	auth.SetJWTSecret(jwtSecret)

	// Ensure data directory exists
	os.MkdirAll(dataDir, 0755)

	// Connect to PostgreSQL with retry
	ctx := context.Background()
	var pool *pgxpool.Pool
	var err error

	for i := 0; i < 30; i++ {
		pool, err = pgxpool.New(ctx, dbURL)
		if err == nil {
			if err = pool.Ping(ctx); err == nil {
				break
			}
		}
		log.Printf("Waiting for database... attempt %d/30", i+1)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Initialize NGAC graph
	graph := ngac.NewGraph()
	store := ngac.NewStore(pool, graph)

	// Create schema
	if err := store.InitSchema(ctx); err != nil {
		log.Fatalf("Failed to init schema: %v", err)
	}

	// Load graph from DB
	if err := store.LoadGraph(ctx); err != nil {
		log.Fatalf("Failed to load graph: %v", err)
	}

	// Seed data if needed
	if err := seed.SeedData(ctx, store); err != nil {
		log.Printf("Warning: seed data error: %v", err)
	}

	// Re-load graph after seeding
	graph2 := ngac.NewGraph()
	store2 := ngac.NewStore(pool, graph2)
	if err := store2.LoadGraph(ctx); err != nil {
		log.Fatalf("Failed to reload graph: %v", err)
	}

	// Setup constraint engine
	constraints := ngac.NewConstraintEngine()
	constraints.Register(ngac.WeekdayOnlyConstraint)

	// Setup handlers
	authHandler := api.NewAuthHandler(store2)
	docHandler := api.NewDocumentHandler(store2, constraints, dataDir)
	adminHandler := api.NewAdminHandler(store2)

	// Setup router
	r := chi.NewRouter()

	// Middleware
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(60 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Public routes
	r.Post("/api/auth/register", authHandler.Register)
	r.Post("/api/auth/login", authHandler.Login)

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(api.AuthMiddleware)

		// Documents
		r.Get("/api/documents", docHandler.List)
		r.Post("/api/documents", docHandler.Upload)
		r.Get("/api/documents/{id}", docHandler.Get)
		r.Delete("/api/documents/{id}", docHandler.Delete)
		r.Post("/api/documents/{id}/approve", docHandler.Approve)
		r.Post("/api/documents/{id}/share", docHandler.Share)
		r.Delete("/api/documents/{id}/share/{shareId}", docHandler.RevokeShare)
		r.Get("/api/documents/{id}/shares", docHandler.ListShares)
		r.Post("/api/documents/{id}/publish", docHandler.Publish)
		r.Delete("/api/documents/{id}/publish", docHandler.Unpublish)

		// Access check
		r.Post("/api/access/check", docHandler.CheckAccess)

		// Admin
		r.Get("/api/companies", adminHandler.ListCompanies)
		r.Get("/api/companies/{id}/departments", adminHandler.ListDepartments)
		r.Get("/api/departments", adminHandler.ListAllDepartments)
		r.Get("/api/users", adminHandler.ListUsers)
	})

	log.Printf("NGAC Document Platform starting on port %s", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), r))
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
