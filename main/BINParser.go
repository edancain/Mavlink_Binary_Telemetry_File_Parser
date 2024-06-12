package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"time"

	"github.com/edancain/telemetry_parser/src"
	"github.com/peterstace/simplefeatures/geom"
)

// GeoJSONFeature represents a GeoJSON feature with associated properties.
type GeoJSONFeature struct {
	Geometry       geom.Geometry            `json:"geometry"`
	ID             interface{}              `json:"id,omitempty"`
	Properties     map[string]interface{}   `json:"properties,omitempty"`
	ForeignMembers map[string]interface{}   `json:"-"`
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
	// Add any specific fields or dependencies for BIN parsing
	data []map[string]interface{}
	lineString *geom.LineString
	geometry *geom.Geometry
}

func (p *BINParser) ParseGeometry(file io.Reader) error {
		// Implement Bin parsing logic here
	// Return the parsed geometry coordinates
	
	if err := p.extractData(file); err != nil {
		return fmt.Errorf("failed to parse data: %v", err)
	}

	p.createPolylineFromData()
	geometry := p.lineString.AsGeometry()
	p.geometry = &geometry
	return nil
}

func (p *BINParser) createPolylineFromData(){
	var coords []float64

	for _, record := range p.data {
		lat, ok1 := record["Lat"].(float64)
		lng, ok2 := record["Lng"].(float64)

		if !ok1 || !ok2 {
			return 
		}

		// Append lng and lat to the coords slice
		coords = append(coords, lng, lat)
	}

	sequence := geom.NewSequence(coords, geom.DimXY)
	linestring := geom.NewLineString(sequence)
	p.lineString = &linestring
}

func getDataLen(reader io.Reader) (int, error) {
    if f, ok := reader.(*os.File); ok {
        fileInfo, err := f.Stat()
        if err != nil {
            return 0, err
        }
        return int(fileInfo.Size()), nil
    }
    // For other types of io.Reader, read into a buffer to determine length
    buf, err := io.ReadAll(reader)
    if err != nil {
        return 0, err
    }
    return len(buf), nil
}

func (p *BINParser)extractData(file io.Reader) error {
	var data []map[string]interface{}
	// Get the length of the data
	dataLen, err := getDataLen(file)
    if err != nil {
		return fmt.Errorf("failed to get data length: %v", err)
	}

	var zeroTimeBase = false

	dfreader, err := src.NewBinaryDataFileReader(file, dataLen, zeroTimeBase, nil )
	if err != nil {
		return fmt.Errorf("failed to create binary data file reader: %v", err)
	}

	if _, ok := dfreader.Messages["GPS"]; !ok {
		return fmt.Errorf("no GPS data found in file")
	} 

	dfreader.ParseNext()
	msg := dfreader.Messages
	count := 0
	messageCount := 0

	// Create a set to store seen times. This is used to test whether the time value has been
	// seen before. If it hasn't it is added and the GPS data is added to data map. Otherwise
	// we will have multiple copies of the same position. This is due to the write rate for
	// other sensors within the Pixhawk. i.e. IMU.
	seenTimes := make(map[int]bool)

	// Iterate over all messages
	for msg != nil {
		messageCount++
		if gpsValues, ok := msg["GPS"]; ok {

			lat := float64(gpsValues.GetAttr("Lat").(int)) / 1e7
			lon := float64(gpsValues.GetAttr("Lng").(int)) / 1e7

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
		return fmt.Errorf("No GPS Data in File")
	}

	p.data = data
	return nil
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

func createPolylineFromData(data []map[string]interface{}) (geom.LineString, error) {
	var coords []float64

	for _, record := range data {
		lat, ok1 := record["Lat"].(float64)
		lng, ok2 := record["Lng"].(float64)

		if !ok1 || !ok2 {
			return geom.LineString{}, fmt.Errorf("missing or invalid Lat or Lng in record: %v", record)
		}

		// Append lng and lat to the coords slice
		coords = append(coords, lng, lat)
	}

	sequence := geom.NewSequence(coords, geom.DimXY)
	lineString := geom.NewLineString(sequence)

	return lineString, nil
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

	filename := "10.bin"
	file, err := os.Open(filename)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer file.Close() // Ensure the file is closed when done

	// Create an io.Reader from the file
	reader := io.Reader(file)

	parser := BINParser{}

	err = parser.ParseGeometry(reader)
	if err != nil {
		fmt.Println(err)
		return
	}

	// how to get the datetime out of the data
	firstElement := parser.data[0]
	gpsTime := getTimeFromGPS(firstElement)
	fmt.Println("GPS Time:", gpsTime)

	// Use the extracted data
	//fmt.Println(data)
	polyline, err := createPolylineFromData(parser.data)
	if err != nil {
		fmt.Println("Error creating polyline:", err)
		return
	}

	//fmt.Println("Polyline WKT:", polyline.AsText())
	geometry := polyline.AsGeometry()
	// Convert geom.Geometry to GeoJSONFeature
	feature := ConvertToGeoJSONFeature(geometry, "example_id", map[string]interface{}{
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
