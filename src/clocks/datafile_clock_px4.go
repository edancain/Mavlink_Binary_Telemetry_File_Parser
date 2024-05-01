package clocks

import "telemetry_parser/src/messages"

type DFReaderClockPX4 struct {
	*DFReaderClock
	px4Timebase int64
}

func NewDFReaderClockPX4() *DFReaderClockPX4 {
	clock := &DFReaderClockPX4{
		DFReaderClock: NewDFReaderClock(),
        px4Timebase:   0,
	}
	return clock
}

func (clock *DFReaderClockPX4) FindTimeBase(gps struct{ GPSTime int64 }) {
	t := float64(gps.GPSTime) * 1.0e-6
    clock.timebase = int64(t - float64(clock.px4Timebase))
}

func (clock *DFReaderClockPX4) SetPX4Timebase(timeMsg struct{ StartTime int64 }) {
	clock.px4Timebase = timeMsg.StartTime
}

func (clock *DFReaderClockPX4) SetMessageTimestamp(message *messages.DFMessage) {
	message.TimeStamp = clock.timebase + clock.px4Timebase
}

func (clock *DFReaderClockPX4) MessageArrived(message *messages.DFMessage) {
    messageType := message.GetType()
    if messageType == "TIME" && len(message.FieldNames) > 0 {
        for _, fieldName := range message.FieldNames {
            if fieldName == "StartTime" {
                clock.SetPX4Timebase(message.Elements[0].(struct{ StartTime int64 }))
                break
            }
        }
    }
}
