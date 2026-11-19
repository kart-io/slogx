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

	// PC 0 means the record has no call site; the stack here would only show
	// the code that built it.
	if r.Level >= slog.LevelError && r.PC != 0 {
		stack := captureStacktrace()
		if stack != "" {
			r.AddAttrs(slog.String("stacktrace", stack))
		}
	}

	return h.Handler.Handle(ctx, r)
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
		if frame.Function != "" && !isInternalFrame(frame.Function) {
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

func isInternalFrame(name string) bool {
	return strings.HasPrefix(name, "runtime.") ||
		strings.HasPrefix(name, "log/slog.") ||
		strings.Contains(name, "github.com/kart-io/slogx.captureStacktrace") ||
		strings.Contains(name, "github.com/kart-io/slogx.(*callerHandler)")
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
