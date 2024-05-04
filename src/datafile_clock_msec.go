package src

//DFReaderClock_msec - a format where many messages have TimeMS in
//their formats, and GPS messages have a "T" field giving msecs
type DFReaderClockMsec struct {
    *DFReaderClockBase
}

func NewDFReaderClockMsec() *DFReaderClockMsec {
    clock := &DFReaderClockMsec{
        DFReaderClockBase: NewDFReaderClockBase(),
    }
    return clock
}

// FindTimeBase calculates the time basis for the log 
func (clock *DFReaderClockMsec) FindTimeBase(gps *GPS, firstMsStamp int64) {
    t := clock.gps_time_to_time(gps.GWk, gps.TimeUS)
    clock.SetTimebase(t - gps.T * 0.001)
    clock.timestamp = clock.timebase + float64(firstMsStamp) * 0.001
}

func (clock *DFReaderClockMsec) SetMessageTimestamp(message *DFMessage) {
    //doesn't get hit
    if message.FieldNames[0] == "TimeMS" {
        message.TimeStamp = clock.timebase + message.TimeMS * 0.001
    } else if message.GetType() == "GPS" || message.GetType() == "GPS2" {
       // Accessing T field from GPS struct
       gps, ok := message.GetAttr("gps").(*GPS)
       if !ok {
           // Handle error
           return
       }
       message.TimeStamp = clock.timebase + gps.T * 0.001  
   } else {
       message.TimeStamp = clock.timestamp
   }
   clock.timestamp = message.TimeStamp
}

func (clock *DFReaderClockMsec) SetTimebase(base float64) {
	clock.timebase = base
}

func (clock *DFReaderClockMsec) MessageArrived(message *DFMessage) {
	// Implement this method based on your requirements
}

func (clock *DFReaderClockMsec) RewindEvent() {
	// Implement this method based on your requirements
}

