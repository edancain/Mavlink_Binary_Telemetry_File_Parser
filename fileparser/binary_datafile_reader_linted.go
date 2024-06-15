package fileparser

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/edsrzf/mmap-go"
)

const (
	HEAD1Const            = 0xA3
	HEAD2Const            = 0x95
	FmtTypeDefault        = 128
	MaxMessageCount       = 256
	EndOfFileGarbageLimit = 528
	FmtName               = "FMT"
	FmtLength             = 89
	FmtFormat             = "BBnNZ"
	PercentMultiplier     = 100.0
	headerSizeAdjustment  = 3
	bytesPerInt16         = 2
	MsgTypeGPS            = "GPS"
	MsgTypeGPS2           = "GPS2"
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

// ArduPlane
/*var modeMappingAPM = map[int]string{
	0:  "MANUAL",
	1:  "CIRCLE",
	2:  "STABILIZE",
	3:  "TRAINING",
	4:  "ACRO",
	5:  "FBWA",
	6:  "FBWB",
	7:  "CRUISE",
	8:  "AUTOTUNE",
	10: "AUTO",
	11: "RTL",
	12: "LOITER",
	13: "TAKEOFF",
	14: "AVOID_ADSB",
	15: "GUIDED",
	16: "INITIALISING",
	17: "QSTABILIZE",
	18: "QHOVER",
	19: "QLOITER",
	20: "QLAND",
	21: "QRTL",
	22: "QAUTOTUNE",
	23: "QACRO",
	24: "THERMAL",
	25: "LOITERALTQLAND",
}*/

// ArduCopter
var modeMappingACM = map[int]string{
	0:  "STABILIZE",
	1:  "ACRO",
	2:  "ALT_HOLD",
	3:  "AUTO",
	4:  "GUIDED",
	5:  "LOITER",
	6:  "RTL",
	7:  "CIRCLE",
	8:  "POSITION",
	9:  "LAND",
	10: "OF_LOITER",
	11: "DRIFT",
	13: "SPORT",
	14: "FLIP",
	15: "AUTOTUNE",
	16: "POSHOLD",
	17: "BRAKE",
	18: "THROW",
	19: "AVOID_ADSB",
	20: "GUIDED_NOGPS",
	21: "SMART_RTL",
	22: "FLOWHOLD",
	23: "FOLLOW",
	24: "ZIGZAG",
	25: "SYSTEMID",
	26: "AUTOROTATE",
	27: "AUTO_RTL",
}

type UnpackerFunc func([]byte) ([]interface{}, error)

type BinaryDataFileReader struct {
	fileHandle    *os.File
	dataMap       mmap.MMap
	HEAD1         byte
	HEAD2         byte
	unpackers     map[int]func([]byte) ([]interface{}, error)
	formats       map[int]*DataFileFormat
	zeroTimeBase  bool
	verbose       bool
	prevType      int
	offset        int
	remaining     int
	offsets       [][]int
	typeNums      []byte
	timestamp     int
	counts        []int
	_count        int
	nameToID      map[string]int
	idToName      map[int]string
	MavType       MavType
	params        map[string]interface{}
	flightmodes   []interface{}
	flightmode    string
	Messages      map[string]*DataFileMessage
	Percent       float64
	clock         *GPSInterpolated
	dataLen       int
	binaryFormats []string
}

func NewBinaryDataFileReader(file io.Reader, dataLen int, zeroTimeBase bool) (*BinaryDataFileReader, error) {
	var columns = []string{"Type", "Length", "Name", "Format", "Columns"}
	df, err := NewDataFileFormat(FmtTypeDefault, FmtName, FmtLength, FmtFormat, columns, nil)
	if err != nil {
		return nil, err
	}

	reader := &BinaryDataFileReader{
		HEAD1:        HEAD1Const,
		HEAD2:        HEAD2Const,
		verbose:      false,
		offset:       0,
		remaining:    0,
		typeNums:     nil,
		zeroTimeBase: zeroTimeBase,
		prevType:     0,
		MavType:      MavTypeGeneric,
		params:       make(map[string]interface{}),
		flightmodes:  nil,
		flightmode:   modeStringACM(0),
		Messages:     map[string]*DataFileMessage{"MAV": nil, "__MAV__": nil},
		Percent:      0.0,
		clock:        nil,
		formats:      make(map[int]*DataFileFormat),
	}
	reader.formats[df.Typ] = df
	reader.binaryFormats = []string{}

	if filehandle, ok := file.(*os.File); ok {
		reader.fileHandle = filehandle

		fileInfo, err := reader.fileHandle.Stat()
		if err != nil {
			return nil, err
		}

		reader.dataLen = int(fileInfo.Size())

		reader.unpackers = make(map[int]func([]byte) ([]interface{}, error))

		reader.dataMap, err = mmap.MapRegion(reader.fileHandle, reader.dataLen, mmap.RDONLY, 0, 0)
		if err != nil {
			return nil, err
		}
	} else {
		reader.dataLen = dataLen

		reader.dataMap = make([]byte, dataLen)
		_, err := io.ReadFull(file, reader.dataMap)
		if err != nil {
			return nil, err
		}
	}

	return reader, nil
}

func (reader *BinaryDataFileReader) init() {
	// Implementation of init function
	reader.offset = 0
	reader.remaining = reader.dataLen
	reader.prevType = 0
	reader.initClock()
	reader._rewind()
	reader.initArrays()
}

func (reader *BinaryDataFileReader) initClock() {
	reader._rewind()
	reader.InitClockGPSInterpolated()

	var firstUsStamp int
	var firstMsStamp int
	count := 0

	for {
		count++
		message, err := reader.recvMsg()
		if err != nil {
			break
		}

		msgType := message.GetType()

		firstUsStamp = reader.checkFirstUsStamp(firstUsStamp, &message)
		firstMsStamp = reader.checkFirstMsStamp(firstMsStamp, msgType, &message)

		if msgType == MsgTypeGPS || msgType == MsgTypeGPS2 {
			if reader.processGPSTime(&message, firstMsStamp) {
				break
			}
		}
	}

	reader._rewind()
}

func (reader *BinaryDataFileReader) checkFirstUsStamp(firstUsStamp int, message *DataFileMessage) int {
	if firstUsStamp == 0 {
		usTimeStampInterface, _ := message.GetAttr("TimeUS")
		usTimeStamp, ok := usTimeStampInterface.(int)
		if ok && usTimeStamp != 0 {
			firstUsStamp = usTimeStamp
		}
	}
	return firstUsStamp
}

func (reader *BinaryDataFileReader) checkFirstMsStamp(firstMsStamp int, msgType string, message *DataFileMessage) int {
	if firstMsStamp == 0 && msgType != MsgTypeGPS && msgType != MsgTypeGPS2 {
		msTimeStampInterface, _ := message.GetAttr("TimeMS")
		msTimeStamp, ok := msTimeStampInterface.(int)
		if ok && msTimeStamp != 0 {
			firstMsStamp = msTimeStamp
		}
	}
	return firstMsStamp
}

func (reader *BinaryDataFileReader) processGPSTime(message *DataFileMessage, firstMsStamp int) bool {
	var timeUS, gwk, t, week int
	var err error

	timeUS, err = processAttr(message, "TimeUS")
	if err != nil {
		log.Println("Error processing TimeUS:", err)
		return false
	}

	gwk, err = processAttr(message, "GWk")
	if err != nil {
		log.Println("Error processing GWk:", err)
		return false
	}

	if timeUS != 0 && gwk != 0 {
		if !reader.zeroTimeBase {
			reader.clock.FindTimeBase(message, firstMsStamp)
		}
		return true
	}

	t, err = processAttr(message, "T")
	if err != nil {
		log.Println("Error processing T:", err)
		return false
	}

	week, err = processAttr(message, "Week")
	if err != nil {
		log.Println("Error processing Week:", err)
		return false
	}

	if t != 0 && week != 0 {
		if firstMsStamp == 0 {
			firstMsStamp = t
		}

		if !reader.zeroTimeBase {
			reader.clock.FindTimeBase(message, firstMsStamp)
		}
		return true
	}

	return false
}

func processAttr(message *DataFileMessage, attr string) (int, error) {
	attrInterface, err := message.GetAttr(attr)
	if err != nil {
		return 0, err
	}

	value, err := processTime(attrInterface)
	if err != nil {
		return 0, err
	}

	return value, nil
}

func processTime(timeInterface interface{}) (int, error) {
	time, ok := timeInterface.(int)
	if !ok {
		return 0, fmt.Errorf("invalid time type")
	}
	return time, nil
}

func (reader *BinaryDataFileReader) InitClockGPSInterpolated() {
	clock := NewGPSInterpolated()
	reader.clock = clock
}

func (reader *BinaryDataFileReader) _rewind() {
	reader.offset = 0
	reader.remaining = reader.dataLen
	reader.typeNums = nil
	reader.timestamp = 0

	reader.Messages = map[string]*DataFileMessage{
		"MAV":     nil,
		"__MAV__": nil,
	}

	if reader.flightmodes != nil && len(reader.flightmodes) > 0 {
		reader.flightmodes = []interface{}{reader.flightmodes[0]}
	} else {
		reader.flightmodes = []interface{}{"UNKNOWN"}
	}
	reader.Percent = 0.0
	if reader.clock != nil {
		reader.clock.RewindEvent()
	}
}

func (reader *BinaryDataFileReader) Rewind() {
	reader._rewind()
}

func (reader *BinaryDataFileReader) initArrays() {
	reader.initializeArrays()
	reader.processMessages()
	reader.aggregateCounts()
	reader.resetOffset()
}

func (reader *BinaryDataFileReader) initializeArrays() {
	reader.offsets = make([][]int, MaxMessageCount)
	reader.counts = make([]int, MaxMessageCount)
	reader._count = 0
	reader.nameToID = make(map[string]int)
	reader.idToName = make(map[int]string)
	reader.formats = make(map[int]*DataFileFormat)
	reader.unpackers = make(map[int]func([]byte) ([]interface{}, error))

	for i := 0; i < MaxMessageCount; i++ {
		reader.offsets[i] = []int{}
		reader.counts[i] = 0
	}
}

func (reader *BinaryDataFileReader) processMessages() {
	typeInstances := make(map[int]map[string]struct{})
	lengths := make([]int, MaxMessageCount)
	for i := range lengths {
		lengths[i] = -1
	}

	ofs := 0
	HEAD1 := int(reader.HEAD1)
	HEAD2 := int(reader.HEAD2)

	for ofs+headerSizeAdjustment < reader.dataLen {
		hdr := reader.dataMap[ofs : ofs+headerSizeAdjustment]
		if int(hdr[0]) != HEAD1 || int(hdr[1]) != HEAD2 {
			reader.handleBadHeader(ofs, hdr)
			ofs++
			continue
		}

		mtype := int(hdr[2])
		reader.offsets[mtype] = append(reader.offsets[mtype], ofs)

		if lengths[mtype] == -1 {
			reader.processFirstMessageType(ofs, mtype, lengths)
		} else {
			reader.processInstanceField(ofs, mtype, typeInstances)
		}

		reader.counts[mtype]++
		mlen := lengths[mtype]

		if mtype == int(FmtTypeDefault) {
			reader.processDefaultFormat(mtype, ofs, mlen)
		}

		if reader.needFmtuType(mtype) {
			reader.processFmtuType(mtype, ofs, mlen)
		}

		ofs += mlen
	}
}

func (reader *BinaryDataFileReader) handleBadHeader(ofs int, hdr []byte) {
	if reader.dataLen-ofs >= EndOfFileGarbageLimit || reader.dataLen < EndOfFileGarbageLimit {
		fmt.Fprintf(os.Stderr, "bad header 0x%02x 0x%02x at %d\n", hdr[0], hdr[1], ofs)
	}
}

func (reader *BinaryDataFileReader) processFirstMessageType(ofs int, mtype int, lengths []int) {
	if _, ok := reader.formats[mtype]; !ok {
		reader.handleUnknownMessageType(ofs, mtype)
		return
	}

	reader.offset = ofs
	if _, err := reader.ParseNext(); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing next: %v\n", err)
		return
	}

	unpacker := reader.formats[mtype].getUnpacker()
	if unpacker == nil {
		return
	}

	reader.unpackers[mtype] = unpacker
	lengths[mtype] = reader.formats[mtype].Len
}

