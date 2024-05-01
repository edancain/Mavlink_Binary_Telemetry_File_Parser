package clocks

import (
	"telemetry_parser/src/messages"
)

type DFReaderClockUsec struct {
    *DFReaderClock
}

func NewDFReaderClockUsec() *DFReaderClockUsec {
    clock := &DFReaderClockUsec{
		DFReaderClock: NewDFReaderClock(),
	}
	return clock

}

func (clock *DFReaderClockUsec) FindTimeBase(gps struct{ GWk, GMS, TimeUS int64 }, firstUsStamp int64) {
    t := clock._gpsTimeToTime(gps.GWk, gps.GMS)
    clock.SetTimebase(t - int64(float64(gps.TimeUS) * 0.000001))
    clock.timestamp = clock.timestamp + int64(float64(firstUsStamp) * 0.000001)
}


func (clock *DFReaderClockUsec) SetMessageTimestamp(message *messages.DFMessage) {
    if message.FieldNames[0] == "TimeMS" {
        message.TimeStamp = int64(float64(clock.timebase) + float64(message.TimeMS) * 0.001)
    } else if message.GetType() == "GPS" || message.GetType() == "GPS2" {
        message.TimeStamp = int64(float64(clock.timebase) + float64(message.TimeMS) * 0.001)
    } else {
        message.TimeStamp = clock.timestamp
    }
    clock.timestamp = message.TimeStamp
}