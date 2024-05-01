package clocks

import "telemetry_parser/src/messages"

type DFReaderClockMsec struct {
    *DFReaderClock
}

func NewDFReaderClockMsec() *DFReaderClockMsec {
    clock := &DFReaderClockMsec{
        DFReaderClock: NewDFReaderClock(),
    }
    return clock
}

// FindTimeBase calculates the time basis for the log in the new style
func (clock *DFReaderClockMsec) FindTimeBase(gps struct{ GWk, GMS, TimeUS int64 }, firstMsStamp float64) {
    t := clock._gpsTimeToTime(gps.GWk, gps.TimeUS)
    clock.SetTimebase(int64(t - int64(float64(gps.GMS) * 0.001)))
    clock.timestamp = int64(float64(clock.timebase) + firstMsStamp * 0.001)
}

func (clock *DFReaderClockMsec) SetMessageTimestamp(message *messages.DFMessage) {
    if message.FieldNames[0] == "TimeMS" {
        message.TimeStamp = int64(float64(clock.timebase) + float64(message.TimeMS) * 0.001)
    } else if message.GetType() == "GPS" || message.GetType() == "GPS2" {
        message.TimeStamp = int64(float64(clock.timebase) + float64(message.TimeMS) * 0.001)
    } else {
        message.TimeStamp = clock.timestamp
    }
    clock.timestamp = message.TimeStamp
}
