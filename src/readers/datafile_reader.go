package readers

import (
	"telemetry_parser/src/clocks"
	"telemetry_parser/src/messages"
)

type DFReader struct {
	clock        clocks.DFReaderClock
	timestamp    int64
	mavType      interface{} // mavutil.mavlink.MAV_TYPE_FIXED_WING
	verbose      bool
	params       map[string]interface{}
	flightmodes  []interface{}
	messages     map[string]interface{} //map[string]*messages.DFMessage
	percent      int
	flightmode   string
	zeroTimeBase bool
	counts       []int
	offsets      []int
	offset       int
	typeNums     []int
	indexes      []int
	dataLen      int
}

func NewDFReader() *DFReader {
	return &DFReader{
		clock:       nil,
		timestamp:   0,
		mavType:     nil,
		verbose:     false,
		params:      make(map[string]interface{}),
		flightmodes: nil,
		messages: map[string]interface{}{
			"MAV":     nil,
			"__MAV__": nil,
		},
		percent: 0,
	}
}

func (d *DFReader) rewind() {
	d.messages = map[string]interface{}{
		"MAV":     nil,
		"__MAV__": nil,
	}
	if d.flightmodes != nil && len(d.flightmodes) > 0 {
		d.flightmodes = []interface{}{d.flightmodes[0]}
	} else {
		d.flightmodes = []interface{}{"UNKNOWN"}
	}
	d.percent = 0
	if d.clock != nil {
		d.clock.RewindEvent()
	}
}

func (d *DFReader) initClockPX4(px4MsgTime, px4MsgGPS interface{}) bool {
	//doesn't get hit
	d.clock = clocks.NewDFReaderClockPX4()
	if !d.zeroTimeBase {
		if clock, ok := d.clock.(*clocks.DFReaderClockPX4); ok {
			// Perform type assertion on px4MsgTime
			if startTime, ok := px4MsgTime.(*messages.TimeMsg); ok {
				clock.SetPX4Timebase(startTime)
			} else {
				return false
			}

			// Perform type assertion on px4MsgGPS
			if gpsTime, ok := px4MsgGPS.(*messages.GPS); ok {
				clock.FindTimeBase(gpsTime)
			} else {
				return false
			}
		}
	}
	return true
}

func (d *DFReader) initClockUsec() {
	//doesn't get hit
    clock := clocks.NewDFReaderClock_usec()
    d.clock = clock
}

func (d *DFReader) initClockMsec() {
    clock := clocks.NewDFReaderClockMsec()
    d.clock = clock
}

func (d *DFReader) initClockGPSInterpolated(gpsClock *clocks.DFReaderClockGPSInterpolated) {	
	//doesn't get hit
	clock := clocks.NewDFReaderClockGPSInterpolated()
	d.clock = clock
}

func (d *DFReader) initClock() {
	d.rewind()

	//speculatively create a gps clock in case we don't find anything better
	gpsClock := clocks.NewDFReaderClockGPSInterpolated()
	d.clock = gpsClock

	var px4MsgTime, px4MsgGPS, firstMsStamp interface{}
	var firstUsStamp float64
	//var gpsInterpMsgGPS1 messages.DFMessage
	haveGoodClock := false

	for {
		message := d.recvMsg()
		if &message == nil {
			break
		}

		msgType := message.GetType()

		if firstUsStamp == 0 {
			firstUsStamp, _ = message.GetAttr("TimeMS").(float64)
		}

		if firstMsStamp == nil && (msgType != "GPS" && msgType != "GPS2") {
			firstMsStamp = message.GetAttr("TimeMS")
		}

		if msgType == "GPS" || msgType == "GPS2" {
            gps, ok := message.GetAttr("GPS").(*messages.GPS)
            if ok {
                if gps.TimeUS != 0 && gps.GWk != 0 {

                    d.initClockUsec()

                    if !d.zeroTimeBase {
                        d.clock.FindTimeBase(gps, firstUsStamp)
                    }

                    haveGoodClock = true
                    break
                }
            }

			if gps.T != 0 && gps.Week != 0 {
				if firstMsStamp == nil {
					firstMsStamp = gps.T
				}

				d.initClockMsec()

				if !d.zeroTimeBase {
					d.clock.FindTimeBase(gps, firstMsStamp.(float64)) 
				}
				haveGoodClock = true
				break
			}
			if message.GetAttr("GPSTime") != 0 {
				px4MsgGPS = message
			}
			if message.GetAttr("Week") != 0 {  //could be gps.Week
				/*if gpsInterpMsgGPS1 != nil && (gpsInterpMsgGPS1.TimeMS != message.TimeMS || gpsInterpMsgGPS1.getWeek() != message.getWeek()) {
					d.initClockGPSInterpolated(gpsClock)
					haveGoodClock = true
					break
				}
				gpsInterpMsgGPS1 = message*/
			}
		} else if msgType == "TIME" {
			//only px4-style logs use TIME
			if message.GetAttr("StartTime") != nil {
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
		if firstUsStamp != 0 {
			d.initClockUsec()
		} else if firstMsStamp != nil {
			d.initClockMsec()
		}
	}

	d.rewind()
}

func (d *DFReader) setTime(m *messages.DFMessage) {
	m.TimeStamp = float64(d.timestamp)
	if len(m.GetFieldNames()) > 0 && d.clock != nil {
		d.clock.SetMessageTimestamp(m)
	}
} // Add closing parenthesis and semicolon here

func (d *DFReader) recvMsg() messages.DFMessage {
	return *d.parseNext()
}

func (d *DFReader) addMsg(m *messages.DFMessage) {
	msgType := m.GetType()
	d.messages[msgType] = m
}
