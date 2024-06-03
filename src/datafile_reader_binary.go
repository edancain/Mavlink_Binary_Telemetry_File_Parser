package src

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"strings"

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

type UnpackerFunc func([]byte) ([]interface{}, error)

type DFReaderBinary struct {
	fileHandle *os.File
	dataMap    mmap.MMap
	HEAD1      byte
	HEAD2      byte
	//unpackers  map[byte]UnpackerFunc
	unpackers    map[int]func([]byte) ([]interface{}, error)
	formats      map[int]*DFFormat
	zeroTimeBase bool
	verbose      bool
	prevType     int
	offset       int
	remaining    int
	offsets      [][]int
	typeNums     []byte
	timestamp    int
	counts       []int
	_count       int
	nameToID     map[string]int
	idToName     map[int]string
	mavType      MavType
	params       map[string]interface{}
	flightmodes  []interface{}
	Messages     map[string]*DFMessage
	Percent      float64
    clock        *DFReaderClockGPSInterpolated
	//flightmode   string
	indexes []int
	dataLen int
    binaryFormats []string
}

func NewDFReaderBinary(filename string, zeroTimeBase bool, progressCallback func(int)) (*DFReaderBinary, error) {
    
    var columns = []string{"Type","Length","Name","Format","Columns"}
    df, err := NewDFFormat(0x80, "FMT", 89, "BBnNZ", columns, nil)
    if err != nil {
        return nil, err
    }

	reader := &DFReaderBinary{
		HEAD1:     0xA3,
		HEAD2:     0x95,
		verbose:   false,
		offset:    0,
		remaining: 0,
		typeNums:  nil,
		zeroTimeBase: zeroTimeBase,
		prevType:     0,
		mavType:      MavTypeGeneric,
		params:       make(map[string]interface{}),
		flightmodes:  nil,
		Messages:     map[string]*DFMessage{"MAV": nil, "__MAV__": nil},
		Percent:      0.0,
        clock: nil,
        formats:       make(map[int]*DFFormat),
	}
    reader.formats[df.Typ] = df
    reader.binaryFormats = []string{}

    reader.fileHandle, err = os.Open(filename)
    if err != nil {
        panic(err)
    }

	fileInfo, err := reader.fileHandle.Stat()
	if err != nil {
		return nil, err
	}

	reader.dataLen = int(fileInfo.Size()) //5510271

    reader.unpackers = make(map[int]func([]byte) ([]interface{}, error))

	reader.dataMap, err = mmap.MapRegion(reader.fileHandle, int(reader.dataLen), mmap.RDONLY, 0, 0)

	if err != nil {
		panic(err)
	}

	//reader.initUnpackers()
	reader.init(progressCallback)
	return reader, nil
}

/*func (r *DFReaderBinary) initUnpackers() {
	r.unpackers = make(map[byte]func([]byte) ([]interface{}, error))
	for msgType, format := range r.formats {
		r.unpackers[msgType] = format.getUnpacker()
	}
}*/

func (reader *DFReaderBinary) init(progressCallback func(int)) {
	// Implementation of init function
	reader.offset = 0
	reader.remaining = reader.dataLen
	reader.prevType = 0
	reader.initClock()
	reader._rewind()
	reader.initArrays(progressCallback)
}

func (d *DFReaderBinary) initClock() {
	d._rewind()

	//speculatively create a gps clock in case we don't find anything better
	d.InitClockGPSInterpolated()
    var firstUsStamp int
    firstUsStamp = 0
    var firstMsStamp int
    firstMsStamp = 0
    count := 0
	for {
        count += 1
		message, err := d.recvMsg()
		if err != nil {
			break
		}

		msgType := message.GetType()
        if count == 471 {   
            fmt.Println("test")
        }

        if firstUsStamp == 0{
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
            timeUS, ok := message.GetAttr("TimeUS").(int)
            if !ok {  
            }

            gwk, ok := message.GetAttr("GWk").(int)
            if !ok {  
            }

            if timeUS != 0 && gwk != 0 {
                if !d.zeroTimeBase {
                    d.clock.FindTimeBase(&message, firstMsStamp)// everything-usec-timestamped
                }
                break;
            }

            t, ok := message.GetAttr("T").(int)
            if !ok {
            }

            week, ok := message.GetAttr("Week").(int)
            if !ok {  
            }

            if t != 0 && week != 0 {
                if firstMsStamp == 0 {
                    firstMsStamp = t
                }

                if !d.zeroTimeBase {
                    d.clock.FindTimeBase(&message, firstMsStamp)
                }
                break
            }

        }
	}

	d._rewind()
}

