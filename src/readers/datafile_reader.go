package readers

import (
    "telemetry_parser/src/clocks"
    "telemetry_parser/src/messages"
)

type DFReader struct {
    clock 	     *clocks.DFReaderClock 
    timestamp    int
    mavType      interface{} // mavutil.mavlink.MAV_TYPE_FIXED_WING
    verbose      bool
    params       map[string]interface{}
    flightmodes  []interface{}
    messages     map[string]interface{}
    percent      int
    flightmode   string
    zeroTimeBase bool
	counts	     []int
	offsets	     []int
	offset 		 int
	typeNums	 []int
	indexes      []int
	dataLen	  	 int
}

func NewDFReader() *DFReader {
    return &DFReader{
        params:   make(map[string]interface{}),
        messages: map[string]interface{}{"MAV": nil, "__MAV__": nil},
    }
}

func (d *DFReader) rewind() {
    d.messages = map[string]interface{}{"MAV": nil, "__MAV__": nil}
    if d.flightmodes != nil && len(d.flightmodes) > 0 {
        d.flightmode = d.flightmodes[0].(string)
    } else {
        d.flightmode = "UNKNOWN"
    }
    d.percent = 0
    if d.clock != nil {
        d.clock.RewindEvent()
    }
}

func (d *DFReader) initClockPX4(px4MsgTime, px4MsgGPS interface{}) bool {
    d.clock = clocks.NewDFReaderClockPX4()
    if !d.zeroTimeBase {
        if clock, ok := d.clock.(*clocks.DFReaderClockPX4); ok {
            // Perform type assertion on px4MsgTime
            if startTime, ok := px4MsgTime.(struct{ StartTime int64 }); ok {
                clock.SetPX4Timebase(startTime)
            } else {
                return false
            }
            
            // Perform type assertion on px4MsgGPS
            if gpsTime, ok := px4MsgGPS.(struct{ GPSTime int64 }); ok {
                clock.FindTimeBase(gpsTime)
            } else {
                return false
            }
        }
    }
    return true
}

func (d *DFReader) initClockMsec() {
    d.clock = clocks.NewDFReaderClockMsec()
}

func (d *DFReader) initClockUsec() {
    d.clock = clocks.NewDFReaderClockUsec()
}

func (d *DFReader) initClockGPSInterpolated(clock interface{}) {
    d.clock = clocks.NewDFReaderClockGPSInterpolated()
}

func (d *DFReader) initClock() {
    d.rewind()
    gpsClock := clocks.NewDFReaderClockGPSInterpolated()
    d.clock = gpsClock

    var px4MsgTime, px4MsgGPS, gpsInterpMsgGPS1, firstUsStamp, firstMsStamp interface{}
    haveGoodClock := false

    for {
        message := d.recvMsg()
        if message == nil {
            break
        }

        msgType := message.GetType()

        if firstUsStamp == nil {
            firstUsStamp = message.getTimeUS()
        }

        if firstMsStamp == nil && (msgType != "GPS" && msgType != "GPS2") {
            firstMsStamp = message.getTimeMS()
        }

        if msgType == "GPS" || msgType == "GPS2" {
            if message.getTimeUS() != 0 && message.getGWk() != 0 {
                d.initClockUsec()
                if !d.zeroTimeBase {
                    d.clock.findTimeBase(message, firstUsStamp)
                }
                haveGoodClock = true
                break
            }
            if message.getT() != 0 && message.getWeek() != 0 {
                if firstMsStamp == nil {
                    firstMsStamp = message.getT()
                }
                d.initClockMsec()
                if !d.zeroTimeBase {
                    d.clock.findTimeBase(message, firstMsStamp)
                }
                haveGoodClock = true
                break
            }
            if message.getGPSTime() != 0 {
                px4MsgGPS = message
            }
            if message.getWeek() != 0 {
                if gpsInterpMsgGPS1 != nil && (gpsInterpMsgGPS1.getTimeMS() != message.getTimeMS() || gpsInterpMsgGPS1.getWeek() != message.getWeek()) {
                    d.initClockGPSInterpolated(gpsClock)
                    haveGoodClock = true
                    break
                }
                gpsInterpMsgGPS1 = message
            }
        } else if msgType == "TIME" {
            if message.getStartTime() != nil {
                px4MsgTime = message
            }
        }

        if px4MsgTime != nil && px4MsgGPS != nil {
            d.initClockPX4(px4MsgTime, px4MsgGPS)
            haveGoodClock = true
            break
        }
    }

    if !haveGoodClock {
        if firstUsStamp != nil {
            d.initClockUsec()
        } else if firstMsStamp != nil {
            d.initClockMsec()
        }
    }

    d.rewind()
}

func (d *DFReader) setTime(m messages.DFMessage) {
	m.Fmt.timestamp = d.timestamp
	if len(m.GetFieldNames()) > 0 && d.clock != nil {
		d.clock.setMessageTimestamp(m)
	}
} // Add closing parenthesis and semicolon here

func (d *DFReader) recvMsg() messages.DFMessage {
	return *d.parseNext()
}

func (d *DFReader) addMsg(m messages.DFMessage) {
	msgType := m.GetType()
	d.messages[msgType] = m
}