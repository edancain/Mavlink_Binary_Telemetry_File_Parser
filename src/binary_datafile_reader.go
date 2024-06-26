package src

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"io"

	"github.com/edsrzf/mmap-go"
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
var modeMappingAPM = map[int]string{
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
}

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
	flightmode	  string
	Messages      map[string]*DataFileMessage
	Percent       float64
	clock         *GPSInterpolated
	dataLen       int
	binaryFormats []string
}

// NewBinaryDataFileReader creates a new reader for binary data files
func NewBinaryDataFileReader(file io.Reader, dataLen int, zeroTimeBase bool, progressCallback func(int)) (*BinaryDataFileReader, error) {
	// Defining columns for the data file format
	var columns = []string{"Type", "Length", "Name", "Format", "Columns"}

	df, err := NewDataFileFormat(0x80, "FMT", 89, "BBnNZ", columns, nil)
	if err != nil {
		return nil, err
	}

	// Initialize the BinaryDataFileReader with default values
	reader := &BinaryDataFileReader{
		HEAD1:        0xA3,
		HEAD2:        0x95,
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

		reader.dataMap, err = mmap.MapRegion(reader.fileHandle, int(reader.dataLen), mmap.RDONLY, 0, 0)

		if err != nil {
			return nil, err
		}
	} else {
		// If it's not a file, read the data into a byte slice
		reader.dataLen = dataLen

		reader.dataMap = make([]byte, dataLen)
		_, err := io.ReadFull(file, reader.dataMap)
		if err != nil {
			return nil, err
		}
	}

	// Initialize the reader
	reader.init()
	return reader, nil
}

func (reader *BinaryDataFileReader) init() {
	reader.offset = 0
	reader.remaining = reader.dataLen
	reader.prevType = 0
	reader.initClock()
	reader.rewind()
	reader.initArrays(progressCallback)
}

// initClock initializes the clock for timestamp handling, this is crucial for GPS data handling
func (reader *BinaryDataFileReader) initClock() {
	reader.rewind()

	reader.InitClockGPSInterpolated()
	var firstUsStamp int
	firstUsStamp = 0
	var firstMsStamp int
	firstMsStamp = 0
	count := 0
	for {
		count += 1
		fmt.Println(count)
		message, err := reader.recvMsg()
		if err != nil {
			break
		}

		msgType := message.GetType()

		if firstUsStamp == 0 {
			usTimeStamp, ok := message.GetAttr("TimeUS").(int)
			if ok {
				if usTimeStamp != 0 {
					if firstUsStamp == 0 {
						firstUsStamp = usTimeStamp
					}
				}
			}
		}

		if firstMsStamp == 0 && msgType != "GPS" && msgType != "GPS2" {
			msTimeStamp, ok := message.GetAttr("TimeMS").(int)
			if ok {
				if msTimeStamp != 0 {
					firstMsStamp = msTimeStamp
				}
			}
		}

		if msgType == "GPS" || msgType == "GPS2" {
			timeUS, _ := message.GetAttr("TimeUS").(int)
			gwk, _ := message.GetAttr("GWk").(int)

			if timeUS != 0 && gwk != 0 {
				if !reader.zeroTimeBase {
					reader.clock.FindTimeBase(&message, firstMsStamp)
				}
				break
			}

			t, _ := message.GetAttr("T").(int)
			week, _ := message.GetAttr("Week").(int)

			if t != 0 && week != 0 {
				if firstMsStamp == 0 {
					firstMsStamp = t
				}

				if !reader.zeroTimeBase {
					reader.clock.FindTimeBase(&message, firstMsStamp)
				}
				break
			}
		}
	}
	reader.rewind()
}

func (reader *BinaryDataFileReader) InitClockGPSInterpolated() {
	clock := NewGPSInterpolated()
	reader.clock = clock
}

// resets the reader to the beginning of the data
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

func (reader *BinaryDataFileReader) Rewind() {
	reader.rewind()
}

