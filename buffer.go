package main

import (
	"fmt"
	"math"
)

type Point struct {
	Lat float64
	Lng float64
}

func (p *Point) Buffer(distance float64, increment float64) [][]float64 {
	// Earth radius in kilometers
	earthRadius := 6371.0

	// Convert distance from meters to kilometers
	distance /= 1000.0

	var coords [][]float64

	for angle := 0.0; angle < 360.0; angle += increment {
		// Convert angle from degrees to radians
		angleRad := angle * math.Pi / 180.0

		// Calculate latitude and longitude offsets using Haversine formula
		latOffset := distance * math.Cos(angleRad) / earthRadius
		lngOffset := distance * math.Sin(angleRad) / (earthRadius * math.Cos(p.Lat*math.Pi/180.0))

		// Calculate new latitude and longitude
		newLat := p.Lat + latOffset * 180.0 / math.Pi
		newLng := p.Lng + lngOffset * 180.0 / math.Pi

		coords = append(coords, []float64{newLng, newLat})
	}

	coords = append(coords, coords[0])
	return coords
}

func main() {
	point := &Point{Lat: 42.328200874288484, Lng: -83.0750883155644} // AirspaceLink
	bufferedCoords := point.Buffer(3218.69, 1)       // Buffer 2 miles (in meters), 1 degree increment

	// Output
	fmt.Print("[")
	for i, coord := range bufferedCoords {
		fmt.Printf("[%f, %f]", coord[0], coord[1])
		if i < len(bufferedCoords)-1 {
			fmt.Print(", ")
		}
	}
	fmt.Print("]")
}