func (reader *BinaryDataFileReader) handleUnknownMessageType(ofs int, mtype int) {
	if reader.dataLen-ofs >= EndOfFileGarbageLimit || reader.dataLen < EndOfFileGarbageLimit {
		fmt.Fprintf(os.Stderr, "unknown msg type 0x%02x (%d) at %d\n", mtype, mtype, ofs)
	}
}

func (reader *BinaryDataFileReader) processInstanceField(ofs int, mtype int, typeInstances map[int]map[string]struct{}) {
	dfmt := reader.formats[mtype]
	if dfmt.InstanceField != nil {
		idata := reader.dataMap[ofs+headerSizeAdjustment+dfmt.InstanceOfs : ofs+headerSizeAdjustment+dfmt.InstanceOfs+dfmt.InstanceLen]

		if _, ok := typeInstances[mtype]; !ok {
			typeInstances[mtype] = make(map[string]struct{})
		}

		idataStr := string(idata)
		if _, ok := typeInstances[mtype][idataStr]; !ok {
			typeInstances[mtype][idataStr] = struct{}{}
			reader.offset = ofs
			if _, err := reader.ParseNext(); err != nil {
				fmt.Fprintf(os.Stderr, "error parsing next: %v\n", err)
				return
			}
		}
	}
}

func (reader *BinaryDataFileReader) aggregateCounts() {
	for _, count := range reader.counts {
		reader._count += count
	}
}

