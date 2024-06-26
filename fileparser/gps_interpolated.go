package fileparser

import (
	"log"
)

const (
	MillisecondsInSecond  = 0.001
	SecondsInDay          = 86400
	DaysInYear            = 365
	YearsInLeapCycle      = 4
	EpochLeapYearOffset   = 1980
	EpochStartYear        = 1969
	EpochDaysFromYear     = 6
	EpochDaysFromWeekday  = 2
	DaysInWeek            = 7
	EpochDaysOffset       = 10
	LeapYearAdjustment    = 1
	LeapSecondsAdjustment = 18
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
	weekInterface, _ := message.GetAttribute("Week")
	week, ok := weekInterface.(int)
	if !ok {
		return
	}

	timeMSInterface, _ := message.GetAttribute("TimeMS")
	timeMs, ok := timeMSInterface.(int)
	if !ok {
		return
	}

	t := clock.GPSTimeToUnixTime(week, timeMs)

	var v = t - float64(firstUsStamp)*MillisecondsInSecond
	clock.SetTimebase(v)
	clock.Timestamp = clock.Timebase + float64(firstUsStamp)*MillisecondsInSecond
}

func (clock *GPSInterpolated) GPSTimeToUnixTime(week int, msec int) float64 {
	// convert GPS week and TOW (Time Of Week) to a time in seconds since 1970
	// epoch := 86400 * (10*365 + int((1980-1969)/4) + 1 + 6 - 2)
	// return float64(epoch) + float64(86400*7*week) + float64(msec)* 0.001 - 18
	epoch := SecondsInDay * (EpochDaysOffset*DaysInYear + int((EpochLeapYearOffset-EpochStartYear)/YearsInLeapCycle) + LeapYearAdjustment + EpochDaysFromYear - EpochDaysFromWeekday)
	return float64(epoch) + float64(SecondsInDay*DaysInWeek*week) + float64(msec)*MillisecondsInSecond - LeapSecondsAdjustment
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

	// First attempt: msec-style GPS message
	gpsWeek, _ = message.GetAttribute("Week")
	gpsTimems, _ = message.GetAttribute("TimeMS")

	// Second attempt: usec-style GPS message
	if gpsWeek == nil {
		gpsWeek, _ = message.GetAttribute("GWk")
		gpsTimems, _ = message.GetAttribute("GMS")
	}

	// Third attempt: PX4-style timestamp
	if gpsWeek == nil {
		gpsTimeInterface, _ := message.GetAttribute("GPSTime")
		if gpsTimeInterface != nil {
			// PX4-style timestamp
			return
		}

		// Fourth attempt: AvA-style logs
		gpsWeek, _ = message.GetAttribute("Wk")
		if gpsWeek != nil {
			gpsTimems, _ = message.GetAttribute("TWk")
		}
	}

	// If no valid GPS time found, return
	if gpsWeek == nil || gpsTimems == nil {
		return
	}

	// Convert GPS time to Unix time
	t := clock.GPSTimeToUnixTime(gpsWeek.(int), gpsTimems.(int))
	deltat := t - clock.Timebase

	// If the time difference is non-positive, return
	if deltat <= 0 {
		return
	}

	// Update message rates based on the time difference
	for msgType := range clock.CountsSinceGPS {
		rate := float64(clock.CountsSinceGPS[msgType]) / float64(deltat)
		if rate > clock.MsgRate[msgType] {
			clock.MsgRate[msgType] = rate
		}
	}

	// Set IMU message rate to 50.0
	clock.MsgRate["IMU"] = 50.0

	// Update timebase and reset message counts
	clock.Timebase = t
	clock.CountsSinceGPS = make(map[string]int)
}

func (clock *GPSInterpolated) SetMessageTimestamp(message *DataFileMessage) {
	rate := clock.MsgRate[message.GetType()]
	if rate == 0 {
		rate = 50
	}
	count := clock.CountsSinceGPS[message.GetType()]
	err := message.SetAttribute("_timestamp", clock.Timebase+float64(count)/rate)
	if err != nil {
		// intentionally ignoring error
		log.Printf("Failed to set attribute _timestamp: %v", err)
	}
}

func (clock *GPSInterpolated) SetTimebase(base float64) {
	clock.Timebase = base
}
