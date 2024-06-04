package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/edancain/telemetry_parser/src"
)

func extractData(filename string) ([]map[string]interface{}, error) {
	_, err := os.Stat(filename)
	if os.IsNotExist(err) {
		fmt.Printf("File %s does not exist.\n", filename)
		return nil, err
	}

	var data []map[string]interface{}

	if strings.HasSuffix(filename, ".log") {
		// dfreader = DFReader_text(filename)
		fmt.Println("log file")
	} else {
		dfreader, err := src.NewBinaryDataFileReader(filename, false, nil)
		if err != nil {
			fmt.Println("Failed to create DFReaderBinary:", err)
			fmt.Println(dfreader) //bogus
			return nil, err
		}

		dfreader.Print_binaryFormats()

		if _, ok := dfreader.Messages["GPS"]; !ok {
			fmt.Println("No GPS data")
			return nil, fmt.Errorf("no GPS data")
		} else {
			fmt.Println("GPS data found")
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
				fmt.Println(lat, lon)

				if lat != 0 {
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
			return nil, fmt.Errorf("No GPS Data in File")
		}

	}
	return data, nil
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
	data, err := extractData(filename)
	if err != nil {
		fmt.Println(err)
		return
	}

	// how to get the datetime out of the data
	firstElement := data[0]
	gpsTime := getTimeFromGPS(firstElement)
	fmt.Println("GPS Time:", gpsTime)

	// Use the extracted data
	fmt.Println(data)
}
