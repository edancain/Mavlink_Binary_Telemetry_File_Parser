package main

import (
    "fmt"
    "os"
    "strings"
)

type GPSValues struct {
    Lat      float64
    TimeMS   int64
    TimeUS   int64
    // Add other fields as per your requirement
}

type DFReader struct {
    messages map[string]GPSValues
    percent  float64
}

func (d *DFReader) parseNext() {
    // Implement this method based on your requirement
}

func extractData(filename string) ([]map[string]interface{}, error) {
    _, err := os.Stat(filename)
    if os.IsNotExist(err) {
        fmt.Printf("File %s does not exist.\n", filename)
        return nil, err
    }

    var dfreader DFReader
    if strings.HasSuffix(filename, ".log") {
        // dfreader = DFReader_text(filename)
	} else {
        // dfreader = DFReader_binary(filename)
    }

    if _, ok := dfreader.messages["GPS"]; !ok {
        fmt.Println("no GPS data")
        return nil, fmt.Errorf("no GPS data")
    }

    dfreader.parseNext()
    msg := dfreader.messages
    count := 0
    messageCount := 0
    var data []map[string]interface{}

    // Get the values of the attributes
    fieldnames := []string{"Lat", "TimeMS", "TimeUS"} // Update this as per your requirement

    // Create a set to store seen times
    seenTimes := make(map[int64]bool)

    // Iterate over all messages
    for msg != nil {
        messageCount++
        if gpsValues, ok := msg["GPS"]; ok {
            if gpsValues.Lat != 0 {
                // Get the values of the fields
                values := []interface{}{gpsValues.Lat, gpsValues.TimeMS, gpsValues.TimeUS} // Update this as per your requirement

                // Create a dictionary from fieldnames and values
                entryDict := make(map[string]interface{})
                for i, field := range fieldnames {
                    entryDict[field] = values[i]
                }

                // Check if this time has been seen before
                time := entryDict["TimeMS"].(int64)
                if !seenTimes[time] {
                    // Add this time to the set of seen times
                    seenTimes[time] = true
                    data = append(data, entryDict)
                    count++
                }
            }
        }

        // Get the next message
        dfreader.parseNext()
        fmt.Printf("%.1f%%\n", dfreader.percent)
        fmt.Printf("%d unique records\n", count)
        if dfreader.percent > 99.99 {
            break
        }

        msg = dfreader.messages
    }

    fmt.Printf("Total messages: %d\n", messageCount)

    if len(data) == 0 {
        fmt.Println("No GPS Data in File")
        return nil, fmt.Errorf("No GPS Data in File")
    }

    return data, nil
}

func main() {
    filename := "10.bin"
    data, err := extractData(filename)
    if err != nil {
        fmt.Println(err)
        return
    }

    // Use the extracted data
    fmt.Println(data)
}