func (reader *BinaryDataFileReader) resetOffset() {
	reader.offset = 0
}

func (reader *BinaryDataFileReader) processDefaultFormat(mtype, ofs, mlen int) {
	body := reader.dataMap[ofs+headerSizeAdjustment : ofs+mlen]
	if len(body)+headerSizeAdjustment < mlen {
		return
	}

	unpacker := reader.unpackers[mtype]
	if unpacker == nil {
		return
	}

	elements, err := unpacker(body)
	if err != nil {
		return
	}

	ftype, _ := elements[0].(int)
	name := nullTerm(string(elements[2].([]byte)))
	length, _ := elements[1].(int)
	format := nullTerm(string(elements[3].([]byte)))

	bytesSlice, ok := elements[4].([]uint8)
	if !ok {
		log.Println("Invalid data type")
		return
	}

	var stringArray []string
	var str string
	for _, b := range bytesSlice {
		if b == 0 {
			if str != "" {
				stringArray = append(stringArray, str)
			}
			str = ""
			continue
		}
		str += string(b)
	}

	var columns []string
	if len(stringArray) > 0 {
		columns = strings.Split(stringArray[0], ",")
	}

	mfmt, err := NewDataFileFormat(ftype, name, length, format, columns, reader.formats[ftype])
	if err != nil {
		return
	}

	reader.formats[ftype] = mfmt
	reader.nameToID[mfmt.Name] = mfmt.Typ
	reader.idToName[mfmt.Typ] = mfmt.Name
}

