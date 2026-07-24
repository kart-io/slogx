package slogx

import (
	"context"
	"log/slog"
	"net"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisHook struct{}

// NewRedisHook returns a Redis hook that forwards command metadata and errors to slog.
// Command arguments are intentionally omitted because they may contain credentials or user data.
func NewRedisHook() redis.Hook {
	return &RedisHook{}
}

func (h *RedisHook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := next(ctx, network, addr)
		if err != nil {
			slog.ErrorContext(ctx, "redis dial failed", "component", "redis", "addr", addr, "error", err)
		}
		return conn, err
	}
}

func (h *RedisHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmd)
		elapsed := time.Since(start)

		if err != nil && err != redis.Nil {
			slog.ErrorContext(ctx, "redis command failed",
				"component", "redis",
				"cmd", cmd.Name(),
				"arg_count", redisArgumentCount(cmd),
				"duration_ms", float64(elapsed.Microseconds())/1000.0,
				"error", err,
			)
		}
		return err
	}
}

func redisArgumentCount(cmd redis.Cmder) int {
	count := len(cmd.Args()) - 1
	if count < 0 {
		return 0
	}
	return count
}

func (h *RedisHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmds)
		elapsed := time.Since(start)

		if err != nil && err != redis.Nil {
			slog.ErrorContext(ctx, "redis pipeline failed",
				"component", "redis",
				"pipeline_cmds_count", len(cmds),
				"duration_ms", float64(elapsed.Microseconds())/1000.0,
				"error", err,
			)
		}
		return err
	}
}
