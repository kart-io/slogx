package slogx

import (
	"os"
	"strings"
)

// resolveEnv resolves the execution environment (user value, WECHAT_LOGGING_ENV/APP_ENV/ENV/WECHAT_SERVER_MODE envs, or fallback).
func resolveEnv(userEnv string) string {
	if trimmed := strings.TrimSpace(userEnv); trimmed != "" {
		return trimmed
	}
	if env := os.Getenv("WECHAT_LOGGING_ENV"); env != "" {
		return strings.TrimSpace(env)
	}
	if env := os.Getenv("APP_ENV"); env != "" {
		return strings.TrimSpace(env)
	}
	if env := os.Getenv("ENV"); env != "" {
		return strings.TrimSpace(env)
	}
	if env := os.Getenv("WECHAT_SERVER_MODE"); env != "" {
		return strings.TrimSpace(env)
	}
	return "development"
}
