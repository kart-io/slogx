package slogx

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

// The package-level functions and a *Logger from Init must write to the same
// place: a program that mixes them (Init in main, slogx.Warn in a package)
// would otherwise lose one side.
func TestFacadeWritesThroughTheDefaultLogger(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	t.Cleanup(func() { slog.SetDefault(old) })

	SetDefault(FromHandler(NewJSONHandler(&buf, &HandlerOptions{Level: LevelDebug})))
	Debug("d", String("k", "v"))
	Info("i", Int("n", 1))
	Warn("w", Any("x", []int{1}))
	Error("e", Bool("ok", false))
	InfoContext(context.Background(), "ic")
	Log(context.Background(), LevelWarn, "l", Duration("d", 0))
	With("scope", "s").Info("with")
	Default().WithGroup("g").Info("grouped", "a", 1)

	out := buf.String()
	for _, want := range []string{
		`"msg":"d"`, `"k":"v"`, `"msg":"i"`, `"n":1`, `"msg":"w"`, `"x":[1]`, `"msg":"e"`, `"ok":false`,
		`"msg":"ic"`, `"msg":"l"`, `"scope":"s"`, `"g":{"a":1}`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %s:\n%s", want, out)
		}
	}
}

// A derived logger keeps the parent's shutdown hook: Init's OTel flush must
// still run when the program only kept the With()-derived child.
func TestDerivedLoggerKeepsShutdown(t *testing.T) {
	called := false
	parent := &Logger{Logger: slog.Default(), shutdownFunc: func(context.Context) error { called = true; return nil }}
	child := parent.With("k", "v").WithGroup("g")
	if err := child.Shutdown(context.Background()); err != nil || !called {
		t.Fatalf("shutdown called=%v err=%v", called, err)
	}
}

// Wrap(nil) and SetDefault(nil) must not panic or clear the default.
func TestNilSafety(t *testing.T) {
	old := slog.Default()
	t.Cleanup(func() { slog.SetDefault(old) })
	SetDefault(nil)
	SetDefault(&Logger{})
	if Wrap(nil).Logger != slog.Default() {
		t.Fatal("Wrap(nil) should adopt the default logger")
	}
}
