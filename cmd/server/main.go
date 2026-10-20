package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"lis-khanza-mapper/internal/config"
	"lis-khanza-mapper/internal/db"
	"lis-khanza-mapper/internal/handler"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	sqlDB, err := db.Open(cfg.DatabaseDSN)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer sqlDB.Close()

	if err := db.Migrate(sqlDB); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	log.Println("migrations applied")
	if strings.EqualFold(os.Getenv("MIGRATE_ONLY"), "true") {
		return
	}

	srv, err := handler.NewServer(cfg, sqlDB)
	if err != nil {
		log.Fatalf("server: %v", err)
	}

	httpSrv := &http.Server{
		Addr:         cfg.Listen,
		Handler:      srv.Router(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		tzLabel := cfg.Timezone
		if tzLabel == "" {
			tzLabel = "Local"
		}
		log.Printf("listening on %s (env=%s mode=%s timezone=%s)", cfg.Listen, cfg.Env, cfg.MapperMode, tzLabel)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
	log.Println("stopped")
}
