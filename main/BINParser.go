package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"time"

	"github.com/edancain/telemetry_parser/fileparser"

	"github.com/peterstace/simplefeatures/geom"
)

const (
	ScalingFactorGPS   = 1e7
	numCoordsPerRecord = 2
)

// GeoJSONFeature represents a GeoJSON feature with associated properties.
type GeoJSONFeature struct {
	Geometry       geom.Geometry          `json:"geometry"`
	ID             interface{}            `json:"id,omitempty"`
	Properties     map[string]interface{} `json:"properties,omitempty"`
	ForeignMembers map[string]interface{} `json:"-"`
}

// ConvertToGeoJSONFeature converts a geom.Geometry object into a GeoJSONFeature.
func ConvertToGeoJSONFeature(g geom.Geometry, id interface{}, properties map[string]interface{}) GeoJSONFeature {
	return GeoJSONFeature{
		Geometry:   g,
		ID:         id,
		Properties: properties,
	}
}

type BINParser struct {
}

func (p *BINParser) ParseGeometry(r io.Reader) (*geom.Geometry, error) {
	data, err := extractData(r)

	if err != nil {
		return nil, fmt.Errorf("failed to parse data: %v", err)
	}

	// how to get the datetime out of the data
	firstElement := data[0]
	gpsTime := getTimeFromGPS(firstElement)
	fmt.Println("GPS Time:", gpsTime)
	fmt.Println("Total unique GPS messages:", len(data))

	return createGeometry(data)
}

func extractData(file io.Reader) ([]map[string]interface{}, error) {
	var data []map[string]interface{}
	var zeroTimeBase = false

	dfreader, err := fileparser.NewBinaryDataFileReader(file, zeroTimeBase)
	if err != nil {
		return nil, fmt.Errorf("failed to create binary data file reader: %v", err)
	}

	if _, ok := dfreader.Messages["GPS"]; !ok {
		return nil, fmt.Errorf("no GPS data found in file")
	}

	datafileMessage, _ := dfreader.ParseNext()

	//currentMessages := dfreader.Messages
	messageCount := 0

	// Create a set to store seen times. This is used to test whether the time value has been
	// seen before. If it hasn't it is added and the GPS data is added to data map. Otherwise
	// we will have multiple copies of the same position. This is due to the write rate for
	// other sensors within the Pixhawk. i.e. IMU.
	seenTimes := make(map[int]bool)
	// Iterate over all messages

	for datafileMessage != nil {
		messageCount++
		if gpsValues, ok := dfreader.Messages["GPS"]; ok {
			processGPSValues(gpsValues, seenTimes, &data)
		}

		datafileMessage, err = dfreader.ParseNext()
		if err != nil {
			break //EOF
		}

		fmt.Printf("%.1f%%\n", dfreader.Percent)
	}
	return data, nil

	/* Iterate over all messages
	for msg != nil {
		messageCount++
		if gpsValues, ok := msg["GPS"]; ok {

			lat := 1 // float64(gpsValues.GetAttr("Lat").(int)) / 1e7
			lon := 1 // float64(gpsValues.GetAttr("Lng").(int)) / 1e7

			if lat != 0 && lon != 0{
				// Create a dictionary from fieldnames and values
				entryDict := make(map[string]interface{})
				for i, field := range gpsValues.FieldNames {
					if field == "Lat" || field == "Lng" {
						entryDict[field] = float64(gpsValues.Elements[i].(int)) / 1e7
					} else {
						entryDict[field] = gpsValues.Elements[i]
					}
				}

				// Check if this time has been seen before
				time := entryDict["TimeMS"].(int)
				if !seenTimes[time] {
					// Add this time to the set of seen times
					seenTimes[time] = true
					data = append(data, entryDict)
					count++
				}
			}
		}

		// Get the next message
		dfreader.ParseNext()
		msg = dfreader.Messages
		fmt.Printf("%.1f%%\n", dfreader.Percent)
		fmt.Printf("%d unique records\n", count)
		fmt.Printf("\n")
		if dfreader.Percent > 99.99 {
			break
		}
	}

	fmt.Println("Total messages:", messageCount)

	if len(data) == 0 {
		fmt.Println("No GPS Data in File")
		return nil, fmt.Errorf("no GPS Data in File")
	}

	return data, nil*/
}

func processGPSValues(gpsValues *fileparser.DataFileMessage, seenTimes map[int]bool, data *[]map[string]interface{}) {
	latInterface, _ := gpsValues.GetAttribute("Lat")
	lat := float64(latInterface.(int)) / ScalingFactorGPS
	lonInterface, _ := gpsValues.GetAttribute("Lng")
	lon := float64(lonInterface.(int)) / ScalingFactorGPS

	if lat != 0 && lon != 0 {
		entryDict := createEntryDict(gpsValues)
		time, ok := entryDict["TimeMS"].(int)
		if ok && !seenTimes[time] {
			seenTimes[time] = true
			*data = append(*data, entryDict)
		}
	}
}

func createEntryDict(gpsValues *fileparser.DataFileMessage) map[string]interface{} {
	entryDict := make(map[string]interface{})
	for i, field := range gpsValues.FieldNames {
		if field == "Lat" || field == "Lng" {
			entryDict[field] = float64(gpsValues.Elements[i].(int)) / ScalingFactorGPS
		} else {
			entryDict[field] = gpsValues.Elements[i]
		}
	}
	return entryDict
}

func getTimeFromGPS(gpsData map[string]interface{}) time.Time {
	// Extract the GPS week and time in milliseconds
	gpsWeek := gpsData["Week"].(int)
	gpsTimeMS := gpsData["TimeMS"].(int)

	// Calculate the total seconds since the GPS epoch (1980-01-06)
	gpsEpoch := time.Date(1980, 1, 6, 0, 0, 0, 0, time.UTC)
	totalSeconds := float64(gpsWeek*604800) + float64(gpsTimeMS)/1000.0

	// Add the total seconds to the GPS epoch to get the actual time
	gpsTime := gpsEpoch.Add(time.Duration(totalSeconds) * time.Second)

	return gpsTime
}

func createGeometry(data []map[string]interface{}) (*geom.Geometry, error) {
	var coords []float64

	for _, record := range data {
		lat, ok1 := record["Lat"].(float64)
		lng, ok2 := record["Lng"].(float64)

		if !ok1 || !ok2 {
			return nil, fmt.Errorf("missing or invalid Lat or Lng in record: %v", record)
		}

		// Append lng and lat to the coords slice
		coords = append(coords, lng, lat)
	}

	sequence := geom.NewSequence(coords, geom.DimXY)
	lineString := geom.NewLineString(sequence)
	geometry := lineString.AsGeometry()
	return &geometry, nil
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered from panic:", r)
			buf := make([]byte, 1<<16)
			runtime.Stack(buf, true)
			fmt.Printf("%s\n", buf)
		}
	}()

	filename := "../test_files/10.bin"
	file, err := os.Open(filename)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer file.Close() // Ensure the file is closed when done

	// Create an io.Reader from the file
	r := io.Reader(file)

	parser := BINParser{}

	geometry, err := parser.ParseGeometry(r)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Convert geom.Geometry to GeoJSONFeature
	feature := ConvertToGeoJSONFeature(*geometry, "example_id", map[string]interface{}{
		"name": "Example feature",
	})

	// Marshal the GeoJSONFeature to JSON
	geoJSON, err := json.Marshal(feature)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Print the GeoJSON string
	fmt.Println("GeoJSON Feature:", string(geoJSON))
}
