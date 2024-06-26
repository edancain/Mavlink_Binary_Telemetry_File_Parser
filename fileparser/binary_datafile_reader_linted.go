package fileparser

import (
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
	FormatName            = "FMT"
	FormatLength          = 89
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

// NewBinaryDataFileReader creates a new reader for binary data files
func NewBinaryDataFileReader(file io.Reader, dataLen int, zeroTimeBase bool) (*BinaryDataFileReader, error) {
	// Defining columns for the data file format
	var columns = []string{"Type", "Length", "Name", "Format", "Columns"}
	df, err := NewDataFileFormat(FmtTypeDefault, FormatName, FormatLength, FmtFormat, columns, nil)
	if err != nil {
		return nil, err
	}

	// Initialize the BinaryDataFileReader with default values
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

	/* Add the initial format to the reader's formats map
	 	NB: Bootstrapping the format parsing:
		The initial format added here is typically the "FMT" (Format) message format. This is crucial
		because the "FMT" messages themselves define the structure of all other message types in the binary data file.
		Self-describing data: Many binary telemetry or log formats are self-describing, meaning they contain metadata
		about their own structure. The "FMT" message is the key to understanding this structure.
	*/
	reader.formats[df.Typ] = df
	reader.binaryFormats = []string{}

	// Handle file input: either as a file or a byte slice
	if filehandle, ok := file.(*os.File); ok {
		// If it's a file, memory map it for efficient reading
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

	// Initialize the reader (BinaryDataFileReader)
	reader.init()
	return reader, nil
}

func (reader *BinaryDataFileReader) init() {
	reader.offset = 0
	reader.remaining = reader.dataLen
	reader.prevType = 0
	reader.initClock()
	reader.rewind()
	reader.initArrays()
}

// initClock initializes the clock for timestamp handling, this is crucial for GPS data handling
func (reader *BinaryDataFileReader) initClock() {
	reader.rewind()
	reader.InitClockGPSInterpolated()

	var firstMsStamp int
	count := 0

	for {
		count++
		message, err := reader.recvMsg()
		if err != nil {
			break
		}

		msgType := message.GetType()
		firstMsStamp = reader.getFirstMsStamp(firstMsStamp, msgType, &message)

		if msgType == MsgTypeGPS || msgType == MsgTypeGPS2 {
			if reader.processGPSTime(&message, firstMsStamp) {
				break
			}
		}
	}

	reader.rewind()
}

// finds the first valid millisecond timestamp in the data
func (reader *BinaryDataFileReader) getFirstMsStamp(firstMsStamp int, msgType string, message *DataFileMessage) int {
	// Only process if we haven't found a valid timestamp yet and the message is not a GPS message
	if firstMsStamp == 0 && msgType != MsgTypeGPS && msgType != MsgTypeGPS2 {
		msTimeStampInterface, _ := message.GetAttribute("TimeMS")
		msTimeStamp, ok := msTimeStampInterface.(int)
		// If conversion is successful and the timestamp is not zero, use this as the first timestamp
		if ok && msTimeStamp != 0 {
			firstMsStamp = msTimeStamp
		}
	}
	return firstMsStamp
}

// extract and processes GPS time information from a message
func (reader *BinaryDataFileReader) processGPSTime(message *DataFileMessage, firstMsStamp int) bool {
	var timeUS, gps_week, t, week int
	var err error

	// Extract TimeUS (microsecond timestamp) from the message
	timeUS, err = processAttribute(message, "TimeUS")
	if err != nil {
		log.Println("Error processing TimeUS:", err)
	}

	gps_week, err = processAttribute(message, "GWk")
	if err != nil {
		log.Println("Error processing GWk:", err)
	}

	// If both TimeUS and gps_week are valid, use them to find the time base
	if timeUS != 0 && gps_week != 0 {
		if !reader.zeroTimeBase {
			reader.clock.FindTimeBase(message, firstMsStamp)
		}
		return true
	}

	// If TimeUS and GWk are not available, try to use T (millisecond timestamp) and Week
	t, err = processAttribute(message, "T")
	if err != nil {
		log.Println("Error processing T:", err)
		return false
	}

	week, err = processAttribute(message, "Week")
	if err != nil {
		log.Println("Error processing Week:", err)
		return false
	}

	// If both T and Week are valid, use them to find the time base
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

// extracts an attribute from a message and processes it
func processAttribute(message *DataFileMessage, attributeName string) (int, error) {
	// Get the attribute from the DataFileMessage
	attrInterface, err := message.GetAttribute(attributeName)
	if err != nil {
		return 0, err
	}

	// Process the attribute as a time value
	value, err := processTime(attrInterface)
	if err != nil {
		return 0, err
	}

	return value, nil
}

// converts a time interface to an integer
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

// rewind resets the reader to the beginning of the data
func (reader *BinaryDataFileReader) rewind() {
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

// initializes the arrays and processes the messages
func (reader *BinaryDataFileReader) initArrays() {
	reader.initializeArrays()
	reader.processMessages()
	reader.aggregateCounts()
	reader.resetOffset()
}

// set up the initial arrays for offsets and counts
func (reader *BinaryDataFileReader) initializeArrays() {
	reader.offsets = make([][]int, MaxMessageCount)
	reader.counts = make([]int, MaxMessageCount)
	reader._count = 0

	for i := 0; i < MaxMessageCount; i++ {
		reader.offsets[i] = []int{}
		reader.counts[i] = 0
	}
}

// read through the data and process each message
func (reader *BinaryDataFileReader) processMessages() {
	typeInstances := make(map[int]map[string]struct{})
	lengths := make([]int, MaxMessageCount)
	for i := range lengths {
		lengths[i] = -1
	}

	offset := 0
	HEAD1 := int(reader.HEAD1)
	HEAD2 := int(reader.HEAD2)

	for offset+headerSizeAdjustment < reader.dataLen {
		// Check for valid message header
		hdr := reader.dataMap[offset : offset+headerSizeAdjustment]
		if int(hdr[0]) != HEAD1 || int(hdr[1]) != HEAD2 {
			reader.handleBadHeader(offset, hdr)
			offset++
			continue
		}

		mtype := int(hdr[2])
		reader.offsets[mtype] = append(reader.offsets[mtype], offset)

		// Process first occurrence of a message type
		if lengths[mtype] == -1 {
			reader.processFirstMessageType(offset, mtype, lengths)
		} else {
			// Process instance fields for known message types
			reader.processInstanceField(offset, mtype, typeInstances)
		}

		reader.counts[mtype]++
		mlen := lengths[mtype]

		// Process format messages
		if mtype == int(FmtTypeDefault) {
			reader.processDefaultFormat(mtype, offset, mlen)
		}

		// Process format unit messages
		if reader.needFmtuType(mtype) {
			reader.processFmtuType(mtype, offset, mlen)
		}

		offset += mlen
	}
}

// log errors for invalid message headers
func (reader *BinaryDataFileReader) handleBadHeader(offset int, header []byte) {
	// Log error if not near end of file
	if int(reader.dataLen)-offset >= EndOfFileGarbageLimit || reader.dataLen < EndOfFileGarbageLimit {
		fmt.Fprintf(os.Stderr, "bad header 0x%02x 0x%02x at %d\n", header[0], header[1], offset)
	}
}

func (reader *BinaryDataFileReader) processFirstMessageType(offset int, messageType int, lengths []int) {
	if _, ok := reader.formats[messageType]; !ok {
		reader.handleUnknownMessageType(offset, messageType)
		return
	}

	reader.offset = offset
	if _, err := reader.ParseNext(); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing next: %v\n", err)
		return
	}

	unpacker := reader.formats[messageType].getUnpacker()
	if unpacker == nil {
		return
	}

	reader.unpackers[messageType] = unpacker
	lengths[messageType] = reader.formats[messageType].Len
}

func (reader *BinaryDataFileReader) handleUnknownMessageType(offset int, messageType int) {
	if int(reader.dataLen)-offset >= EndOfFileGarbageLimit || reader.dataLen < EndOfFileGarbageLimit {
		fmt.Fprintf(os.Stderr, "unknown msg type 0x%02x (%d) at %d\n", messageType, messageType, offset)
	}
}

func (reader *BinaryDataFileReader) processInstanceField(offset int, messageType int, typeInstances map[int]map[string]struct{}) {
	dataFileFormat := reader.formats[messageType]
	if dataFileFormat.InstanceField != nil {
		// Extract instance data
		idata := reader.dataMap[offset+headerSizeAdjustment+dataFileFormat.InstanceOffset : offset+headerSizeAdjustment+dataFileFormat.InstanceOffset+dataFileFormat.InstanceLength]

		// Initialize map for this message type if not exists
		if _, ok := typeInstances[messageType]; !ok {
			typeInstances[messageType] = make(map[string]struct{})
		}

		// Process new instance
		idataStr := string(idata)
		if _, ok := typeInstances[messageType][idataStr]; !ok {
			typeInstances[messageType][idataStr] = struct{}{}
			reader.offset = offset
			if _, err := reader.ParseNext(); err != nil {
				fmt.Fprintf(os.Stderr, "error parsing next: %v\n", err)
				return
			}
		}
	}
}

// sum up all message counts
func (reader *BinaryDataFileReader) aggregateCounts() {
	for _, count := range reader.counts {
		reader._count += count
	}
}

// set the offset back to the beginning
func (reader *BinaryDataFileReader) resetOffset() {
	reader.offset = 0
}

// handles the processing of format messages
func (reader *BinaryDataFileReader) processDefaultFormat(messageType, offset, messageLength int) {
	body := reader.dataMap[offset+headerSizeAdjustment : offset+messageLength]
	if len(body)+headerSizeAdjustment < messageLength {
		return
	}

	unpacker := reader.unpackers[messageType]
	if unpacker == nil {
		return
	}

	elements, err := unpacker(body)
	if err != nil {
		return
	}

	reader.processFmtMessage(elements)
}

func (reader *BinaryDataFileReader) needFmtuType(messageType int) bool {
	return reader.formats[messageType].Name == "FMTU"
}

// process FMTU (Format Unit) messages
func (reader *BinaryDataFileReader) processFmtuType(messageType, offset, messageLength int) {
	dataFormat := reader.formats[messageType]
	body := reader.dataMap[offset+headerSizeAdjustment : offset+messageLength]
	if len(body)+headerSizeAdjustment < messageLength {
		return
	}

	// Get the unpacker function for this message type
	unpacker := reader.unpackers[messageType]
	if unpacker == nil {
		return
	}

	// Unpack the message elements
	elements, err := unpacker(body)
	if err != nil {
		return
	}

	// Extract the format type from the elements
	// Not all message formats may include unit or multiplier information.
	// By checking for existence first, the code can handle different types of format definitions flexibly.
	// Unit IDs and Multiplier IDs might be optional metadata for some data types.
	// Only setting them when they exist allows the system to work with both detailed and simple data formats.

	formatType, _ := elements[1].(int)
	if fmt2, ok := reader.formats[formatType]; ok {
		// If UnitIds exist in the column hash, set them for the format
		if _, columnExists := dataFormat.ColumnHash["UnitIds"]; columnExists {
			unitIds := nullTerm(string(elements[dataFormat.ColumnHash["UnitIds"]].([]byte)))
			fmt2.SetUnitIds(&unitIds)
		}

		if _, columnExists := dataFormat.ColumnHash["MultIds"]; columnExists {
			multIds := nullTerm(string(elements[dataFormat.ColumnHash["MultIds"]].([]byte)))
			fmt2.SetMultIds(&multIds)
		}
	}
}

// During clock initialization, the reader needs to process messages sequentially until it finds
// the necessary time information.
// This function allows the initialization code to easily retrieve messages one by one without
// directly dealing with ParseNext()
func (reader *BinaryDataFileReader) recvMsg() (DataFileMessage, error) {
	message, err := reader.ParseNext()
	if err != nil {
		return DataFileMessage{}, err
	}
	return *message, nil
}

func (reader *BinaryDataFileReader) ParseNext() (*DataFileMessage, error) {
	var messageType int

	// Loop until a valid message header is found
	for {
		if reader.dataLen-reader.offset < headerSizeAdjustment {
			return nil, fmt.Errorf("insufficient data for message header")
		}

		header := reader.dataMap[reader.offset : reader.offset+headerSizeAdjustment]
		if header[0] == reader.HEAD1 && header[1] == reader.HEAD2 {
			messageType = int(header[2])

			// Check if the message type is known
			if _, ok := reader.formats[messageType]; ok {
				reader.prevType = messageType
				break
			}
		}

		reader.offset++
		reader.remaining--
	}

	reader.offset += headerSizeAdjustment
	reader.remaining = len(reader.dataMap) - reader.offset

	// Get the format for this message type
	dataFormat, ok := reader.formats[messageType]
	if !ok {
		return nil, fmt.Errorf("unknown message type: %d", messageType)
	}

	// Check if there's enough data for the full message
	if reader.remaining < dataFormat.Len-headerSizeAdjustment {
		return nil, fmt.Errorf("out of data")
	}

	// Extract the message body
	body := reader.dataMap[reader.offset : reader.offset+dataFormat.Len-headerSizeAdjustment]

	// Unpack the message elements
	elements, err := reader.unpackMessageElements(messageType, dataFormat, body)
	if err != nil {
		return nil, err
	}

	if elements == nil {
		return reader.ParseNext()
	}

	// If this is a format message, process it
	if dataFormat.Name == FormatName {
		if err := reader.processFmtMessage(elements); err != nil {
			return reader.ParseNext()
		}
	}

	// Update reader state
	reader.offset += dataFormat.Len - headerSizeAdjustment
	reader.remaining = reader.dataLen - reader.offset
	dataFileMessage := NewDFMessage(dataFormat, elements, true, reader)

	// Add the message to the reader's message list
	reader.addMessage(dataFileMessage)

	// Update progress percentage
	reader.Percent = PercentMultiplier * float64(reader.offset) / float64(reader.dataLen)

	return dataFileMessage, nil
}

// unpack the message elements based on the message type and format
func (reader *BinaryDataFileReader) unpackMessageElements(messageType int, dataFormat *DataFileFormat, body []byte) ([]interface{}, error) {
	// If unpacker for this message type doesn't exist, create one
	if _, ok := reader.unpackers[messageType]; !ok {
		unpacker := dataFormat.getUnpacker()
		reader.unpackers[messageType] = unpacker
	}
	// Set the message structure format
	dataFormat.MessageStruct = "<" + dataFormat.Format

	// Unpack the message body
	elements, err := reader.unpackers[messageType](body)
	if err != nil {
		// Handle error if near end of file
		if reader.remaining < EndOfFileGarbageLimit {
			return nil, fmt.Errorf("no valid data")
		}

		fmt.Fprintf(os.Stderr, "Failed to parse %s/%s with len %d (remaining %d)\n",
			dataFormat.Name, dataFormat.MessageStruct, len(body), reader.remaining)
		return nil, err
	}

	// Convert specific elements to int16 slices if needed
	for _, aIndex := range dataFormat.AIndexes {
		if aIndex < len(elements) {
			elements[aIndex], _ = bytesToInt16Slice(elements[aIndex].([]byte))
		}
	}

	return elements, nil
}

// process a format (FMT) message
func (reader *BinaryDataFileReader) processFmtMessage(elements []interface{}) error {
	formatType, ok := elements[0].(int)
	if !ok {
		return fmt.Errorf("unexpected type for FMT message")
	}

	// Extract and process name, format, and columns
	nameBytes, _ := elements[2].(string)
	name := strings.TrimRight(nameBytes, "\x00")
	formatBytes, _ := elements[3].(string)
	format := strings.TrimRight(formatBytes, "\x00")
	bytesSlice, _ := elements[4].(string)
	btslicename := strings.TrimRight(bytesSlice, "\x00")
	columns := strings.Split(btslicename, ",")
	length, _ := elements[1].(int)

	// Create new data file format
	dataFormat, err := NewDataFileFormat(formatType, name, length, format, columns, reader.formats[formatType])
	if err != nil {
		return err
	}

	// Add new format to reader's formats
	reader.formats[formatType] = dataFormat

	return nil
}

// converts a byte slice to a slice of int16
// a slice is a flexible and powerful data structure that provides a view into an
// underlying array. Technically, a slice consists of three components:
// A pointer to the first element of the array that the slice references
// The length of the slice (the number of elements it contains)
// The capacity of the slice (the maximum number of elements it can contain without reallocation)
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

// finds an unused format type number. By searching for unused IDs, it ensures that new formats
// don't overwrite or conflict with existing ones.
func (reader *BinaryDataFileReader) FindUnusedFormat() int {
	for i := 254; i > 1; i-- {
		if _, ok := reader.formats[i]; !ok {
			return i
		}
	}
	return 0
}

// adds a new format to the reader's formats
func (reader *BinaryDataFileReader) AddFormat(dfmt *DataFileFormat) *DataFileFormat {
	newType := reader.FindUnusedFormat()
	if newType == 0 {
		return nil
	}
	dfmt.Typ = newType
	reader.formats[newType] = dfmt
	return dfmt
}

func (reader *BinaryDataFileReader) addMessage(dataMessage *DataFileMessage) {
	messageType := dataMessage.GetType()
	reader.Messages[messageType] = dataMessage

	message := dataMessage.GetMessage()
	if messageType == "MSG" && message != "" {
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
	if messageType == "MODE" {
		mode := dataMessage.GetMode()
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
