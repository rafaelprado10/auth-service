package main

import (
	"context"
	"log"
	"os"
	"time"

	"go.opentelemetry.io/otel/trace"
)

type ctxKey string

const (
	ctxKeyMessageID ctxKey = "message_id"
	ctxKeyStartTime ctxKey = "start_time"
)

func serviceName() string {
	if v := os.Getenv("OTEL_SERVICE_NAME"); v != "" {
		return v
	}
	return "auth-service"
}

func logLine(ctx context.Context, message string) {
	msgID := "-"
	traceID := "-"
	ts := time.Now().Format("2006-01-02T15:04:05.000000")

	if ctx != nil {
		if v, ok := ctx.Value(ctxKeyMessageID).(string); ok && v != "" {
			msgID = v
		}
		span := trace.SpanFromContext(ctx)
		if sc := span.SpanContext(); sc.IsValid() {
			traceID = sc.TraceID().String()
		}
	}

	log.Printf(" %s | %s | %s | %s | %s", ts, msgID, traceID, serviceName(), message)
}

func logInfo(ctx context.Context, message string) {
	logLine(ctx, message)
}

func logWarning(ctx context.Context, message string) {
	logLine(ctx, message)
}

func logError(ctx context.Context, message string) {
	logLine(ctx, message)
}

func logCritical(ctx context.Context, message string) {
	logLine(ctx, message)
}
