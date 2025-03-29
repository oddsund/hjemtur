package main

import (
	"flag"
	"log/slog"
	"os"
	"strconv"

	handler "github.com/oddsund/hjemtur"
)

type Location struct {
	Longitude string `json:"longitude"`
	Latitude  string `json:"latitude"`
}

func main() {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug, AddSource: true})
	slog.SetDefault(slog.New(h))
	logger := slog.Default()

	fromLongitude := flag.String("from-long", "", "Longitude for starting location")
	fromLatitude := flag.String("from-lat", "", "Latitude for starting location")
	toLongitude := flag.String("to-long", "", "Longitude for destination location")
	toLatitude := flag.String("to-lat", "", "Latitude for destination location")
	clientName := flag.String("client-name", "", "Value to be sent in the ET-Client-Name header")

	flag.Parse()

	if *fromLongitude == "" || *fromLatitude == "" || *toLongitude == "" || *toLatitude == "" || *clientName == "" {
		flag.Usage()
		return
	}
	if _, err := strconv.ParseFloat(*fromLongitude, 64); err != nil {
		logger.Error("Invalid longitude for 'from'", "value", fromLongitude)
		return
	}
	if _, err := strconv.ParseFloat(*fromLatitude, 64); err != nil {
		logger.Error("Invalid latitude for 'from'", "value", fromLatitude)
		return
	}
	if _, err := strconv.ParseFloat(*toLongitude, 64); err != nil {
		logger.Error("Invalid longitude for 'to'", "value", toLongitude)
		return
	}
	if _, err := strconv.ParseFloat(*toLatitude, 64); err != nil {
		logger.Error("Invalid latitude for 'to'", "value", toLatitude)
		return
	}

	svar, err := handler.HentCaHjemtidspunkt(handler.TurRequest{
		Debug: true,
		From: Location{
			Longitude: *fromLongitude,
			Latitude:  *fromLatitude,
		},
		To: Location{
			Longitude: *toLongitude,
			Latitude:  *toLatitude,
		},
	}, *clientName)
	logger.Info("svar", "json", svar)
	logger.Info("feil", "err", err)
}