func (reader *BinaryDataFileReader) needFmtuType(mtype int) bool {
	return reader.formats[mtype].Name == "FMTU"
}

func (reader *BinaryDataFileReader) processFmtuType(mtype, ofs, mlen int) {
	dfmt := reader.formats[mtype]
	body := reader.dataMap[ofs+headerSizeAdjustment : ofs+mlen]
	if len(body)+headerSizeAdjustment < mlen {
		return
	}

	unpacker := reader.unpackers[mtype]
	if unpacker == nil {
		return
	}

	elements, err := unpacker(body)
	if err != nil {
		return
	}

	ftype, _ := elements[1].(int)
	if fmt2, ok := reader.formats[ftype]; ok {
		if _, colExists := dfmt.Colhash["UnitIds"]; colExists {
			unitIds := nullTerm(string(elements[dfmt.Colhash["UnitIds"]].([]byte)))
			fmt2.SetUnitIds(&unitIds)
		}
		if _, colExists := dfmt.Colhash["MultIds"]; colExists {
			multIds := nullTerm(string(elements[dfmt.Colhash["MultIds"]].([]byte)))
			fmt2.SetMultIds(&multIds)
		}
	}
}

func (reader *BinaryDataFileReader) recvMsg() (DataFileMessage, error) {
	msg, err := reader.ParseNext()
	if err != nil {
		return DataFileMessage{}, err
	}
	return *msg, nil
}