// initializes arrays for fast message matching
func (reader *BinaryDataFileReader) initArrays(progressCallback func(int)) {
	// Initialize arrays for storing message offsets and counts
	reader.offsets = make([][]int, 256)
	reader.counts = make([]int, 256)
	reader._count = 0
	typeInstances := make(map[int]map[string]struct{})

	for i := 0; i < 256; i++ {
		reader.offsets[i] = []int{}
		reader.counts[i] = 0
	}

	fmtType := int(128)
	fmtuType := int(0)

	ofs := int(0)
	pct := 0

	HEAD1 := int(reader.HEAD1)
	HEAD2 := int(reader.HEAD2)

	lengths := make([]int, 256)
	for i := range lengths {
		lengths[i] = -1
	}

	for ofs + 3 < reader.dataLen {
		hdr := reader.dataMap[ofs : ofs+3]
		if int(hdr[0]) != HEAD1 || int(hdr[1]) != HEAD2 {
			// avoid end of file garbage, 528 bytes has been use consistently throughout this implementation
			// but it needs to be at least 249 bytes which is the block based logging page size (256) less a 6 byte header and
			// one byte of data. Block based logs are sized in pages which means they can have up to 249 bytes of trailing space.
			if int(reader.dataLen)-ofs >= 528 || reader.dataLen < 528 {
				fmt.Fprintf(os.Stderr, "bad header 0x%02x 0x%02x at %d\n", hdr[0], hdr[1], ofs)
			}
			ofs++
			continue
		}

		mtype := int(hdr[2])

		reader.offsets[mtype] = append(reader.offsets[mtype], ofs)

		if lengths[mtype] == -1 {
			if _, ok := reader.formats[mtype]; !ok {
				if int(reader.dataLen)-ofs >= 528 || reader.dataLen < 528 {
					fmt.Fprintf(os.Stderr, "unknown msg type 0x%02x (%d) at %d\n", mtype, mtype, ofs)
				}
				break
			}

			reader.offset = ofs
			reader.ParseNext()

			dfmt, ok := reader.formats[mtype]
			if !ok {
				// Handle the case when the key is not found
				continue
			}
			lengths[mtype] = dfmt.Len

		} else if reader.formats[mtype].InstanceField != nil {
			dfmt := reader.formats[mtype]
			idata := reader.dataMap[ofs+3+dfmt.InstanceOfs : ofs+3+dfmt.InstanceOfs+dfmt.InstanceLen]

			if _, ok := typeInstances[mtype]; !ok {
				typeInstances[mtype] = make(map[string]struct{})
			}

			idataStr := string(idata)
			if _, ok := typeInstances[mtype][idataStr]; !ok {
				typeInstances[mtype][idataStr] = struct{}{}
				reader.offset = ofs
				reader.ParseNext()
			}
		}

		reader.counts[mtype]++
		mlen := lengths[mtype]

		if mtype == fmtType {
			body := reader.dataMap[ofs+3 : ofs+mlen]
			if len(body)+3 < int(mlen) {
				break
			}

			elements, err := reader.unpackers[mtype](body)
			if err != nil {
				// Handle the error
				continue
			}

			ftype := elements[0].(int)
			name := nullTerm(string(elements[2].([]byte)))
			length := elements[1].(int)
			format := nullTerm(string(elements[3].([]byte)))

			// Get the byte slice from elements[4] and convert to string
			bytesSlice, ok := elements[4].([]uint8)
			if !ok {
				fmt.Println("Invalid data type")
				return
			}
			// Convert []uint8 to string array
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

			var columns = []string{}

			if len(stringArray) > 0 {
				columns = strings.Split(stringArray[0], ",") //, ok := elements[4].(string)
			}

			mfmt, err := NewDataFileFormat(ftype, name, length, format, columns, reader.formats[ftype])
			if err != nil {
				// Handle the error
				continue
			}

			reader.formats[ftype] = mfmt
			if mfmt.Name == "FMTU" {
				fmtuType = mfmt.Typ
			}
		}

		if fmtuType != 0 && mtype == fmtuType {
			dfmt := reader.formats[mtype]
			body := reader.dataMap[ofs+3 : ofs+mlen]
			if len(body)+3 < int(mlen) {
				break
			}

			elements, err := reader.unpackers[mtype](body)
			if err != nil {
				// Handle the error
				continue
			}
			ftype := elements[1].(int)
			if _, ok := reader.formats[ftype]; ok {
				fmt2 := reader.formats[ftype]
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

		ofs += mlen
		if progressCallback != nil {
			newPct := (100 * int(ofs)) / int(reader.dataLen)
			if newPct != pct {
				progressCallback(newPct)
				pct = newPct
			}
		}
	}

	for _, count := range reader.counts {
		reader._count += count
	}
	reader.offset = 0
}

func (d *BinaryDataFileReader) recvMsg() (DataFileMessage, error) {
	msg, err := d.ParseNext()
	if err != nil {
		return DataFileMessage{}, err
	}
	return *msg, nil
}

func (reader *BinaryDataFileReader) ParseNext() (*DataFileMessage, error) {
	var skipType []byte
	skipStart := 0
	var msgType int
	for {
		if reader.dataLen-reader.offset < 3 {
			return nil, fmt.Errorf("insufficient data for message header")
		}

		hdr := reader.dataMap[reader.offset : reader.offset+3]
		if hdr[0] == reader.HEAD1 && hdr[1] == reader.HEAD2 {
			// Signature found
			if skipType != nil {
				// Emit message about skipped bytes
				if reader.remaining >= 528 {
					skipBytes := reader.offset - skipStart
					fmt.Printf("Skipped %d bad bytes in log at offset %d, type=%v (prev=%d)\n", skipBytes, skipStart, skipType, reader.prevType)
				}
				skipType = nil
			}

			// check we recognise this message type:
			msgType = int(hdr[2])

			if _, ok := reader.formats[msgType]; ok {
				// recognised message found
				reader.prevType = msgType
				break
			}
		}

		if skipType == nil {
			skipType = hdr
			skipStart = reader.offset
		}

		reader.offset++
		reader.remaining--
	}

	reader.offset += 3
	reader.remaining = len(reader.dataMap) - reader.offset

	dfmt, ok := reader.formats[msgType]
	if !ok {
		return nil, fmt.Errorf("unknown message type: %d", msgType)
	}

	if reader.remaining < dfmt.Len-3 {
		// Out of data
		if reader.verbose {
			fmt.Println("out of data")
		}
		return nil, nil
	}

	body := reader.dataMap[reader.offset : reader.offset+dfmt.Len-3]
	var elements []interface{}

	if _, ok := reader.unpackers[msgType]; !ok {
		if msgType == 130 || msgType == 144 {
			fmt.Println("here")
		}
		if dfmt.MsgStruct == "<BB4s16s64s" {
			fmt.Println("stop here")
		}

		unpacker := dfmt.getUnpacker()

		reader.unpackers[msgType] = unpacker
	}
	dfmt.MsgStruct = "<" + dfmt.Format

	if dfmt.Format == "BIHBcLLeeEefI" {
		fmt.Println("stop here")
	}
	elements, err := reader.unpackers[msgType](body)
	if err != nil {
		if reader.remaining < 528 {
			// We can have garbage at the end of an APM2 log
			return nil, nil
		}
		// We should also cope with other corruption; logs
		// transferred via DataFlash_MAVLink may have blocks of 0s
		// in them, for example
		fmt.Fprintf(os.Stderr, "Failed to parse %s/%s with len %d (remaining %d)\n",
			dfmt.Name, dfmt.MsgStruct, len(body), reader.remaining)
	}

	if elements == nil {
		return reader.ParseNext()
	}

	name := dfmt.Name
	for _, aIndex := range dfmt.AIndexes {
		if aIndex < len(elements) {
			elements[aIndex] = bytesToInt16Slice(elements[aIndex].([]byte))
		}
	}

	if name == "FMT" {
		var ftype int
		// Get the uint8 value from elements[0]
		ftype, ok := elements[0].(int)
		if !ok {
			// Handle the case where elements[0] is not a uint8
			fmt.Printf("Unexpected type for FMT message, expected uint8, got %T\n", elements[0])
			return reader.ParseNext()
		}

		// Get the byte slice from elements[2] and convert to string
		nameBytes, ok := elements[2].([]byte)
		if !ok {
			// Handle the case where elements[2] is not a []byte
			fmt.Printf("Unexpected type for FMT message name, expected []byte, got %T\n", elements[2])
			return reader.ParseNext()
		}
		name := string(bytes.TrimRight(nameBytes, "\x00"))

		// Get the byte slice from elements[3] and convert to string
		formatBytes, ok := elements[3].([]byte)
		if !ok {
			// Handle the case where elements[3] is not a []byte
			fmt.Printf("Unexpected type for FMT message format, expected []byte, got %T\n", elements[3])
			return reader.ParseNext()
		}
		format := string(bytes.TrimRight(formatBytes, "\x00"))

		// Get the byte slice from elements[4] and convert to string
		bytesSlice, ok := elements[4].([]uint8)
		if !ok {
			fmt.Println("Invalid data type")
			return nil, nil
		}

		// Convert []uint8 to string array
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

		var columns = []string{}

		if len(stringArray) > 0 {
			columns = strings.Split(stringArray[0], ",")
		}

		length, ok := elements[1].(int)
		if !ok {
			// Handle the case where elements[1] is not a uint8
			fmt.Printf("Unexpected type for FMT message length, expected uint8, got %T\n", elements[1])
			return reader.ParseNext()
		}

		if name == "GPS" {
			fmt.Println("test")
		}

		mfmt, err := NewDataFileFormat(ftype, name, int(length), format, columns, reader.formats[ftype])
		if err != nil {
			return reader.ParseNext()
		}
		reader.formats[ftype] = mfmt
	}

	reader.offset += dfmt.Len - 3
	reader.remaining = reader.dataLen - reader.offset
	m := NewDFMessage(dfmt, elements, true, reader)

	// Add the message to the parser
	reader.addMsg(m)

	reader.Percent = 100.0 * float64(reader.offset) / float64(reader.dataLen)

	return m, nil
}

func bytesToInt16Slice(b []byte) []int16 {
	if len(b)%2 != 0 {
		panic("bytesToInt16Slice: byte slice length is not a multiple of 2")
	}

	result := make([]int16, len(b)/2)
	for i := 0; i < len(b); i += 2 {
		result[i/2] = int16(binary.LittleEndian.Uint16(b[i : i+2]))
	}

	return result
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

func (d *BinaryDataFileReader) addMsg(m *DataFileMessage) {
	msgType := m.GetType()
	d.Messages[msgType] = m

	message := m.GetMessage()
	if msgType == "MSG" && message != "" {
		if strings.Contains(message, "Rover") {
			d.MavType = MavTypeGroundRover 
		} else if strings.Contains(message, "Plane") {
			d.MavType = MavTypeFixedWing 
		} else if strings.Contains(message, "Copter") {
			d.MavType = MavTypeQuadrotor 
		} else if strings.HasPrefix(message, "Antenna") {
			d.MavType = MavTypeAntennaTracker 
		} else if strings.Contains(message, "ArduSub") {
			d.MavType = MavTypeSubmarine 
		} else if strings.Contains(message, "Blimp") {
			d.MavType = MavTypeAirship
		}
	}

	if msgType == "MODE" {
		mode := m.GetMode()
		if mode != -1 {
			d.flightmode = modeStringACM(mode)
		} else {
			d.flightmode = "UNKNOWN"
		}
	}
}

func modeStringACM(modeNumber int) string {
	if mode, ok := modeMappingACM[modeNumber]; ok {
		return mode
	}
	return fmt.Sprintf("Mode(%d)", modeNumber)
}

func modeStringAPM(modeNumber int) string {
	if mode, ok := modeMappingAPM[modeNumber]; ok {
		return mode
	}
	return fmt.Sprintf("Mode(%d)", modeNumber)
}

func progressCallback(progress int) {
	fmt.Printf("Progress: %d%%\n", progress)
}
