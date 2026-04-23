// cmd/worker/main.go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/realestate/backend/internal/config"
	"github.com/realestate/backend/internal/db"
	rdb "github.com/realestate/backend/internal/redis"
	"github.com/realestate/backend/internal/worker"
)

func main() {
	ctx := context.Background()

	// ── Config ────────────────────────────────────────────────────────────────
	cfg := config.Load()

	// ── Database ──────────────────────────────────────────────────────────────
	pool, err := db.New(ctx, cfg)
	if err != nil {
		log.Fatalf("worker: database init failed: %v", err)
	}
	defer pool.Close()
	log.Println("worker: database connected")

	// ── Redis ─────────────────────────────────────────────────────────────────
	redisClient, err := rdb.New(ctx, cfg)
	if err != nil {
		log.Fatalf("worker: redis init failed: %v", err)
	}
	defer redisClient.Close()
	log.Println("worker: redis connected")

	// ── Worker client (used by handlers to enqueue follow-on tasks) ───────────
	workerClient, err := worker.NewClient(cfg)
	if err != nil {
		log.Fatalf("worker: client init failed: %v", err)
	}
	defer workerClient.Close()

	// ── Dispatcher — holds all handler dependencies ───────────────────────────
	dispatcher := worker.NewDispatcher(worker.DispatcherConfig{
		DB:            pool,
		WorkerClient:  workerClient,
		TwilioSID:     cfg.TwilioAccountSID,
		TwilioAuth:    cfg.TwilioAuthToken,
		TwilioWAFrom:  cfg.TwilioWhatsAppFrom,
		TwilioSMSFrom: cfg.TwilioSMSFrom,
		GCSBucket:     cfg.GCSBucket,
	})

	if cfg.TwilioAccountSID == "" {
		log.Println("worker: Twilio not configured — WhatsApp/SMS tasks will log and mark FAILED")
	} else {
		log.Println("worker: Twilio configured")
	}

	// ── Asynq server ──────────────────────────────────────────────────────────
	srv, err := worker.NewAsynqServer(cfg)
	if err != nil {
		log.Fatalf("worker: asynq server init failed: %v", err)
	}

	mux := worker.NewMux(dispatcher)

	// Graceful shutdown on SIGINT / SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("worker: shutting down…")
		srv.Shutdown()
	}()

	log.Printf("worker: starting (env=%s)", cfg.AppEnv)
	if err := srv.Run(mux); err != nil {
		log.Fatalf("worker: run failed: %v", err)
	}

	// suppress unused variable warnings (redisClient is closed above via defer)
	_ = redisClient
}
