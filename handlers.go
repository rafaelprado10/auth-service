package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Estrutura para o corpo da requisição de criação de chave
type CreateKeyRequest struct {
	Name string `json:"name"`
}

// Estrutura para a resposta da criação de chave
type CreateKeyResponse struct {
	Name    string `json:"name"`
	Key     string `json:"key"`
	Message string `json:"message"`
}

func (a *App) healthHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logInfo(ctx, "GET /health | health check")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		logError(ctx, "GET /health | erro ao serializar resposta: "+err.Error())
		http.Error(w, "Erro ao serializar resposta", http.StatusInternalServerError)
	}
}

func (a *App) validateKeyHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()
	logInfo(ctx, "GET /validate | iniciando validação de chave")

	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attribute.String("http.route", "/validate"))

	authHeader := r.Header.Get("Authorization")
	keyString := strings.TrimPrefix(authHeader, "Bearer ")

	if keyString == "" {
		logWarning(ctx, "GET /validate | Authorization header ausente")
		http.Error(w, "Authorization header não encontrado", http.StatusUnauthorized)
		return
	}

	keyHash := hashAPIKey(keyString)
	logInfo(ctx, "GET /validate | executando SELECT no PostgreSQL")

	var id int
	err := a.DB.QueryRow(
		"SELECT id FROM api_keys WHERE key_hash = $1 AND is_active = true",
		keyHash,
	).Scan(&id)
	if err != nil {
		logWarning(ctx, "GET /validate | chave inválida ou inativa")
		http.Error(w, "Chave de API inválida ou inativa", http.StatusUnauthorized)
		return
	}

	elapsedMs := time.Since(start).Milliseconds()
	span.SetAttributes(
		attribute.Int("auth.key_id", id),
		attribute.Int64("execution_time_ms", elapsedMs),
	)
	logInfo(ctx, fmt.Sprintf("GET /validate | chave válida | key_id=%d | elapsed_ms=%d", id, elapsedMs))

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"message": "Chave válida"}); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logError(ctx, "GET /validate | erro ao serializar resposta: "+err.Error())
		http.Error(w, "Erro ao serializar resposta", http.StatusInternalServerError)
	}
}

func (a *App) createKeyHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()
	logInfo(ctx, "POST /admin/keys | iniciando criação de chave")

	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attribute.String("http.route", "/admin/keys"))

	if r.Method != http.MethodPost {
		logWarning(ctx, "POST /admin/keys | método não permitido: "+r.Method)
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	var req CreateKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logWarning(ctx, "POST /admin/keys | corpo inválido: "+err.Error())
		http.Error(w, "Corpo da requisição inválido", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		logWarning(ctx, "POST /admin/keys | campo 'name' ausente")
		http.Error(w, "O campo 'name' é obrigatório", http.StatusBadRequest)
		return
	}

	span.SetAttributes(attribute.String("auth.key_name", req.Name))
	logInfo(ctx, "POST /admin/keys | key_name="+req.Name)

	newKey, err := generateAPIKey()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logError(ctx, "POST /admin/keys | erro ao gerar chave: "+err.Error())
		http.Error(w, "Erro ao gerar a chave", http.StatusInternalServerError)
		return
	}
	newKeyHash := hashAPIKey(newKey)

	logInfo(ctx, "POST /admin/keys | executando INSERT no PostgreSQL")
	var newID int
	err = a.DB.QueryRow(
		"INSERT INTO api_keys (name, key_hash) VALUES ($1, $2) RETURNING id",
		req.Name, newKeyHash,
	).Scan(&newID)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logError(ctx, "POST /admin/keys | erro ao salvar chave: "+err.Error())
		http.Error(w, "Erro ao salvar a chave", http.StatusInternalServerError)
		return
	}

	elapsedMs := time.Since(start).Milliseconds()
	span.SetAttributes(
		attribute.Int("auth.key_id", newID),
		attribute.Int64("execution_time_ms", elapsedMs),
	)
	logInfo(ctx, fmt.Sprintf(
		"POST /admin/keys | chave criada | key_id=%d | key_name=%s | elapsed_ms=%d",
		newID, req.Name, elapsedMs,
	))

	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(CreateKeyResponse{
		Name:    req.Name,
		Key:     newKey,
		Message: "Guarde esta chave com segurança! Você não poderá vê-la novamente.",
	}); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logError(ctx, "POST /admin/keys | erro ao serializar resposta: "+err.Error())
		http.Error(w, "Erro ao serializar resposta", http.StatusInternalServerError)
	}
}

func (a *App) masterKeyAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		authHeader := r.Header.Get("Authorization")
		keyString := strings.TrimPrefix(authHeader, "Bearer ")

		if keyString == "" {
			logWarning(ctx, r.Method+" "+r.URL.Path+" | Authorization header ausente")
			http.Error(w, "Acesso não autorizado", http.StatusForbidden)
			return
		}

		if keyString != a.MasterKey {
			logWarning(ctx, r.Method+" "+r.URL.Path+" | MASTER_KEY inválida")
			http.Error(w, "Acesso não autorizado", http.StatusForbidden)
			return
		}

		logInfo(ctx, r.Method+" "+r.URL.Path+" | MASTER_KEY validada")
		next.ServeHTTP(w, r)
	})
}
