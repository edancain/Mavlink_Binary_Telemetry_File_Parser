package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/edancain/telemetry_parser/src"
)

type GPSValues struct {
    Lat      float64
    TimeMS   int64
    TimeUS   int64
    // Add other fields as per your requirement
}


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
        dfreader, err := src.NewDFReaderBinary(filename, false, nil)
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
        

        // Get the values of the attributes
        if gpsValues, ok := dfreader.Messages["GPS"]; ok {
            fieldnames := gpsValues.FieldNames
            fmt.Println(fieldnames)
        }
        
        // Create a set to store seen times
        seenTimes := make(map[int]bool)

        // Iterate over all messages
        for msg != nil {
            messageCount++
            if gpsValues, ok := msg["GPS"]; ok {
                
                lat := float64(gpsValues.GetAttr("Lat").(int)) / 1e7
                lon := float64(gpsValues.GetAttr("Lng").(int)) / 1e7
                fmt.Println(lat, lon)
                
                if lat != 0 {
                    // Get the values of the fields
                    //values := []interface{}{gpsValues.Lat, gpsValues.TimeMS, gpsValues.TimeUS} // Update this as per your requirement

                    // Create a dictionary from fieldnames and values
                    entryDict := make(map[string]interface{})
                    for i, field := range gpsValues.FieldNames {
                        if field == "Lat" || field == "Lng"{
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

    // Use the extracted data
    fmt.Println(data)
}

