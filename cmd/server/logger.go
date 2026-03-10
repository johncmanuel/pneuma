package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
)

// consoleHandler is a minimal slog.Handler that emits log lines in the form:
//
//	[2006-01-02T15:04:05.000-07:00] [LEVEL]: msg  key=val  key=val
type consoleHandler struct {
	mu    sync.Mutex
	w     io.Writer
	level slog.Level
	attrs []slog.Attr
	group string
}

func newConsoleHandler(w io.Writer, level slog.Level) slog.Handler {
	return &consoleHandler{w: w, level: level}
}

func (h *consoleHandler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.level
}

func (h *consoleHandler) Handle(_ context.Context, r slog.Record) error {
	var buf bytes.Buffer

	// Timestamp
	fmt.Fprintf(&buf, "[%s] ", r.Time.Format("2006-01-02T15:04:05.000-07:00"))

	// Level
	fmt.Fprintf(&buf, "[%s]: ", r.Level.String())

	// Message
	buf.WriteString(r.Message)

	// Handler-level attrs (from WithAttrs)
	for _, a := range h.attrs {
		fmt.Fprintf(&buf, "  %s=%v", fmtKey(h.group, a.Key), a.Value)
	}

	// Record attrs
	r.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(&buf, "  %s=%v", fmtKey(h.group, a.Key), a.Value)
		return true
	})

	buf.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.w.Write(buf.Bytes())
	return err
}

func (h *consoleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	cp := h
	cp.attrs = append(cp.attrs, attrs...)
	return cp
}

func (h *consoleHandler) WithGroup(name string) slog.Handler {
	cp := h
	if cp.group != "" {
		cp.group += "." + name
	} else {
		cp.group = name
	}
	return cp
}

func fmtKey(group, key string) string {
	if group != "" {
		return group + "." + key
	}
	return key
}
