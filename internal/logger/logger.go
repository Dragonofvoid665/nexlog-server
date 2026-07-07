// Package logger provides structured JSON logging over stdlib log/slog.
// Uses log/slog (stdlib since Go 1.21) — zero external dependencies.
package logger

import (
	"context"
	"log/slog"
	"os"
)

var L *slog.Logger

func Init(isDev bool) {
	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: slog.LevelDebug}
	if isDev {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}
	L = slog.New(handler)
	slog.SetDefault(L)
}

func Info(msg string, args ...any)  { L.Info(msg, args...) }
func Warn(msg string, args ...any)  { L.Warn(msg, args...) }
func Error(msg string, args ...any) { L.Error(msg, args...) }
func Debug(msg string, args ...any) { L.Debug(msg, args...) }

func FromCtx(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok {
		return l
	}
	return L
}

type ctxKey struct{}

func WithCtx(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}
