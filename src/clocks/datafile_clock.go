package clocks

import "https://github.com/edancain/telemetry_parser/blob/main/src/messages"

type DFReaderClock struct {
	timebase  float64
	timestamp float64
}

func NewDFReaderClock() *DFReaderClock {
	return &DFReaderClock{
		timebase:  0,
		timestamp: 0,
	}
}

func (clock *DFReaderClock) GpsTimeToTime(week, msec int) float64 {
	epoch := 86400 * (10*365 + int((1980-1969)/4) + 1 + 6 - 2)
	return float64(epoch + 86400*7*week + int(float64(msec)*1e-3) - 18)
}

func (clock *DFReaderClock) SetTimebase(base float64) {
	clock.timebase = base
}

func (clock *DFReaderClock) MessageArrived(message *messages.DFMessage) {
	// Implement this method based on your requirements
}

func (clock *DFReaderClock) RewindEvent() {
	// Implement this method based on your requirements
}
