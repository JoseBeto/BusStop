package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

type Route struct {
	Route_id    string
	Agency_id   int
	Route_label string
}

type RouteDirection struct {
	Direction_id   int
	Direction_name string
}

type PlaceCode struct {
	Place_code  string
	Description string
}

type RouteDepartures struct {
	Departures []Departure
}

type Departure struct {
	Departure_time int64
}

func main() {
	// Retrieve arguments. If an errMsg is returned, print it and exit
	busRoute, busStop, direction, errMsg := parseArgs()
	if errMsg != "" {
		fmt.Println(errMsg)
		return
	}
	// Given route, stop, and direction, print TimeTillNextBus (or error)
	fmt.Println(calculateTimeTillNextBus(busRoute, busStop, direction))
}

func parseArgs() (busRoute string, busStop string, direction string, errMsg string) {
	if len(os.Args) != 4 {
		return "", "", "", "Not enough arguments. Use: go run nextBus.go [BusRoute] [BusStop] [Direction]"
	}
	return os.Args[1], os.Args[2], os.Args[3], ""
}

/**
This function calculates the time till the next bus departs at the given bus stop going the given
	direction on the given bus route. If at any point there is an error, return an error message
Params:
	busRoute: 			int
	busStop:			string
	direction:			string
Returns:
	timeTillNextBus:	string
*/
func calculateTimeTillNextBus(busRoute string, busStop string, direction string) (timeTillNextBus string) {
	// Get Bus routes
	var routes []Route
	routes, err := getRoutes()
	if err != nil {
		return "Error retrieving routes: " + err.Error()
	}

	// Loop through routes and retrieve route that the user has requested
	var requestedRoute Route
	for _, route := range routes {
		if route.Route_label == busRoute {
			requestedRoute = route
			break
		}
	}

	// Return error if route not found
	if requestedRoute.Route_label == "" {
		return "Error: Route not found"
	}

	// Get direction_id of route given a direction
	direction_id, err := getBusDirectionID(requestedRoute.Route_id, direction)
	if err != nil {
		return "Error getting bus direction ID: " + err.Error()
	}

	// Get place_code of route given a direction_id and busStop name
	place_code, err := getBusStopPlaceCode(requestedRoute.Route_id, direction_id, busStop)
	if err != nil {
		return "Error getting bus direction ID: " + err.Error()
	}

	// Get timeTillNextBus of route given a direction_id and place_code
	timeTillNextBus, err = getTimeTillNextBus(requestedRoute.Route_id, direction_id, place_code)
	if err != nil {
		return "Error getting time till next bus stop: " + err.Error()
	}

	return
}

/**
This function returns bus routes
Returns:
	routes:	[]Route
	err:	error
*/
func getRoutes() (routes []Route, err error) {
	// Pull in bus routes
	resp, err := http.Get("https://svc.metrotransit.org/nextripv2/routes")
	if err != nil {
		return
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	err = json.Unmarshal(body, &routes)
	return
}

/**
This function, given a route, pulls in route directions. We then loop through the route directions
	and match with the requested direction. If the direction exists in this route, we return the
	direction_id. If no match is found, we return an error.
Params:
	route_id: 		int
	direction:		string
Returns:
	direction_id:	int
	err:			error
*/
func getBusDirectionID(route_id string, direction string) (direction_id int, err error) {
	// Pull in route directions given a route
	resp, err := http.Get(fmt.Sprintf("https://svc.metrotransit.org/nextripv2/directions/%s", route_id))
	if err != nil {
		return
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	var routeDirections []RouteDirection
	err = json.Unmarshal(body, &routeDirections)
	if err != nil {
		return
	}

	// Loop through routeDirections until direction_name contains direction. If not found, return error
	for _, routeDirection := range routeDirections {
		if strings.Contains(strings.ToLower(routeDirection.Direction_name), direction) {
			return routeDirection.Direction_id, nil
		}
	}
	return direction_id, errors.New("Route direction not found")
}

/**
This function, given a route and direction, pulls in placeCodes of the route. We then loop
	through the placesCodes and match with the requested busStop. If there is a match, we return the
	place_code. If no match is found, we return an error.
Params:
	route_id		string
	direction_id	int
	busStop			string
Returns:
	place_code		string
	err				error
*/
func getBusStopPlaceCode(route_id string, direction_id int, busStop string) (place_code string, err error) {
	// Pull in placeCodes given a route and direction
	resp, err := http.Get(fmt.Sprintf("https://svc.metrotransit.org/nextripv2/stops/%s/%d", route_id, direction_id))
	if err != nil {
		return
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	var placeCodes []PlaceCode
	err = json.Unmarshal(body, &placeCodes)
	if err != nil {
		return
	}

	// Loop through placeCodes until description matches busStop. If not found, return error
	for _, placeCode := range placeCodes {
		if strings.Contains(placeCode.Description, busStop) {
			return placeCode.Place_code, nil
		}
	}
	return place_code, errors.New("Bus stop place code not found")
}

/**
This function, given a route, direction_id, and place_code, pulls in routeDepartures. If no departures
	are available, we return an empty string. If a departure is available, we get the first (earliest)
	departure, calculate the time until the departure time, and return it.
Params:
	route_id			string
	direction_id		int
	place_code			string
Retruns:
	timeTillNextBusStop	string
	err					error
*/
func getTimeTillNextBus(route_id string, direction_id int, place_code string) (timeTillNextBusStop string, err error) {
	resp, err := http.Get(fmt.Sprintf("https://svc.metrotransit.org/nextripv2/%s/%d/%s", route_id, direction_id, place_code))
	if err != nil {
		return
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	var routeDepartures RouteDepartures
	err = json.Unmarshal(body, &routeDepartures)
	if err != nil {
		return
	}

	// Return empty string if no departures are scheduled
	if len(routeDepartures.Departures) == 0 {
		return "", err
	}

	// Get earliest departure and calculate minutes until departure_time
	departure_time := time.Unix(routeDepartures.Departures[0].Departure_time, 0)
	currentTime := time.Now()
	diff := departure_time.Sub(currentTime)

	return fmt.Sprintf("%d Minutes", int(diff.Minutes())), nil
}
