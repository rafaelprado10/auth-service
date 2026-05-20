package main

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"time"

	_ "github.com/jackc/pgx/v4/stdlib"
	"github.com/joho/godotenv"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// App struct (para injeção de dependência)
type App struct {
	DB        *sql.DB
	MasterKey string
}

func main() {
	ctx := context.Background()

	_ = godotenv.Load()

	shutdown, err := initTelemetry(ctx)
	if err != nil {
		logWarning(ctx, "OpenTelemetry desabilitado ou falhou ao iniciar: "+err.Error())
	} else {
		defer func() {
			_ = shutdown(context.Background())
		}()
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8001"
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		logCritical(ctx, "DATABASE_URL deve ser definida")
		os.Exit(1)
	}

	masterKey := os.Getenv("MASTER_KEY")
	if masterKey == "" {
		logCritical(ctx, "MASTER_KEY deve ser definida")
		os.Exit(1)
	}

	db, err := connectDB(ctx, databaseURL)
	if err != nil {
		logCritical(ctx, "Não foi possível conectar ao banco de dados: "+err.Error())
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	app := &App{
		DB:        db,
		MasterKey: masterKey,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", app.healthHandler)
	mux.HandleFunc("/validate", app.validateKeyHandler)
	mux.Handle(
		"/admin/keys",
		app.masterKeyAuthMiddleware(http.HandlerFunc(app.createKeyHandler)),
	)

	handler := otelhttp.NewHandler(
		requestContextMiddleware(mux),
		serviceName(),
	)

	logInfo(ctx, "Serviço de Autenticação (Go) iniciado na porta "+port)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		logCritical(ctx, err.Error())
		os.Exit(1)
	}
}

func connectDB(ctx context.Context, databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}

	if err = db.Ping(); err != nil {
		return nil, err
	}

	logInfo(ctx, "Conectado ao PostgreSQL com sucesso")
	return db, nil
}
