package main

import (
	"context"
	"net/http"
	_ "net/http/pprof" // registers /debug/pprof automatically
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"nexlog/configs"
	appdb "nexlog/internal/db"
	"nexlog/internal/cache"
	"nexlog/internal/handlers"
	"nexlog/internal/logger"
	"nexlog/internal/migrate"
	mw "nexlog/internal/middleware"
	"nexlog/internal/repository"
	"nexlog/internal/service"
)

func main() {
	// ─── Config ──────────────────────────────────────────────────────────────
	cfg, err := configs.Load()
	if err != nil {
		// Do NOT start without valid config — especially without JWT_SECRET
		logger.Init(true)
		logger.Error("❌ Config error", "err", err)
		os.Exit(1)
	}

	logger.Init(cfg.IsDev)
	runtime.GOMAXPROCS(runtime.NumCPU())
	logger.Info("🔧 Go runtime", "GOMAXPROCS", runtime.GOMAXPROCS(0))

	// ─── Database ─────────────────────────────────────────────────────────────
	db, err := appdb.Open(cfg)
	if err != nil {
		logger.Error("❌ DB open failed", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	// ─── Migrations ───────────────────────────────────────────────────────────
	if err := migrate.Run(db, cfg.MigrationsDir); err != nil {
		logger.Error("❌ Migration failed", "err", err)
		os.Exit(1)
	}

	// ─── Layers ───────────────────────────────────────────────────────────────
	repo := repository.New(db)
	svc := service.New(repo, db)
	appCache := cache.New(30 * time.Second)

	ctx := context.Background()
	if err := svc.SeedIfEmpty(ctx); err != nil {
		logger.Error("❌ Seed failed", "err", err)
		os.Exit(1)
	}

	h := handlers.New(svc, appCache, cfg.JWTSecret, cfg.UploadDir)

	// ─── Router ───────────────────────────────────────────────────────────────
	mux := http.NewServeMux()
	authMiddleware := mw.Auth(cfg.JWTSecret)
	h.RegisterRoutes(mux, authMiddleware, cfg.PublicDir)

	if cfg.EnablePprof {
		// pprof is auto-registered to DefaultServeMux — expose on separate port
		go func() {
			logger.Info("🔬 pprof listening on :6060")
			http.ListenAndServe(":6060", nil)
		}()
	}

	// ─── Middleware stack ─────────────────────────────────────────────────────
	rl := mw.NewRateLimiter(cfg.RateLimitPerMin)
	handler := rl.Middleware(
		mw.CORS(cfg.AllowedOrigins)(
			mw.SecurityHeaders(
				mw.Gzip(mux),
			),
		),
	)

	// ─── HTTP Server ─────────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("✅ NexLog server started", "port", cfg.Port, "env", func() string {
			if cfg.IsDev { return "development" }; return "production"
		}())
		logger.Info("🔧 Admin panel", "url", "http://localhost:"+cfg.Port+"/admin")
		logger.Info("🔑 Default password: password  ← change immediately!")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server fatal error", "err", err)
		}
	}()

	<-quit
	logger.Info("🛑 Graceful shutdown...")
	shutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		logger.Error("Shutdown error", "err", err)
	}
	logger.Info("👋 Server stopped")
}