func (reader *BinaryDataFileReader) ParseNext() (*DataFileMessage, error) {
	var skipType []byte
	var msgType int

	for {
		if reader.dataLen-reader.offset < headerSizeAdjustment {
			return nil, fmt.Errorf("insufficient data for message header")
		}

		hdr := reader.dataMap[reader.offset : reader.offset+headerSizeAdjustment]
		if hdr[0] == reader.HEAD1 && hdr[1] == reader.HEAD2 {
			if skipType != nil {
				skipType = nil
			}

			msgType = int(hdr[2])

			if _, ok := reader.formats[msgType]; ok {
				reader.prevType = msgType
				break
			}
		}

		if skipType == nil {
			skipType = hdr
		}

		reader.offset++
		reader.remaining--
	}

	reader.offset += headerSizeAdjustment
	reader.remaining = len(reader.dataMap) - reader.offset

	dfmt, ok := reader.formats[msgType]
	if !ok {
		return nil, fmt.Errorf("unknown message type: %d", msgType)
	}

	if reader.remaining < dfmt.Len-headerSizeAdjustment {
		if reader.verbose {
			log.Println("out of data")
		}
		return nil, fmt.Errorf("out of data")
	}

	body := reader.dataMap[reader.offset : reader.offset+dfmt.Len-headerSizeAdjustment]
	elements, err := reader.unpackMessageElements(msgType, dfmt, body)
	if err != nil {
		return nil, err
	}

	if elements == nil {
		return reader.ParseNext()
	}

	if dfmt.Name == FmtName {
		if err := reader.processFmtMessage(elements); err != nil {
			return reader.ParseNext()
		}
	}

	reader.offset += dfmt.Len - headerSizeAdjustment
	reader.remaining = reader.dataLen - reader.offset
	m := NewDFMessage(dfmt, elements, true, reader)

	reader.addMsg(m)
	reader.Percent = PercentMultiplier * float64(reader.offset) / float64(reader.dataLen)

	return m, nil
}

func (reader *BinaryDataFileReader) unpackMessageElements(msgType int, dfmt *DataFileFormat, body []byte) ([]interface{}, error) {
	if _, ok := reader.unpackers[msgType]; !ok {
		unpacker := dfmt.getUnpacker()
		reader.unpackers[msgType] = unpacker
	}
	dfmt.MsgStruct = "<" + dfmt.Format

	elements, err := reader.unpackers[msgType](body)
	if err != nil {
		if reader.remaining < EndOfFileGarbageLimit {
			return nil, fmt.Errorf("no valid data")
		}
		fmt.Fprintf(os.Stderr, "Failed to parse %s/%s with len %d (remaining %d)\n",
			dfmt.Name, dfmt.MsgStruct, len(body), reader.remaining)
		return nil, err
	}

	for _, aIndex := range dfmt.AIndexes {
		if aIndex < len(elements) {
			elements[aIndex], _ = bytesToInt16Slice(elements[aIndex].([]byte))
		}
	}

	return elements, nil
}

