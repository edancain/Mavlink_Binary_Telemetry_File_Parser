package clocks

import "telemetry_parser/src/messages"

type DFReaderClockPX4 struct {
	*DFReaderClockBase
	px4Timebase float64
}



//doesn't get hit



func NewDFReaderClockPX4() *DFReaderClockPX4 {
	clock := &DFReaderClockPX4{
		DFReaderClockBase: NewDFReaderClockBase(),
        px4Timebase:   0,
	}
	return clock
}

func (clock *DFReaderClockPX4) FindTimeBase(gps *messages.GPS) {
	t := gps.GPSTime * 1.0e-6
    clock.timebase = t - clock.px4Timebase
}

func (clock *DFReaderClockPX4) SetPX4Timebase(timeMsg *interface{}) {
	//clock.px4Timebase = timeMsg.StartTime * 1.0e-6
}

func (clock *DFReaderClockPX4) SetMessageTimestamp(message *messages.DFMessage) {
	message.TimeStamp = clock.timebase + clock.px4Timebase
}

func (clock *DFReaderClockPX4) MessageArrived(message *messages.DFMessage) {
    messageType := message.GetType()
    if messageType == "TIME" && stringInSlice("StartTime", message.FieldNames){
        //timeMsg := &messages.DFMessage{StartTime: message.GetAttr("StartTime").(float64)}
		//clock.SetPX4Timebase(timeMsg)
    }
}

func stringInSlice(str string, slice []string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

func (clock *DFReaderClockPX4) SetTimebase(base float64) {
	clock.timebase = base
}

func (clock *DFReaderClockPX4) RewindEvent() {
	// Implement this method based on your requirements
}


