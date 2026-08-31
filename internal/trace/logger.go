package trace

import (
	"context"
	"io"
	"log/slog"
	"os"
	"time"
)

type Logger struct {
	*slog.Logger
}

func New(w io.Writer, level slog.Level) *Logger {
	if w == nil {
		w = os.Stdout
	}
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.String("timestamp", a.Value.Time().Format(time.RFC3339Nano))
			}
			return a
		},
	})
	return &Logger{slog.New(handler)}
}

func (l *Logger) LLMCall(ctx context.Context, fields map[string]any) {
	l.InfoContext(ctx, "llm_call", attrsFromMap(fields)...)
}

func (l *Logger) StateTransition(ctx context.Context, userID int64, from, to, trigger string) {
	l.InfoContext(ctx, "state_transition",
		slog.Int64("user_id", userID),
		slog.String("from_state", from),
		slog.String("to_state", to),
		slog.String("trigger", trigger),
	)
}

func (l *Logger) DomainEvent(ctx context.Context, event string, fields map[string]any) {
	args := []any{slog.String("event", event)}
	args = append(args, attrsFromMap(fields)...)
	l.InfoContext(ctx, "domain_event", args...)
}

func attrsFromMap(m map[string]any) []any {
	args := make([]any, 0, len(m)*2)
	for k, v := range m {
		args = append(args, slog.Any(k, v))
	}
	return args
}
