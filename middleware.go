package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type statusRecorder struct {
	http.ResponseWriter
	status    int
	messageID string
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.status == 0 {
		r.status = code
	}
	r.ResponseWriter.Header().Set("X-Request-ID", r.messageID)
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}

func requestContextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		msgID := r.Header.Get("X-Request-ID")
		if msgID == "" {
			msgID = uuid.New().String()
		}

		start := time.Now()
		ctx := context.WithValue(r.Context(), ctxKeyMessageID, msgID)
		ctx = context.WithValue(ctx, ctxKeyStartTime, start)

		logInfo(ctx, fmt.Sprintf("%s %s | início da requisição", r.Method, r.URL.Path))

		rec := &statusRecorder{
			ResponseWriter: w,
			messageID:      msgID,
		}
		next.ServeHTTP(rec, r.WithContext(ctx))

		if rec.status == 0 {
			rec.status = http.StatusOK
		}
		elapsedMs := time.Since(start).Milliseconds()
		logInfo(ctx, fmt.Sprintf(
			"%s %s | status=%d | elapsed_ms=%d",
			r.Method, r.URL.Path, rec.status, elapsedMs,
		))
	})
}
