package clocks

import "github.com/edancain/telemetry_parser/src/messages"


type DFReaderClock interface {
	SetTimebase(base float64)
	MessageArrived(message *messages.DFMessage)
	RewindEvent()
	FindTimeBase(gps *messages.GPS, firstMsStamp int64)
}

type DFReaderClockBase struct {
	timebase  float64
	timestamp float64
}

func NewDFReaderClockBase() *DFReaderClockBase {
	return &DFReaderClockBase{
		timebase:  0,
		timestamp: 0,
	}
}

func (clock *DFReaderClockBase) gps_time_to_time(week, msec int) float64 {
	epoch := 86400 * (10*365 + int((1980-1969)/4) + 1 + 6 - 2)
	return float64(epoch + 86400*7*week + int(float64(msec)*1e-3) - 18)
}

func (clock *DFReaderClockBase) SetTimebase(base float64) {
	clock.timebase = base
}

func (clock *DFReaderClockBase) MessageArrived(message *messages.DFMessage) {
	// Implement this method based on your requirements
}

func (clock *DFReaderClockBase) RewindEvent() {
	// Implement this method based on your requirements
}

func (clock *DFReaderClockBase) FindTimeBase(gps *messages.GPS, firstMsStamp int64) {
	// Implement this method based on your requirements
}
