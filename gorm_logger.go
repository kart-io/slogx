package slogx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	gormlogger "gorm.io/gorm/logger"
)

type GORMLogger struct {
	SlowThreshold time.Duration
	LogLevel      gormlogger.LogLevel
}

// NewGORMLogger returns a GORM logger interface that routes query metadata to slog.
// Interpolated SQL is intentionally omitted because it may contain credentials or user data.
func NewGORMLogger(slowThreshold time.Duration) gormlogger.Interface {
	if slowThreshold <= 0 {
		slowThreshold = 200 * time.Millisecond
	}
	return &GORMLogger{
		SlowThreshold: slowThreshold,
		LogLevel:      gormlogger.Info,
	}
}

func (l *GORMLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	newLogger := *l
	newLogger.LogLevel = level
	return &newLogger
}

func (l *GORMLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormlogger.Info {
		slog.InfoContext(ctx, fmt.Sprintf(msg, data...), "component", "gorm")
	}
}

func (l *GORMLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormlogger.Warn {
		slog.WarnContext(ctx, fmt.Sprintf(msg, data...), "component", "gorm")
	}
}

func (l *GORMLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormlogger.Error {
		slog.ErrorContext(ctx, fmt.Sprintf(msg, data...), "component", "gorm")
	}
}

func (l *GORMLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	if l.LogLevel <= gormlogger.Silent {
		return
	}

	elapsed := time.Since(begin)
	sql, rows := fc()

	attrs := []any{
		"component", "gorm",
		"duration_ms", float64(elapsed.Microseconds()) / 1000.0,
		"rows", rows,
		"statement", sqlStatement(sql),
		"sql_length", len(sql),
	}

	switch {
	case err != nil && !errors.Is(err, gormlogger.ErrRecordNotFound):
		attrs = append(attrs, "error", err)
		slog.ErrorContext(ctx, "gorm query error", attrs...)
	case elapsed > l.SlowThreshold && l.SlowThreshold != 0:
		attrs = append(attrs, "slow_query", true)
		slog.WarnContext(ctx, fmt.Sprintf("gorm slow query (> %v)", l.SlowThreshold), attrs...)
	case l.LogLevel >= gormlogger.Info:
		slog.DebugContext(ctx, "gorm query executed", attrs...)
	}
}

func sqlStatement(sql string) string {
	fields := strings.Fields(sql)
	if len(fields) == 0 {
		return "UNKNOWN"
	}
	statement := strings.ToUpper(fields[0])
	switch statement {
	case "SELECT", "INSERT", "UPDATE", "DELETE", "MERGE", "WITH",
		"CREATE", "ALTER", "DROP", "TRUNCATE",
		"BEGIN", "COMMIT", "ROLLBACK", "SAVEPOINT", "RELEASE",
		"EXPLAIN", "PRAGMA", "VACUUM", "ANALYZE":
		return statement
	default:
		return "OTHER"
	}
}
