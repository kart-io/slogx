package slogx

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

var builderPool = sync.Pool{
	New: func() interface{} {
		return new(strings.Builder)
	},
}

type callerHandler struct {
	slog.Handler
	addSource bool
}

func newCallerHandler(h slog.Handler, addSource bool) slog.Handler {
	return &callerHandler{
		Handler:   h,
		addSource: addSource,
	}
}

func (h *callerHandler) Handle(ctx context.Context, r slog.Record) error {
	if (r.Level >= slog.LevelError || h.addSource) && r.PC != 0 {
		fs := runtime.CallersFrames([]uintptr{r.PC})
		f, _ := fs.Next()
		if f.File != "" {
			dir, file := filepath.Split(f.File)
			parent := filepath.Base(dir)
			short := file
			if parent != "" && parent != "." && parent != "/" {
				short = filepath.Join(parent, file)
			}
			callerStr := fmt.Sprintf("%s:%d", short, f.Line)
			r.AddAttrs(slog.String("caller", callerStr))
		}
	}

	if r.Level >= slog.LevelError {
		stack := captureStacktrace()
		if stack != "" {
			r.AddAttrs(slog.String("stacktrace", stack))
		}
	}

	cleanRecord := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		redacted := redactAttr(nil, a)
		if redacted.Key != "" {
			cleanRecord.AddAttrs(redacted)
		}
		return true
	})

	return h.Handler.Handle(ctx, cleanRecord)
}

func captureStacktrace() string {
	pcs := make([]uintptr, 64)
	n := runtime.Callers(1, pcs)
	if n == 0 {
		return ""
	}

	frames := runtime.CallersFrames(pcs[:n])
	sb := builderPool.Get().(*strings.Builder)
	sb.Reset()
	defer builderPool.Put(sb)

	for {
		frame, more := frames.Next()
		if frame.Function != "" &&
			!strings.HasPrefix(frame.Function, "runtime.") &&
			!strings.HasPrefix(frame.Function, "log/slog.") &&
			!strings.Contains(frame.Function, "pkg/logging.captureStacktrace") &&
			!strings.Contains(frame.Function, "pkg/logging.(*callerHandler)") {
			dir, file := filepath.Split(frame.File)
			parent := filepath.Base(dir)
			short := file
			if parent != "" && parent != "." && parent != "/" {
				short = filepath.Join(parent, file)
			}
			fmt.Fprintf(sb, "%s\n\t%s:%d\n", frame.Function, short, frame.Line)
		}
		if !more {
			break
		}
	}
	return strings.TrimSpace(sb.String())
}

func (h *callerHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &callerHandler{
		Handler:   h.Handler.WithAttrs(attrs),
		addSource: h.addSource,
	}
}

func (h *callerHandler) WithGroup(name string) slog.Handler {
	return &callerHandler{
		Handler:   h.Handler.WithGroup(name),
		addSource: h.addSource,
	}
}
