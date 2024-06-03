package src

import (
	"fmt"
)


type DFReaderClockGPSInterpolated struct {
    MsgRate         map[string]float64
    Counts          map[string]int
    CountsSinceGPS  map[string]int
	Timebase  float64
	Timestamp float64
}

func NewDFReaderClockGPSInterpolated() *DFReaderClockGPSInterpolated {
    clock := &DFReaderClockGPSInterpolated{
		MsgRate:        make(map[string]float64),
		Counts:         make(map[string]int),
		CountsSinceGPS: make(map[string]int),
		Timebase:  0,
		Timestamp: 0,
	}
	return clock
}

//doesn't get hit
func (clock *DFReaderClockGPSInterpolated) RewindEvent() {
	clock.Counts = make(map[string]int)
	clock.CountsSinceGPS = make(map[string]int)
}

func (clock *DFReaderClockGPSInterpolated) FindTimeBase(message *DFMessage, firstUsStamp int) {
	week, ok := message.GetAttr("Week").(int)
	if !ok {
		return
	}

	timeMs, ok := message.GetAttr("TimeMS").(int)
	if !ok {
		return
	}

	t := clock.GPSTimeToTime(week, timeMs)
	
	var v = t - float64(firstUsStamp)*0.001
	clock.SetTimebase(v)
	clock.Timestamp = clock.Timebase + float64(firstUsStamp)*0.001
	fmt.Println(clock.Timestamp)
}

func (clock *DFReaderClockGPSInterpolated) GPSTimeToTime(week int, msec int) float64 {
	// convert GPS week and TOW to a time in seconds since 1970
	epoch := 86400 * (10*365 + int((1980-1969)/4) + 1 + 6 - 2)
	return float64(epoch) + float64(86400*7*int(week)) + float64(msec)*0.001 - 18
}

//doesn't get hit
func (clock *DFReaderClockGPSInterpolated) MessageArrived(message *DFMessage) {
	msgType := message.GetType()
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
		clock.GPSMessageArrived(message)
	}
}

//doesn't get hit
func (clock *DFReaderClockGPSInterpolated) GPSMessageArrived(message *DFMessage) {
    var gpsWeek, gpsTimems interface{}

    // msec-style GPS message?
    gpsWeek = message.GetAttr("Week")
    gpsTimems = message.GetAttr("TimeMS")

    gpsWeek = message.GetAttr("Week")
    gpsTimems = message.GetAttr("TimeMS")
    if gpsWeek == nil {
        // usec-style GPS message?
        gpsWeek = message.GetAttr("GWk")
        gpsTimems = message.GetAttr("GMS")
        if gpsWeek == nil {
            if message.GetAttr("GPSTime") != nil {
                // PX4-style timestamp; we've only been called
                // because we were speculatively created in case no
                // better clock was found.
                return
            }
            gpsWeek = message.GetAttr("Wk")
            if gpsWeek != nil {
                // AvA-style logs
                gpsTimems = message.GetAttr("TWk")
            }
        }
    }

    if gpsWeek == nil || gpsTimems == nil {
        return
    }

    t := clock.GPSTimeToTime(gpsWeek.(int), gpsTimems.(int))
    deltat := t - clock.Timebase
    if deltat <= 0 {
        return
    }

    for msgType := range clock.CountsSinceGPS {
        rate := float64(clock.CountsSinceGPS[msgType]) / deltat
        if rate > clock.MsgRate[msgType] {
            clock.MsgRate[msgType] = rate
        }
    }

    clock.MsgRate["IMU"] = 50.0
    clock.Timebase = t
    clock.CountsSinceGPS = make(map[string]int)
}


func (clock *DFReaderClockGPSInterpolated) SetMessageTimestamp(message *DFMessage) {
    rate := clock.MsgRate[message.GetType()]
    if rate == 0 {
        rate = 50
    }
    count := clock.CountsSinceGPS[message.GetType()]
    message.SetAttr("_timestamp", clock.Timebase+float64(count)/rate)
}

func (clock *DFReaderClockGPSInterpolated) SetTimebase(base float64) {
	clock.Timebase = base
}

