package nutrition

import (
	"io"
	"log/slog"
)

func closeBody(body io.Closer) {
	if body == nil {
		return
	}
	if err := body.Close(); err != nil {
		slog.Error("http response body close failed", "error", err)
	}
}
