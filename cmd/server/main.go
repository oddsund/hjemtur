package main

import (
	"log/slog"
	"os"

	handler "github.com/oddsund/hjemtur"
	"github.com/scaleway/serverless-functions-go/local"
)

func main() {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug, AddSource: true})
	slog.SetDefault(slog.New(h))
	local.ServeHandler(handler.Handle, local.WithPort(8080))
}
