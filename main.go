package main

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/joho/godotenv"

	"timetracking/posgresql"
	"timetracking/timetracking"
)

var Logger = slog.Default()

func main() {
	slog.SetLogLoggerLevel(slog.LevelDebug)

	Logger.Debug("Starting timetracking service")

	Logger.Debug("Loading posgresql config")
	pgconfig, err := loadPGConfig()
	if err != nil {
		Logger.Error("load posgresql config failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	Logger.Debug("New posgresql storage")
	db, err := posgresql.NewPosgresqlStorage(pgconfig)
	if err != nil {
		Logger.Error("new posgresql storage failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer db.Close()

	Logger.Debug("Setup handlers")

	fiberApp := fiber.New()
	groupTTS := fiberApp.Group("/")

	app := timetracking.NewTimeTrackingService(db)
	app.SetupHandlers(groupTTS)

	Logger.Debug("Starting server")
	if err := fiberApp.Listen(":3000"); err != nil {
		Logger.Error("fiber listen failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	Logger.Debug("Server stoped")
}

// malual load config from .env file
func loadPGConfig() (*posgresql.PsqlConfig, error) {
	err := godotenv.Load()
	if err != nil {
		return nil, fmt.Errorf("load .env file failed: %w", err)
	}

	host, okHost := os.LookupEnv("host")
	port, okPort := os.LookupEnv("port")
	username, okUsername := os.LookupEnv("username")
	password, okPassword := os.LookupEnv("password")
	database, okDatabase := os.LookupEnv("database")

	Logger.Debug("loaded .env file", "host", host, "port", port, "username", username, "password", password, "database", database)

	if !okHost || !okPort || !okUsername || !okPassword || !okDatabase {
		return nil, fmt.Errorf("load .env file failed: %w", err)
	}

	portInt, err := strconv.Atoi(port)
	if host == "" || portInt == 0 || err != nil || username == "" || password == "" || database == "" {
		return nil, fmt.Errorf("load .env file failed: %w", err)
	}

	Logger.Debug("loaded .env file typed", "host", host, "port", portInt, "username", username, "password", password, "database", database)

	return &posgresql.PsqlConfig{
		Host:     host,
		Port:     portInt,
		Username: username,
		Password: password,
		Database: database,
	}, nil
}
