package worker

import (
	"log"

	"github.com/hibiken/asynq"
	"github.com/mightyfzeus/doc-explain/internal/env"
	"github.com/mightyfzeus/doc-explain/internal/jobs"
)

func main() {
	redisOpt := asynq.RedisClientOpt{
		Addr:     env.GetString("REDIS_URL", "localhost:6379"),
		Username: env.GetString("REDIS_USERNAME", ""),
		Password: env.GetString("REDIS_PASSWORD", ""),
		DB:       env.GetInt("REDIS_DB", 0),
	}

	server := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: 3,
		Queues: map[string]int{
			"rag":     10,
			"default": 1,
		},
	})

	mux := asynq.NewServeMux()

	processor := NewDocumentProcessor()
	mux.Handle(jobs.TypeProcessDocument, processor)

	if err := server.Run(mux); err != nil {
		log.Fatal(err)
	}
}
