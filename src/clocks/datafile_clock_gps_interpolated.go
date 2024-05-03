package clocks

import (
	"telemetry_parser/src/messages"
)

type DFReaderClockGPSInterpolated struct {
    *DFReaderClockBase
    MsgRate         map[string]float64
    Counts          map[string]float64
    CountsSinceGPS  map[string]float64
}

func NewDFReaderClockGPSInterpolated() *DFReaderClockGPSInterpolated {
    clock := &DFReaderClockGPSInterpolated{
		DFReaderClockBase:  NewDFReaderClockBase(),
		MsgRate:        make(map[string]float64),
		Counts:         make(map[string]float64),
		CountsSinceGPS: make(map[string]float64),
	}
	return clock
}

//doesn't get hit
func (clock *DFReaderClockGPSInterpolated) RewindEvent() {
	clock.Counts = make(map[string]float64)
	clock.CountsSinceGPS = make(map[string]float64)
}

func (clock *DFReaderClockGPSInterpolated) FindTimeBase(gps *messages.GPS){//, firstUsStamp float64) {
	t := clock.gps_time_to_time(gps.GWk, gps.TimeUS)
	clock.SetTimebase(t - gps.T * 0.001)
}

//doesn't get hit
func (clock *DFReaderClockGPSInterpolated) MessageArrived(message *messages.DFMessage) {
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
func (clock *DFReaderClockGPSInterpolated) GPSMessageArrived(message *messages.DFMessage) {
    gpsWeek := message.GetAttr("Week").(int)
	gpsTimeMs := message.GetAttr("TimeMS").(int)

    if gpsWeek == 0 || gpsTimeMs == 0 {
		return
	}

    t := clock.gps_time_to_time(gpsWeek, gpsTimeMs)

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
    clock.CountsSinceGPS = make(map[string]float64)
}

//doesn't get hit
func (clock *DFReaderClockGPSInterpolated) SetMessageTimestamp(message *messages.DFMessage) {
    rate := clock.MsgRate[message.Fmt.Name]
    if int(rate) == 0 {
        rate = 50
    }
    count := clock.CountsSinceGPS[message.Fmt.Name]
    message.TimeStamp = clock.timebase + count/rate
}

func (clock *DFReaderClockGPSInterpolated) SetTimebase(base float64) {
	clock.timebase = base
}
