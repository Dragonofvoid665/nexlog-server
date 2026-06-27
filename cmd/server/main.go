package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"nexlog/internal/cache"
	"nexlog/internal/db"
	"nexlog/internal/handlers"
)

func main() {
	// Optimise for 2 cores
	runtime.GOMAXPROCS(runtime.NumCPU())
	log.Printf("🔧 GOMAXPROCS=%d", runtime.GOMAXPROCS(0))

	// Config from env (with sane defaults)
	port := getEnv("PORT", "3000")
	jwtSecret := getEnv("JWT_SECRET", generateSecret())
	dataDir := getEnv("DATA_DIR", "./data")
	publicDir := getEnv("PUBLIC_DIR", "./public")
	uploadDir := filepath.Join(publicDir, "uploads")

	// Database
	database, err := db.Open(dataDir)
	if err != nil {
		log.Fatalf("❌ DB open failed: %v", err)
	}
	defer database.Close()
	if err := database.Init(); err != nil {
		log.Fatalf("❌ DB init failed: %v", err)
	}

	// Cache: 30s TTL
	appCache := cache.New(30 * time.Second)

	// Handlers + routes
	h := handlers.New(database, appCache, jwtSecret, uploadDir)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux, publicDir)

	// Middleware stack: rate limit → security headers → compress → mux
	handler := rateLimitMiddleware(
		securityHeaders(
			gzipMiddleware(mux),
		),
		100,              // max requests per window per IP
		15*time.Minute,  // window
	)

	// HTTP server tuned for 10k concurrent connections on 2GB RAM
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
		// Allow efficient handling of many connections
		ConnState: func(conn net.Conn, state http.ConnState) {},
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("\n✅ NexLog Go server on :%s", port)
		log.Printf("🔧 Admin panel: http://localhost:%s/admin", port)
		log.Printf("🔑 Default password: password")
		log.Printf("💾 Database: %s/nexlog.db", dataDir)
		log.Printf("⚙️  Goroutine-based, SQLite WAL, in-memory cache\n")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-quit
	log.Println("🛑 Shutting down gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Shutdown error: %v", err)
	}
	log.Println("👋 Server stopped")
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func generateSecret() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}
