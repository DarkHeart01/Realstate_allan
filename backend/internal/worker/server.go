// internal/worker/server.go
package worker

import (
	"context"
	"fmt"
	"log"

	"github.com/hibiken/asynq"
	"github.com/realestate/backend/internal/config"
)

// NewAsynqServer creates an Asynq server connected to Redis.
func NewAsynqServer(cfg *config.Config) (*asynq.Server, error) {
	opts, err := asynq.ParseRedisURI(cfg.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("worker: parse Redis URI: %w", err)
	}

	srv := asynq.NewServer(opts, asynq.Config{
		Concurrency: 10,
		Queues: map[string]int{
			QueueCritical: 6,
			QueueDefault:  3,
			QueueLow:      1,
		},
		ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
			log.Printf("[worker] task %q failed: %v", task.Type(), err)
		}),
	})
	return srv, nil
}

// NewMux builds the task router and registers all handlers via the Dispatcher.
func NewMux(d *Dispatcher) *asynq.ServeMux {
	mux := asynq.NewServeMux()
	mux.HandleFunc(TaskOCRProcessImage, d.HandleOCR)
	mux.HandleFunc(TaskNotifyStale, d.HandleStale)
	mux.HandleFunc(TaskNotifyWhatsApp, d.HandleWhatsApp)
	mux.HandleFunc(TaskNotifySMS, d.HandleSMS)
	return mux
}
