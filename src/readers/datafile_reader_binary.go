package readers

import (
	"TelemetryParser/telemetry_parser/src/messages"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"syscall"
)

type DFReaderBinary struct {
    DFReader
    fileHandle *os.File
    dataMap    []byte
    HEAD1      byte
    HEAD2      byte
    unpackers  map[byte]func([]byte) ([]interface{}, error)
    formats    map[byte]*messages.DFFormat
	zeroTimeBase bool
    verbose    bool
	prevType     byte
	offset       int64
	remaining    int64
	offsets    [][]int64
	typeNums   []byte
	timestamp  int
	counts     []int
	_count     int
	nameToID   map[string]byte	
	idToName   map[byte]string
	//messages map[string]interface{}
}

func NewDFReaderBinary(filename string, zeroTimeBase bool, progressCallback func(int)) (*DFReaderBinary, error) {
    reader := &DFReaderBinary{
        DFReader: DFReader{
            clock:        nil,
        },
		HEAD1:        0xA3,
		HEAD2:        0x95,
		unpackers:    make(map[byte]func([]byte) ([]interface{}, error)),
        verbose:     false,
        offset:      0,
        remaining:  0,
        typeNums:   nil,
        formats: map[byte]*messages.DFFormat{
            0x80: &messages.DFFormat{
                Typ:    0x80,
                Name:    "FMT",
                Len:     89,
                Format:  "BBnNZ",
                Columns: []string{"Type", "Length", "Name", "Format", "Columns"},
            },
        },
		zeroTimeBase: zeroTimeBase,
		prevType:     0,
	}

    var err error
    reader.fileHandle, err = os.Open(filename)
	if err != nil {
		panic(err)
	}

    fileInfo, err := reader.fileHandle.Stat()
    if err != nil {
        return nil, err
    }

    reader.dataLen = fileInfo.Size()

    // Read the whole file into memory
	reader.dataMap, err = syscall.Mmap(int(reader.fileHandle.Fd()), 0, int(reader.dataLen), syscall.PROT_READ, syscall.MAP_PRIVATE)
    if err != nil {
        panic(err)
    }

    reader.init(progressCallback)
    return reader, nil
}

func (reader *DFReaderBinary) init(progressCallback func(int)) {
	// Implementation of init function
	reader.offset = 0
	reader.remaining = reader.dataLen
	reader.prevType = 0
	reader.initClock()
	reader.prevType = 0
	reader._rewind()
	reader.initArrays(progressCallback)
}

func (reader *DFReaderBinary) _rewind() {
    reader.offset = 0
    reader.remaining = reader.dataLen
    reader.typeNums = nil
    reader.timestamp = 0
}

func (reader *DFReaderBinary) Rewind() {
    reader._rewind()
}