func (d *DFReaderBinary) InitClockGPSInterpolated() {
	clock := NewDFReaderClockGPSInterpolated()
	d.clock = clock
}

func (reader *DFReaderBinary) _rewind() {
	reader.offset = 0
	reader.remaining = reader.dataLen
	reader.typeNums = nil
	reader.timestamp = 0

	reader.Messages = map[string]*DFMessage{
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

func (reader *DFReaderBinary) Rewind() {
	reader._rewind()
}

func (reader *DFReaderBinary) initArrays(progressCallback func(int)) {
	//'''initialise arrays for fast recv_match()'''
	reader.offsets = make([][]int, 256)
	reader.counts = make([]int, 256)
	reader._count = 0
	reader.nameToID = make(map[string]int)
	reader.idToName = make(map[int]string)
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

	for ofs+3 < reader.dataLen {
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
				//fmt.Fprintf("Key %x not found in formats\n", mtype)
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

			//dfmt := reader.formats[mtype]
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
                     if str != ""{
                         stringArray = append(stringArray, str)
                     }
                     str = ""
                     continue
                 }
                 str += string(b)            
             }
     
             var columns = []string{}
     
             if len(stringArray) > 0{
                 columns = strings.Split(stringArray[0], ",")//, ok := elements[4].(string)
             }  

			mfmt, err := NewDFFormat(ftype, name, length, format, columns, reader.formats[ftype])
			if err != nil {
				// Handle the error
				continue
			}

			reader.formats[ftype] = mfmt
			reader.nameToID[mfmt.Name] = mfmt.Typ
			reader.idToName[mfmt.Typ] = mfmt.Name
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

func (d *DFReaderBinary) recvMsg() (DFMessage, error) {
	msg, err := d.ParseNext()
	if err != nil {
		return DFMessage{}, err
	}
	return *msg, nil
}

func (reader *DFReaderBinary) ParseNext() (*DFMessage, error) {
	var skipType []byte
	skipStart:= 0
	//var hdr mmap.MMap
	var msgType int
	for {
		if reader.dataLen - reader.offset < 3 {
			//if reader.offset >= len(reader.dataMap) {
			//	return nil, io.EOF
			//}
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

        if skipType == nil{
            skipType = hdr // (u_ord(hdr[0]), u_ord(hdr[1]), u_ord(hdr[2]))
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

	/*if reader.offset+dfmt.Len > len(reader.dataMap) {
		if reader.verbose {
			fmt.Printf("Insufficient data for message type %s (offset: %d, length: %d, data length: %d)\n",
				dfmt.Name, reader.offset, dfmt.Len, len(reader.dataMap))
		}
		return reader.ParseNext()
	}*/

	body := reader.dataMap[reader.offset : reader.offset + dfmt.Len-3]
	var elements []interface{}

    if _, ok := reader.unpackers[msgType]; !ok {
        if msgType == 130 || msgType == 144 {
            fmt.Println("here")
        }
        if dfmt.MsgStruct == "<BB4s16s64s" {
            fmt.Println("stop here")
        }

        unpacker := dfmt.getUnpacker()
            // Unpack the binary data using the format's message structure
            // You'll need to implement this function based on your specific requirements
            // It should take the binary data and return a slice of interface{} values
            // representing the unpacked elements
            // You can use the `binary` package in Go to perform the unpacking
            // based on the format specified in fmt.MsgStruct
            // For example:
            // var result []interface{}
            // // Perform unpacking using binary.Read or binary.Unpack
            // return result, nil
        //}

        reader.unpackers[msgType] = unpacker
    }
    dfmt.MsgStruct = "<" + dfmt.Format

    if dfmt.Format == "BIHBcLLeeEefI"{
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
                if str != ""{
                    stringArray = append(stringArray, str)
                }
                str = ""
                continue
            }
            str += string(b)            
        }

        var columns = []string{}

        if len(stringArray) > 0{
            columns = strings.Split(stringArray[0], ",")//, ok := elements[4].(string)
        }
		
		//columnsStr = strings.TrimRight(columnsStr, "\x00")
        // Split the string by comma to get the columns
        //columns := strings.Split(columnsStr, ",")   
		// Get the uint8 value from elements[1]
		length, ok := elements[1].(int)
		if !ok {
			// Handle the case where elements[1] is not a uint8
			fmt.Printf("Unexpected type for FMT message length, expected uint8, got %T\n", elements[1])
			return reader.ParseNext()
		}

        if name == "GPS"{
            fmt.Println("test")
        }

		mfmt, err := NewDFFormat(ftype, name, int(length), format, columns, reader.formats[ftype])
		if err != nil {
			return reader.ParseNext()
		}
		reader.formats[ftype] = mfmt
	}

    reader.offset += dfmt.Len - 3
    reader.remaining = reader.dataLen - reader.offset
    m := NewDFMessage(dfmt, elements, true, reader)

    if m.Fmt.Name == "FMTU" {
        // Add to units information
        //FmtType := int(elements[0].(uint8))
        //UnitIds := elements[1].([]byte)
        //MultIds := elements[2].([]byte)
        //if f, ok := reader.formats[byte(FmtType)]; ok {
            //f.SetUnitIds(UnitIds)
            //f.SetMultIds(MultIds)
        //}
    }

    // Add the message to the parser
    // You'll need to implement the _addMsg method based on your specific requirements
    reader.addMsg(m)

    reader.Percent = 100.0 * float64(reader.offset) / float64(reader.dataLen)
    fmt.Println(reader.Percent)

    return m, nil

	/*unpacker := dfmt.getUnpacker()
    if msgType == 130 || msgType == 144 {
        fmt.Println("test")
        //data := []byte{0x03, 0x08, 0x71, 0x18, 0x19, 0x3d, 0x07, 0x09, 0xac, 0x00, 0x83, 0x43, 0x56, 0x14, 0x3b, 0xba, 0x37, 0xb9, 0x00, 0x00, 0x00, 0x00, 0xd7, 0x5a, 0x00, 0x00, 0x04, 0x00, 0x00, 0x00, 0x7c, 0x8a, 0x00, 0x00, 0x8f, 0xc2, 0xf5, 0xbd, 0x18, 0xc2, 0x07, 0x00}
        data := body
        var elems []interface{}
        elems = append(elems, data[0])                           // B
        elems = append(elems, binary.LittleEndian.Uint32(data[1:5]))  // I
        elems = append(elems, binary.LittleEndian.Uint16(data[5:7]))  // H
        elems = append(elems, data[7])                           // B
        elems = append(elems, int16(binary.LittleEndian.Uint16(data[8:10]))) // h
        elems = append(elems, int32(binary.LittleEndian.Uint32(data[10:14]))) // i
        elems = append(elems, int32(binary.LittleEndian.Uint32(data[14:18]))) // i
        elems = append(elems, int32(binary.LittleEndian.Uint32(data[18:22]))) // i
        elems = append(elems, int32(binary.LittleEndian.Uint32(data[22:26]))) // i
        elems = append(elems, binary.LittleEndian.Uint32(data[26:30])) // I
        elems = append(elems, math.Float32frombits(binary.LittleEndian.Uint32(data[30:34]))) // f
        elems = append(elems, binary.LittleEndian.Uint32(data[34:38])) // I
        fmt.Println("Manual unpacking result")
        fmt.Println(elems)
    }

	elements, err := unpacker(body)
    fmt.Println("Unpacking result")
    fmt.Println(elements)

    fmt.Println("body value:")
    fmt.Println(body)
    fmt.Println("dfmt.format value:")
    fmt.Println(dfmt.MsgStruct)
    reader.binaryFormats = addUnique(reader.binaryFormats, dfmt.MsgStruct)
	if err != nil {
		if reader.remaining < 528 {
			// we can have garbage at the end of an APM2 log
			return nil, nil
		}
		// we should also cope with other corruption; logs
		// transferred via DataFlash_MAVLink may have blocks of 0s
		// in them, for example
		fmt.Fprintf(os.Stderr, "Failed to parse %s/%s with len %d (remaining %d): %v\n",
			dfmt.Name, dfmt.MsgStruct, len(body), reader.remaining, err)
		return reader.ParseNext()
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
		var ftype byte
		// Get the uint8 value from elements[0]
		ftype, ok := elements[0].(uint8)
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
		columnsStr, ok := elements[4].(string)
		if !ok {
			// Handle the case where elements[4] is not a []byte
			fmt.Printf("Unexpected type for FMT message columns, expected []byte, got %T\n", elements[4])
			return reader.ParseNext()
		}
		columnsStr = strings.TrimRight(columnsStr, "\x00")
        // Split the string by comma to get the columns
        columns := strings.Split(columnsStr, ",")   
		// Get the uint8 value from elements[1]
		length, ok := elements[1].(uint8)
		if !ok {
			// Handle the case where elements[1] is not a uint8
			fmt.Printf("Unexpected type for FMT message length, expected uint8, got %T\n", elements[1])
			return reader.ParseNext()
		}

        if name == "GPS"{
            fmt.Println("test")
        }

		mfmt, err := NewDFFormat(ftype, name, int(length), format, columns, reader.formats[ftype])
		if err != nil {
			return reader.ParseNext()
		}
		reader.formats[ftype] = mfmt
	}

	reader.offset += dfmt.Len - 3
	reader.remaining = reader.dataLen - reader.offset
	m := NewDFMessage(dfmt, elements, true, reader)

	reader.addMsg(m)
	//if err != nil {
	//    fmt.Printf("bad msg at offset %d: %v\n", reader.offset, err)
	//}
	reader.Percent = 100.0 * float64(reader.offset) / float64(reader.dataLen)

	return m, nil*/
}

func (reader *DFReaderBinary) Print_binaryFormats() {
    fmt.Println("Binary formats:")
    for _, format := range reader.binaryFormats {
        fmt.Println(format)
    }
}

func addUnique(list []string, s string) []string {
    // Create a map where the keys are the strings in the list
    m := make(map[string]bool)
    for _, item := range list {
        m[item] = true
    }

    // If the string is not in the map, add it to the list
    if !m[s] {
        list = append(list, s)
    }

    return list
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

func bytesToInt16Array(b []byte) []int16 {
	if len(b)%2 != 0 {
		return nil
	}
	arr := make([]int16, 0, len(b)/2)
	for i := 0; i < len(b); i += 2 {
		num := int16(binary.LittleEndian.Uint16(b[i : i+2]))
		arr = append(arr, num)
	}
	return arr
}

func (reader *DFReaderBinary) FindUnusedFormat() int {
	for i := 254; i > 1; i-- {
		if _, ok := reader.formats[i]; !ok {
			return i
		}
	}
	return 0
}

func (reader *DFReaderBinary) AddFormat(dfmt *DFFormat) *DFFormat {
	newType := reader.FindUnusedFormat()
	if newType == 0 {
		return nil
	}
	dfmt.Typ = newType
	reader.formats[newType] = dfmt
	return dfmt
}

/*
func (d *DFReaderBinary) MakeMsgbuf(fmt *DFFormat, values []interface{}) []byte {
    /*ret := []byte{0xA3, 0x95, fmt.Typ}
    msgBuf := make([]byte, 0, len(ret)+binary.Size(fmt.MsgStruct))
    msgBuf = append(msgBuf, ret...)
    valueBuf := make([]byte, binary.Size(fmt.MsgStruct))
    binary.LittleEndian.PutUint64(valueBuf, uint64(values[0].(uint64)))
    msgBuf = append(msgBuf, valueBuf...)
    return msgBuf
    return nil
}

func (reader *DFReaderBinary) makeFormatMsgbuf(fmt *DFFormat) []byte {
    /*fmtFmt, ok := reader.Formats[0x80]
    if !ok {
        return nil
    }
    ret := []byte{0xA3, 0x95, 0x80}
    name := fmt.Name
    format := fmt.Format
    columns := strings.Join(fmt.Columns, ",")
    values := []interface{}{
        fmt.Typ,
        uint8(binary.Size(fmt.MsgStruct) + 3),
        []byte(name),
        []byte(format),
        []byte(columns),
    }
    valueBuf := make([]byte, binary.Size(fmtFmt.MsgStruct))
    binary.LittleEndian.PutUint64(valueBuf, uint64(values[0].(uint64)))
    binary.LittleEndian.PutUint64(valueBuf[8:], uint64(values[1].(uint8)))
    copy(valueBuf[16:], values[2].([]byte))
    copy(valueBuf[80:], values[3].([]byte))
    copy(valueBuf[144:], values[4].([]byte))
    msgBuf := make([]byte, 0, len(ret)+len(valueBuf))
    msgBuf = append(msgBuf, ret...)
    msgBuf = append(msgBuf, valueBuf...)
    return msgBuf
    return nil
}*/

func (d *DFReaderBinary) addMsg(m *DFMessage) {
	msgType := m.GetType()
	d.Messages[msgType] = m
}
