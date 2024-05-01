package clocks

import "telemetry_parser/src/messages"

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
func (clock *DFReaderClockMsec) FindTimeBase(gps *messages.GPS, firstMsStamp float64) {
    t := clock.gps_time_to_time(gps.GWk, gps.TimeUS)
    clock.SetTimebase(t - gps.T * 0.001)
    clock.timestamp = clock.timebase + firstMsStamp * 0.001
}

func (clock *DFReaderClockMsec) SetMessageTimestamp(message *messages.DFMessage) {
    if message.FieldNames[0] == "TimeMS" {
        message.TimeStamp = clock.timebase + message.TimeMS * 0.001
    } else if message.GetType() == "GPS" || message.GetType() == "GPS2" {
       // Accessing T field from GPS struct
       gps, ok := message.GetAttr("gps").(*messages.GPS)
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
