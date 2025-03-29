package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"
	_ "time/tzdata"
)

func Handle(w http.ResponseWriter, r *http.Request) {
	clientName := os.Getenv("CLIENT_NAME")
	decoder := json.NewDecoder(r.Body)
	var request TurRequest
	err := decoder.Decode(&request)
	if err != nil {
		panic(err)
	}
	slog.Default().Info("Got request", "json", request)

	svar, err := HentCaHjemtidspunkt(request, clientName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(svar)
}

func HentCaHjemtidspunkt(tur TurRequest, clientName string) (TravelDetails, error) {
	logger := slog.Default()

	var currentTime string
	if tur.Debug {
		today := time.Now()
		nextDay := today.AddDate(0, 0, 1)
		for nextDay.Weekday() == time.Saturday || nextDay.Weekday() == time.Sunday {
			nextDay = nextDay.AddDate(0, 0, 1)
		}
		location, err := time.LoadLocation("Europe/Oslo")
		if err != nil {
			logger.Error("Feil ved lasting av tidssone:", "err", err)
			return TravelDetails{}, err
		}
		currentTime = time.Date(
			nextDay.Year(), nextDay.Month(), nextDay.Day(),
			16, 52, 00, 00,
			location,
		).Format("2006-01-02T15:04:05.000-07:00")
	} else {
		currentTime = time.Now().Format("2006-01-02T15:04:05.000-07:00")
	}

	logger.Info("Fikk et kall", "turRequest", tur)

	data := []byte("{\"query\":\"{trip(" +
		fmt.Sprintf("from:{coordinates:{latitude:%s longitude:%s}} ", tur.From.Latitude, tur.From.Longitude) +
		fmt.Sprintf("to:{coordinates:{latitude:%s longitude:%s}} ", tur.To.Latitude, tur.To.Longitude) +
		fmt.Sprintf(`numTripPatterns:3 dateTime:\"%s\" walkSpeed:1.3 `, currentTime) +
		"arriveBy:false modes:{accessMode:bike_rental egressMode:foot transportModes:{transportMode:bus}} " +
		"bicycleOptimisationMethod:quick){tripPatterns{expectedStartTime duration expectedEndTime endTime " +
		"aimedEndTime directDuration legs{mode distance bikeRentalNetworks duration fromPlace{latitude longitude name " +
		"bikeRentalStation{bikesAvailable id}} expectedEndTime expectedStartTime line{id publicCode " +
		" transportMode} toPlace{name bikeRentalStation{bikesAvailable spacesAvailable id}}}}}}\"}")

	logger.Debug(string(data))
	client := &http.Client{}
	req, err := http.NewRequest("POST", "https://api.entur.io/journey-planner/v3/graphql", bytes.NewBuffer(data))
	if err != nil {
		logger.Debug("Got error while creating request", "err", err)
		return TravelDetails{}, err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("ET-Client-Name", clientName)

	logger.Debug("Calling entur")
	resp, err := client.Do(req)
	if err != nil {
		logger.Debug("Got error from entur", "err", err)
		return TravelDetails{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Debug("Couldn't deserialize entur response", "err", err, "body", string(body))
		return TravelDetails{}, err
	}
	logger.Debug("Response from entur", "resp", string(body))
	var trips JourneyPlannerResponse
	err = json.Unmarshal(body, &trips)
	if err != nil {
		logger.Debug("Couldn't deserialize entur response", "err", err, "body", string(body))
		return TravelDetails{}, err
	}
	logger.Debug("Entur svar", "svar", trips)

	logger.Debug("Alternative ankomsttider")
	alternativeTidspunkter := ""
	nextTrip := ""
	allTrips := ""
	for i := range trips.Data.Trip.TripPatterns {
		trip := trips.Data.Trip.TripPatterns[i]
		logger.Debug(fmt.Sprintf("%s\n", trip.ExpectedEndTime))
		route := ""
		availableBikes := ""
		availableLocks := ""
		for j := range trip.Legs {
			leg := trip.Legs[j]
			if leg.Mode == "foot" || leg.Mode == "bicycle" {
				route += fmt.Sprintf("%s(%s) - ", leg.FromPlace.Name, leg.Mode)
			} else {
				route += fmt.Sprintf("%s(%s, %s) - ", leg.FromPlace.Name, leg.Line.PublicCode, leg.Line.TransportMode)
			}
			if leg.Mode == "bicycle" {
				availableBikes += fmt.Sprintf("%s(%d) ", leg.FromPlace.Name, leg.FromPlace.BikeRentalStation.BikesAvailable)
				availableLocks += fmt.Sprintf("%s(%d) ", leg.ToPlace.Name, leg.ToPlace.BikeRentalStation.SpacesAvailable)
			}
		}
		route += "Fremme!"
		tripInfo := fmt.Sprintf(
			"Seneste avreise: %s\n"+
				"Ledige sykler: %s\n"+
				"Ledige låser: %s\n"+
				"Rute: %s\n",
			trip.ExpectedStartTime.Format("15:04"),
			availableBikes,
			availableLocks,
			route,
		)
		if i == 0 {
			nextTrip = tripInfo
		} else {
			alternativeTidspunkter += fmt.Sprintf("%s, ", trip.ExpectedEndTime.Format("15:04"))
		}
		allTrips += tripInfo + "\n"
	}

	if len(trips.Data.Trip.TripPatterns) == 0 {
		return TravelDetails{}, err
	} else {
		hjemtid := fmt.Sprintf("Hjemme ca %s(alternativt %s)", trips.Data.Trip.TripPatterns[0].ExpectedEndTime.Format("15:04"), alternativeTidspunkter)
		logger.Debug(hjemtid)
		return TravelDetails{
			Sms:      hjemtid,
			NextTrip: nextTrip,
			AllTrips: allTrips,
		}, nil
	}

}

type TurRequest struct {
	From struct {
		Longitude string `json:"longitude"`
		Latitude  string `json:"latitude"`
	} `json:"from"`
	To struct {
		Longitude string `json:"longitude"`
		Latitude  string `json:"latitude"`
	} `json:"to"`
	Debug bool `json:"debug"`
}

type Availability struct {
	Name   string
	Amount int
}

type TravelDetails struct {
	Sms      string
	NextTrip string
	AllTrips string
}

type JourneyPlannerResponse struct {
	Data struct {
		Trip struct {
			TripPatterns []struct {
				ExpectedStartTime time.Time `json:"expectedStartTime"`
				Duration          int       `json:"duration"`
				ExpectedEndTime   time.Time `json:"expectedEndTime"`
				EndTime           time.Time `json:"endTime"`
				AimedEndTime      time.Time `json:"aimedEndTime"`
				DirectDuration    int       `json:"directDuration"`
				Legs              []struct {
					Mode               string        `json:"mode"`
					Distance           float64       `json:"distance"`
					BikeRentalNetworks []interface{} `json:"bikeRentalNetworks"`
					Duration           int           `json:"duration"`
					FromPlace          struct {
						Latitude          float64 `json:"latitude"`
						Longitude         float64 `json:"longitude"`
						Name              string  `json:"name"`
						BikeRentalStation struct {
							BikesAvailable int    `json:"bikesAvailable"`
							Id             string `json:"id"`
						} `json:"bikeRentalStation"`
					} `json:"fromPlace"`
					ExpectedEndTime   time.Time `json:"expectedEndTime"`
					ExpectedStartTime time.Time `json:"expectedStartTime"`
					Line              struct {
						Id            string `json:"id"`
						PublicCode    string `json:"publicCode"`
						TransportMode string `json:"transportMode"`
					} `json:"line"`
					ToPlace struct {
						Name              string `json:"name"`
						BikeRentalStation struct {
							SpacesAvailable int    `json:"spacesAvailable"`
							Id              string `json:"id"`
						} `json:"bikeRentalStation"`
					} `json:"toPlace"`
				} `json:"legs"`
			} `json:"tripPatterns"`
		} `json:"trip"`
	} `json:"data"`
}