func (reader *BinaryDataFileReader) processFmtMessage(elements []interface{}) error {
	ftype, ok := elements[0].(int)
	if !ok {
		return fmt.Errorf("unexpected type for FMT message")
	}

	nameBytes, ok := elements[2].([]byte)
	if !ok {
		return fmt.Errorf("unexpected type for FMT message name")
	}
	name := string(bytes.TrimRight(nameBytes, "\x00"))

	formatBytes, ok := elements[3].([]byte)
	if !ok {
		return fmt.Errorf("unexpected type for FMT message format")
	}
	format := string(bytes.TrimRight(formatBytes, "\x00"))

	bytesSlice, ok := elements[4].([]uint8)
	if !ok {
		return fmt.Errorf("invalid data type")
	}

	columns := parseColumns(bytesSlice)

	length, ok := elements[1].(int)
	if !ok {
		return fmt.Errorf("unexpected type for FMT message length")
	}

	mfmt, err := NewDataFileFormat(ftype, name, length, format, columns, reader.formats[ftype])
	if err != nil {
		return err
	}
	reader.formats[ftype] = mfmt

	return nil
}

func parseColumns(bytesSlice []uint8) []string {
	var stringArray []string
	var str string
	for _, b := range bytesSlice {
		if b == 0 {
			if str != "" {
				stringArray = append(stringArray, str)
			}
			str = ""
			continue
		}
		str += string(b)
	}

	if len(stringArray) > 0 {
		return strings.Split(stringArray[0], ",")
	}
	return []string{}
}

func bytesToInt16Slice(b []byte) ([]int16, error) {
	if len(b)%bytesPerInt16 != 0 {
		return nil, fmt.Errorf("bytesToInt16Slice: byte slice length is not a multiple of 2")
	}

	result := make([]int16, len(b)/bytesPerInt16)
	for i := 0; i < len(b); i += bytesPerInt16 {
		result[i/bytesPerInt16] = int16(binary.LittleEndian.Uint16(b[i : i+bytesPerInt16]))
	}

	return result, nil
}

func (reader *BinaryDataFileReader) FindUnusedFormat() int {
	for i := 254; i > 1; i-- {
		if _, ok := reader.formats[i]; !ok {
			return i
		}
	}
	return 0
}

func (reader *BinaryDataFileReader) AddFormat(dfmt *DataFileFormat) *DataFileFormat {
	newType := reader.FindUnusedFormat()
	if newType == 0 {
		return nil
	}
	dfmt.Typ = newType
	reader.formats[newType] = dfmt
	return dfmt
}

func (reader *BinaryDataFileReader) addMsg(m *DataFileMessage) {
	msgType := m.GetType()
	reader.Messages[msgType] = m

	message := m.GetMessage()
	if msgType == "MSG" && message != "" {
		switch {
		case strings.Contains(message, "Rover"):
			reader.MavType = MavTypeGroundRover
		case strings.Contains(message, "Plane"):
			reader.MavType = MavTypeFixedWing
		case strings.Contains(message, "Copter"):
			reader.MavType = MavTypeQuadrotor
		case strings.HasPrefix(message, "Antenna"):
			reader.MavType = MavTypeAntennaTracker
		case strings.Contains(message, "ArduSub"):
			reader.MavType = MavTypeSubmarine
		case strings.Contains(message, "Blimp"):
			reader.MavType = MavTypeAirship
		}
	}

	// Code to demonstrate that we can capture the flightmode settings throughout the flight
	if msgType == "MODE" {
		mode := m.GetMode()
		if mode != -1 {
			reader.flightmode = modeStringACM(mode)
		} else {
			reader.flightmode = "UNKNOWN"
		}
	}

	// Future work around the PX4 not Ardupilot.
	// if msgType == "STAT" && contains(m.FieldNames, "MainState") {
	//	d.flightmode = m.MainState // Placeholder for px4(m.MainState)
	//	}
}

func modeStringACM(modeNumber int) string {
	if mode, ok := modeMappingACM[modeNumber]; ok {
		return mode
	}
	return fmt.Sprintf("Mode(%d)", modeNumber)
}

// func modeStringAPM(modeNumber int) string {
//	if mode, ok := modeMappingAPM[modeNumber]; ok {
//		return mode
//	}
//	return fmt.Sprintf("Mode(%d)", modeNumber)
// }
