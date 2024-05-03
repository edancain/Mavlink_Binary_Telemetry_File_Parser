package clocks

import (
	"telemetry_parser/src/messages"
)

type DFReaderClockUsec struct {
    *DFReaderClockBase
}


//doesn't get hit

func NewDFReaderClock_usec() *DFReaderClockUsec {
	clock := &DFReaderClockUsec{
        DFReaderClockBase: NewDFReaderClockBase(),
    }
	clock.timestamp = 0
	return clock
}

func (clock *DFReaderClockUsec) FindTimeBase(gps *messages.GPS){//, firstUsStamp float64) {
    t := clock.gps_time_to_time(gps.GWk, gps.GMS)
    clock.SetTimebase(t - float64(gps.TimeUS)*0.000001)
    //clock.timestamp = clock.timebase + firstUsStamp*0.000001
}


func (clock *DFReaderClockUsec) SetMessageTimestamp(message *messages.DFMessage) {
    if message.FieldNames[0] == "TimeMS" {
        message.TimeStamp = float64(float64(clock.timebase) + float64(message.TimeMS) * 0.001)
    } else if message.GetType() == "GPS" || message.GetType() == "GPS2" {
        message.TimeStamp = float64(float64(clock.timebase) + float64(message.TimeMS) * 0.001)
    } else {
        message.TimeStamp = clock.timestamp
    }
    clock.timestamp = message.TimeStamp
}


func (clock *DFReaderClockUsec) SetTimebase(base float64) {
	clock.timebase = base
}

func (clock *DFReaderClockUsec) MessageArrived(message *messages.DFMessage) {
	// Implement this method based on your requirements
}

func (clock *DFReaderClockUsec) RewindEvent() {
	// Implement this method based on your requirements
}

