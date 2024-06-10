package src

import (
	"fmt"
)

type GPSInterpolated struct {
	MsgRate        map[string]float64
	Counts         map[string]int
	CountsSinceGPS map[string]int
	Timebase       float64
	Timestamp      float64
}

func NewGPSInterpolated() *GPSInterpolated {
	clock := &GPSInterpolated{
		MsgRate:        make(map[string]float64),
		Counts:         make(map[string]int),
		CountsSinceGPS: make(map[string]int),
		Timebase:       0,
		Timestamp:      0,
	}
	return clock
}

func (clock *GPSInterpolated) RewindEvent() {
	clock.Counts = make(map[string]int)
	clock.CountsSinceGPS = make(map[string]int)
}

func (clock *GPSInterpolated) FindTimeBase(message *DataFileMessage, firstUsStamp int) {
	week, ok := message.GetAttr("Week").(int)
	if !ok {
		return
	}

	timeMs, ok := message.GetAttr("TimeMS").(int)
	if !ok {
		return
	}

	t := clock.GPSTimeToTime(week, timeMs)

	var v = t - float64(firstUsStamp)*0.001
	clock.SetTimebase(v)
	clock.Timestamp = clock.Timebase + float64(firstUsStamp)*0.001
	fmt.Println(clock.Timestamp)
}

func (clock *GPSInterpolated) GPSTimeToTime(week int, msec int) float64 {
	// convert GPS week and TOW to a time in seconds since 1970
	epoch := 86400 * (10*365 + int((1980-1969)/4) + 1 + 6 - 2)
	return float64(epoch) + float64(86400*7*int(week)) + float64(msec)*0.001 - 18
}


func (clock *GPSInterpolated) MessageArrived(message *DataFileMessage) {
	msgType := message.GetType()
	if _, ok := clock.Counts[msgType]; !ok {
		clock.Counts[msgType] = 1
	} else {
		clock.Counts[msgType]++
	}

	if _, ok := clock.CountsSinceGPS[msgType]; !ok {
		clock.CountsSinceGPS[msgType] = 1
	} else {
		clock.CountsSinceGPS[msgType]++
	}

	if msgType == "GPS" || msgType == "GPS2" {
		clock.GPSMessageArrived(message)
	}
}

func (clock *GPSInterpolated) GPSMessageArrived(message *DataFileMessage) {
	var gpsWeek, gpsTimems interface{}

	// msec-style GPS message?
	gpsWeek = message.GetAttr("Week")
	gpsTimems = message.GetAttr("TimeMS")

	if gpsWeek == nil {
		// usec-style GPS message?
		gpsWeek = message.GetAttr("GWk")
		gpsTimems = message.GetAttr("GMS")
		if gpsWeek == nil {
			if message.GetAttr("GPSTime") != nil {
				// PX4-style timestamp;
				return
			}
			gpsWeek = message.GetAttr("Wk")
			if gpsWeek != nil {
				// AvA-style logs
				gpsTimems = message.GetAttr("TWk")
			}
		}
	}

	if gpsWeek == nil || gpsTimems == nil {
		return
	}

	t := clock.GPSTimeToTime(gpsWeek.(int), gpsTimems.(int))
	deltat := t - clock.Timebase
	if deltat <= 0 {
		return
	}

	for msgType := range clock.CountsSinceGPS {
		rate := float64(clock.CountsSinceGPS[msgType]) / deltat
		if rate > clock.MsgRate[msgType] {
			clock.MsgRate[msgType] = rate
		}
	}

	clock.MsgRate["IMU"] = 50.0
	clock.Timebase = t
	clock.CountsSinceGPS = make(map[string]int)
}

func (clock *GPSInterpolated) SetMessageTimestamp(message *DataFileMessage) {
	rate := clock.MsgRate[message.GetType()]
	if rate == 0 {
		rate = 50
	}
	count := clock.CountsSinceGPS[message.GetType()]
	message.SetAttr("_timestamp", clock.Timebase+float64(count)/rate)
}

func (clock *GPSInterpolated) SetTimebase(base float64) {
	clock.Timebase = base
}