func (reader *DFReaderBinary) initArrays(progressCallback func(int)) {
    reader.offsets = make([][]int64, 256)
    reader.counts = make([]int, 256)
    reader._count = 0
    reader.nameToID = make(map[string]byte)
    reader.idToName = make(map[byte]string)
    typeInstances := make(map[byte]map[string]struct{})

    for i := 0; i < 256; i++ {
		reader.offsets[i] = []int64{}
		reader.counts[i] = 0
	}

    fmtType := byte(0x80)
    fmtuType := byte(0)

    ofs := int64(0)
    pct := 0

    HEAD1 := reader.HEAD1
    HEAD2 := reader.HEAD2

    lengths := make([]int64, 256)
    for i := range lengths {
        lengths[i] = -1
    }

    for ofs+3 < reader.dataLen {
        hdr := reader.dataMap[ofs : ofs+3]
        if hdr[0] != HEAD1 || hdr[1] != HEAD2 {
            // avoid end of file garbage, 528 bytes has been use consistently throughout this implementation
            // but it needs to be at least 249 bytes which is the block based logging page size (256) less a 6 byte header and
            // one byte of data. Block based logs are sized in pages which means they can have up to 249 bytes of trailing space.
            if reader.dataLen-ofs >= 528 || reader.dataLen < 528 {
                fmt.Fprintf(os.Stderr, "bad header 0x%02x 0x%02x at %d\n", hdr[0], hdr[1], ofs)
            }
            ofs++
            continue
        }

        mtype := hdr[2]

        reader.offsets[mtype] = append(reader.offsets[mtype], ofs)

        if lengths[mtype] == -1 {
            if _, ok := reader.formats[mtype]; !ok {
                if reader.dataLen - ofs >= 528 || reader.dataLen < 528 {
                    fmt.Fprintf(os.Stderr, "unknown msg type 0x%02x (%d) at %d\n", mtype, mtype, ofs)
                }
                break
            }

            reader.offset = ofs
            reader.parseNext()

            dfmt, ok := reader.formats[mtype]
            if !ok {
                // Handle the case when the key is not found
                //fmt.Fprintf("Key %x not found in formats\n", mtype)
                continue
            }
            lengths[mtype] = dfmt.Len

        } else if reader.formats[mtype].InstanceField != nil {
            dfmt := reader.formats[mtype]
            idata := reader.dataMap[ofs+3+int64(dfmt.InstanceOfs) : ofs+3+int64(dfmt.InstanceOfs)+int64(dfmt.InstanceLen)]

            if _, ok := typeInstances[mtype]; !ok {
                typeInstances[mtype] = make(map[string]struct{})  
            }

            idataStr := string(idata)
            if _, ok := typeInstances[mtype][idataStr]; !ok {
                typeInstances[mtype][idataStr] = struct{}{}
                reader.offset = ofs
                reader.parseNext()
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

            ftype := byte(elements[0].(uint8))
            name := nullTerm(string(elements[2].([]byte)))
            length := int64(elements[1].(uint8))
            format := nullTerm(string(elements[3].([]byte)))
            columns := nullTerm(string(elements[4].([]byte)))

            mfmt, err := messages.NewDFFormat(ftype, name, length, format, columns, reader.formats[ftype])
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
            ftype := byte(elements[1].(uint8))
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

func nullTerm(s string) string {
    idx := strings.Index(s, "\x00")
    if idx != -1 {
        s = s[:idx]
    }
    return s
}

func (d *DFReaderBinary) recvMsg() messages.DFMessage {
	return *d.parseNext()
}

func (reader *DFReaderBinary) SkipToType(typeSet map[string]struct{}) {
    if reader.typeNums == nil {
        typeSet["MODE"] = struct{}{}
        typeSet["MSG"] = struct{}{}
        typeSet["PARM"] = struct{}{}
        typeSet["STAT"] = struct{}{}
        typeSet["ORGN"] = struct{}{}
        typeSet["VER"] = struct{}{}
        reader.indexes = make([]int, 0)
        reader.typeNums = make([]byte, 0)
        for t := range typeSet {
            if id, ok := reader.nameToID[t]; ok {
                reader.typeNums = append(reader.typeNums, id)
                reader.indexes = append(reader.indexes, 0)
            }
        }
    }
    smallestIndex := -1
    smallestOffset := reader.dataLen
    for i := range reader.typeNums {
        mtype := reader.typeNums[i]
        if reader.indexes[i] >= reader.counts[mtype] {
            continue
        }
        ofs := reader.offsets[mtype][reader.indexes[i]]
        if ofs < smallestOffset {
            smallestOffset = ofs
            smallestIndex = i
        }
    }
    if smallestIndex >= 0 {
        reader.indexes[smallestIndex]++
        reader.offset = smallestOffset
    }
}

func (reader *DFReaderBinary) parseNext() *messages.DFMessage {
    skipType := [3]byte{}
    skipStart := int64(0)
    var msgType byte
    for {
        if reader.dataLen-reader.offset < 3 {
            return nil
        }

        hdr := reader.dataMap[reader.offset : reader.offset+3]
        if hdr[0] == reader.HEAD1 && hdr[1] == reader.HEAD2 {
            if skipType != [3]byte{} {
                if reader.remaining >= 528 {
                    skipBytes := reader.offset - skipStart
                    fmt.Fprintf(os.Stderr, "Skipped %d bad bytes in log at offset %d, type=%v (prev=%d)\n", skipBytes, skipStart, skipType, reader.prevType)
                }
                skipType = [3]byte{}
            }
            msgType := hdr[2]
            if _, ok := reader.formats[msgType]; ok {
                reader.prevType = msgType
                break
            }
            if skipType == [3]byte{} {
                skipType = [3]byte{hdr[0], hdr[1], hdr[2]}
                skipStart = reader.offset
            }
            reader.offset++
            reader.remaining--
            continue
        }
    }
        
    reader.offset += 3
    reader.remaining = reader.dataLen - reader.offset

    dfmt, ok := reader.formats[msgType]
    if !ok {
        return reader.parseNext()
    }

    if reader.remaining < dfmt.Len-3 {
        if reader.verbose {
            fmt.Println("out of data")
        }
        return nil
    }
        
    body := reader.dataMap[reader.offset : reader.offset+dfmt.Len-3]
        
    elements, err := reader.unpackers[msgType](body)
    if err != nil {
        fmt.Println(err)
        if reader.remaining < 528 {
            return nil
        }
        fmt.Fprintf(os.Stderr, "Failed to parse %s/%s with len %d (remaining %d)\n", dfmt.Name, dfmt.MsgStruct, len(body), reader.remaining)
    }
        
    if elements == nil {
        return reader.parseNext()
    }
        
    name := dfmt.Name
    for _, aIndex := range dfmt.AIndexes {
        if aIndex < len(elements) {
            data, ok := elements[aIndex].([]byte)
            if ok {
                elements[aIndex] = bytesToInt16Array(data)
            } else {
                fmt.Fprintf(os.Stderr, "Failed to transform array: %v\n", elements[aIndex])
            }
        }
    }

    if name == "FMT" {
        //ftype := byte(elements[0].(uint8))
        ftype, ok := elements[0].(byte)
        if !ok {
            return reader.parseNext()
        }
        
        name := nullTerm(string(elements[2].([]byte)))
        length := int64(elements[1].(uint8))
        format := nullTerm(string(elements[3].([]byte)))
        columns := nullTerm(string(elements[4].([]byte)))

        mfmt, err := messages.NewDFFormat(ftype, name, length, format, columns, reader.formats[ftype])
        if err != nil {
            return reader.parseNext()
        }

        reader.formats[ftype] = mfmt
    }

    reader.offset += int64(dfmt.Len) - 3
    reader.remaining = reader.dataLen - reader.offset
    m := messages.NewDFMessage(dfmt, elements, true, reader)

    if m.Fmt.Name == "FMTU" {
        FmtType := int(elements[0].(uint8))
        UnitIds := elements[1].(string)
        MultIds := elements[2].(string)
        if fmt, ok := reader.formats[byte(FmtType)]; ok {
            fmt.SetUnitIds(&UnitIds)
            fmt.SetMultIds(&MultIds)
        }
    }

    reader.addMsg(m)
   
    reader.percent = 100.0 * float64(reader.offset) / float64(reader.dataLen)

    return m
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

func (reader *DFReaderBinary) FindUnusedFormat() byte {
    for i := 254; i > 1; i-- {
        if _, ok := reader.formats[byte(i)]; !ok {
            return byte(i)
        }
    }
    return 0
}

func (reader *DFReaderBinary) AddFormat(fmt *messages.DFFormat) *messages.DFFormat {
    newType := reader.FindUnusedFormat()
    if newType == 0 {
        return nil
    }
    fmt.Typ = newType
    reader.formats[newType] = fmt
    return fmt
}

func (d *DFReaderBinary) MakeMsgbuf(fmt *messages.DFFormat, values []interface{}) []byte {
    /*ret := []byte{0xA3, 0x95, fmt.Typ}
    msgBuf := make([]byte, 0, len(ret)+binary.Size(fmt.MsgStruct))
    msgBuf = append(msgBuf, ret...)
    valueBuf := make([]byte, binary.Size(fmt.MsgStruct))
    binary.LittleEndian.PutUint64(valueBuf, uint64(values[0].(uint64)))
    msgBuf = append(msgBuf, valueBuf...)
    return msgBuf*/
    return nil
}

func (reader *DFReaderBinary) makeFormatMsgbuf(fmt *messages.DFFormat) []byte {
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
    return msgBuf*/
    return nil
}

func (d *DFReaderBinary) addMsg(m *messages.DFMessage) {
    msgType := m.GetType()
    d.messages[msgType] = m
}