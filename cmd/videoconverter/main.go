package main

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"imersaofc/internal/converter"
	"imersaofc/pkg/log"

	_ "github.com/lib/pq"
)

// connectPostgres establishes a connection with PostgreSQL using environment variables for configuration.
func connectPostgres() (*sql.DB, error) {
	user := getEnvOrDefault("POSTGRES_USER", "user")
	password := getEnvOrDefault("POSTGRES_PASSWORD", "password")
	dbname := getEnvOrDefault("POSTGRES_DB", "converter")
	host := getEnvOrDefault("POSTGRES_HOST", "host.docker.internal")
	sslmode := getEnvOrDefault("POSTGRES_SSL_MODE", "disable")

	connStr := fmt.Sprintf("user=%s password=%s dbname=%s host=%s sslmode=%s", user, password, dbname, host, sslmode)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		slog.Error("Failed to connect to PostgreSQL", slog.String("error", err.Error()))
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		slog.Error("Failed to ping PostgreSQL", slog.String("error", err.Error()))
		return nil, err
	}

	slog.Info("Connected to PostgreSQL successfully")
	return db, nil
}

// getEnvOrDefault fetches the value of an environment variable or returns a default value if it's not set.
func getEnvOrDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func main() {
	isDebug := getEnvOrDefault("DEBUG", "false") == "true"
	logger := log.NewLogger(isDebug)
	slog.SetDefault(logger)

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	db, err := connectPostgres()
	if err != nil {
		return
	}
	defer db.Close()

	videoConverter := converter.NewVideoConverter(db)

	msgs := make(chan []byte)
	go sendMsgs(5, msgs)

	var wg sync.WaitGroup
	go func() {
		for d := range msgs {
			wg.Add(1)
			go func(delivery []byte) {
				defer wg.Done()
				videoConverter.HandleMessage(delivery)
			}(d)
		}
	}()

	<-signalChan
	slog.Info("Shutdown signal received, finalizing processing...")

	wg.Wait()

	slog.Info("Processing completed, exiting...")
}

func sendMsgs(totalMsgs int, output chan []byte) {
	// generate json messages. Format: {"video_id": 1, "path": "/media/uploads/1"}
	for i := range totalMsgs {
		i++
		msg := fmt.Sprintf(`{"video_id": %d, "path": "/media/uploads/%d"}`, i, i)
		output <- []byte(msg)
	}
}
