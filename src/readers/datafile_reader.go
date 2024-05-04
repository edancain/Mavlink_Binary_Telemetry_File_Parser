package readers

import (
	"telemetry_parser/src/clocks"
	"telemetry_parser/src/messages"
)

type MavType int

const (
	MavTypeGeneric                 MavType = iota // 0
	MavTypeFixedWing                              // 1
	MavTypeQuadrotor                              // 2
	MavTypeCoaxial                                // 3
	MavTypeHelicopter                             // 4
	MavTypeAntennaTracker                         // 5
	MavTypeGCS                                    // 6
	MavTypeAirship                                // 7
	MavTypeFreeBalloon                            // 8
	MavTypeRocket                                 // 9
	MavTypeGroundRover                            // 10
	MavTypeSurfaceBoat                            // 11
	MavTypeSubmarine                              // 12
	MavTypeHexarotor                              // 13
	MavTypeOctorotor                              // 14
	MavTypeTricopter                              // 15
	MavTypeFlappingWing                           // 16
	MavTypeKite                                   // 17
	MavTypeOnboardController                      // 18
	MavTypeVtolTailsitterDuorotor                 // 19
	MavTypeVtolTailsitterQuadrotor                // 20
	MavTypeVtolTiltrotor                          // 21
	MavTypeVtolFixedrotor                         // 22
	MavTypeVtolTailsitter                         // 23
	MavTypeVtolTiltwing                           // 24
	MavTypeVtolReserved5                          // 25
	MavTypeGimbal                                 // 26
	MavTypeADSB                                   // 27
	MavTypeParafoil                               // 28
	MavTypeDodecarotor                            // 29
	MavTypeCamera                                 // 30
	MavTypeChargingStation                        // 31
	MavTypeFlarm                                  // 32
	MavTypeServo                                  // 33
	MavTypeODID                                   // 34
	MavTypeDecarotor                              // 35
	MavTypeBattery                                // 36
	MavTypeParachute                              // 37
	MavTypeLog                                    // 38
	MavTypeOSD                                    // 39
	MavTypeIMU                                    // 40
	MavTypeGPS                                    // 41
	MavTypeWinch                                  // 42
)

type DFReader struct {
	clock        clocks.DFReaderClock
	timestamp    int64
	mavType      MavType
	verbose      bool
	params       map[string]interface{}
	flightmodes  []interface{}
	messages     map[string]*messages.DFMessage
	percent      float64
	flightmode   string
	zeroTimeBase bool
	counts       []int
	offsets      []int
	offset       int
	typeNums     []int
	indexes      []int
	dataLen      int64
	
}

func NewDFReader() *DFReader {
	return &DFReader{
		clock:       nil,
		timestamp:   0,
		mavType:     MavTypeGeneric,
		verbose:     false,
		params:      make(map[string]interface{}),
		flightmodes: nil,
		messages: map[string]*messages.DFMessage{
			"MAV":     nil,
			"__MAV__": nil,
		},
		percent: 0.0,
	}
}

func (d *DFReader) rewind() {
	d.messages = map[string]*messages.DFMessage{
		"MAV":     nil,
		"__MAV__": nil,
	}

	if d.flightmodes != nil && len(d.flightmodes) > 0 {
		d.flightmodes = []interface{}{d.flightmodes[0]}
	} else {
		d.flightmodes = []interface{}{"UNKNOWN"}
	}
	d.percent = 0.0
	if d.clock != nil {
		d.clock.RewindEvent()
	}
}

func (d *DFReader) initClockMsec() {
	clock := clocks.NewDFReaderClockMsec()
	d.clock = clock
}

func (d *DFReader) initClock() {
	d.rewind()

	//speculatively create a gps clock in case we don't find anything better
	//gpsClock := clocks.NewDFReaderClockGPSInterpolated()
	//d.clock = gpsClock
	d.initClockMsec()

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

					//d.initClockUsec()

					if !d.zeroTimeBase {
						//d.clock.FindTimeBase(gps, firstUsStamp)
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
					d.clock.FindTimeBase(gps, firstMsStamp.(int64))
				}
				haveGoodClock = true
				break
			}
			if message.GetAttr("GPSTime") != 0 {
				px4MsgGPS = message
			}
			if message.GetAttr("Week") != 0 { //could be gps.Week
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
			//d.initClockPX4(px4MsgTime, px4MsgGPS)
			//haveGoodClock = true
			//break
		}
	}

	if !haveGoodClock {
		if firstUsStamp != 0 {
			//d.initClockUsec()
		} else if firstMsStamp != nil {
			d.initClockMsec()
		}
	}

	d.rewind()
}

func (d *DFReader) setTime(m *messages.DFMessage) {
	m.TimeStamp = float64(d.timestamp)
	if len(m.GetFieldNames()) > 0 && d.clock != nil {
		//d.clock.SetMessageTimestamp(m)
	}
} // Add closing parenthesis and semicolon here

func (d *DFReader) recvMsg() messages.DFMessage {
	//return *d.ParseNext()
}

func (d *DFReader) addMsg(m *messages.DFMessage) {
	msgType := m.GetType()
	d.messages[msgType] = m
}


