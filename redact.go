package slogx

import (
	"context"
	"log/slog"
	"strings"
)

var defaultSensitiveKeys = map[string]struct{}{
	"authorization": {}, "cookie": {}, "password": {}, "secret": {}, "token": {},
	"access_token": {}, "refresh_token": {}, "api_key": {}, "service_token": {}, "jwt": {},
}

// redactAttr redacts sensitive log attributes such as passwords, tokens, and secrets.
func redactAttr(groups []string, attr slog.Attr) slog.Attr {
	if attr.Key == slog.SourceKey {
		return slog.Attr{}
	}
	key := strings.ToLower(strings.TrimSpace(attr.Key))
	if _, ok := defaultSensitiveKeys[key]; ok ||
		strings.HasSuffix(key, "_password") ||
		strings.HasSuffix(key, "_secret") ||
		strings.HasSuffix(key, "_token") ||
		strings.HasSuffix(key, "_api_key") {
		return slog.String(attr.Key, "[REDACTED]")
	}
	value := attr.Value.Resolve()
	if value.Kind() == slog.KindGroup {
		return slog.Attr{Key: attr.Key, Value: slog.GroupValue(redactAttrs(value.Group())...)}
	}
	attr.Value = value
	return attr
}

func redactAttrs(attrs []slog.Attr) []slog.Attr {
	redacted := make([]slog.Attr, 0, len(attrs))
	for _, attr := range attrs {
		redacted = append(redacted, redactAttr(nil, attr))
	}
	return redacted
}

type redactingHandler struct {
	next slog.Handler
}

func newRedactingHandler(next slog.Handler) slog.Handler {
	return &redactingHandler{next: next}
}

func (h *redactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *redactingHandler) Handle(ctx context.Context, record slog.Record) error {
	redacted := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	record.Attrs(func(attr slog.Attr) bool {
		redacted.AddAttrs(redactAttr(nil, attr))
		return true
	})
	return h.next.Handle(ctx, redacted)
}

func (h *redactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &redactingHandler{next: h.next.WithAttrs(redactAttrs(attrs))}
}

func (h *redactingHandler) WithGroup(name string) slog.Handler {
	return &redactingHandler{next: h.next.WithGroup(name)}
}
