package db

import (
	"database/sql"
	"log/slog"
)

func closeRows(rows *sql.Rows) {
	if rows == nil {
		return
	}
	if err := rows.Close(); err != nil {
		slog.Error("sql rows close failed", "error", err)
	}
}
