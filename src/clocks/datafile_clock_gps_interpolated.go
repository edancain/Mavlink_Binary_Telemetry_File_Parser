package clocks

import "telemetry_parser/src/messages"

type DFReaderClockGPSInterpolated struct {
    *DFReaderClock
    MsgRate         map[string]float64
    Counts          map[string]int
    CountsSinceGPS  map[string]int
}

func NewDFReaderClockGPSInterpolated() *DFReaderClockGPSInterpolated {
    clock := &DFReaderClockGPSInterpolated{
		DFReaderClock:  NewDFReaderClock(),
		MsgRate:        make(map[string]float64),
		Counts:         make(map[string]int),
		CountsSinceGPS: make(map[string]int),
	}
	return clock
}

func (clock *DFReaderClockGPSInterpolated) RewindEvent() {
	clock.Counts = make(map[string]int)
	clock.CountsSinceGPS = make(map[string]int)
}

// MessageArrived handles the arrival of a message
func (clock *DFReaderClockGPSInterpolated) MessageArrived(m *messages.DFMessage) {
	msgType := m.GetType()
	if _, ok := clock.Counts[msgType]; !ok {
		clock.Counts[msgType] = 1
	} else {
		clock.Counts[msgType]++
	}

	if _, ok := clock.CountsSinceGPS[msgType]; !ok {
		clock.CountsSinceGPS[msgType] = 1
	} else {
		clock.CountsSinceGPS[msgType]++
	}

	if msgType == "GPS" || msgType == "GPS2" {
		clock.GPSMessageArrived(m)
	}
}

// GPSMessageArrived adjusts the time base from GPS message

func (clock *DFReaderClockGPSInterpolated) GPSMessageArrived(message *messages.DFMessage) {
    var gpsWeek, gpsTimeMs int64

    // Loop through each element in the slice
    for _, element := range message.Elements {
        // Check if the element is a map[string]interface{}
        if elementMap, ok := element.(map[string]interface{}); ok {
            // Try to get the "Week" value
            if val, ok := elementMap["Week"].(int64); ok {
                gpsWeek = val
                break // Break out of the loop if value is found
            } else if val, ok := elementMap["GWk"].(int64); ok {
                gpsWeek = val
                break // Break out of the loop if value is found
            } else if val, ok := elementMap["Wk"].(int64); ok {
                gpsWeek = val
                break // Break out of the loop if value is found
            }
        }
    }

    // Similarly, try to get the "TimeMS" value
    for _, element := range message.Elements {
        if val, ok := element.(int64); ok {
            gpsTimeMs = val
            break // Break out of the loop if value is found
        }
    }

    t := clock._gpsTimeToTime(gpsWeek, gpsTimeMs)

    deltat := t - clock.timebase
    if deltat <= 0 {
        return
    }

    for msgType, count := range clock.CountsSinceGPS {
        rate := float64(count) / float64(deltat)
        if rate > clock.MsgRate[msgType] {
            clock.MsgRate[msgType] = rate
        }
    }
    clock.MsgRate["IMU"] = 50.0
    clock.timebase = t
    clock.CountsSinceGPS = make(map[string]int)
}
