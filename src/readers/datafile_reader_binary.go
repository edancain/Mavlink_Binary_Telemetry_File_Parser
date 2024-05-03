package readers

import (
	"fmt"
	"os"
	"syscall"
	"telemetry_parser/src/messages"
)

type DFReaderBinary struct {
    fileHandle *os.File
    dataLen    int
    dataMap    []byte
    HEAD1      byte
    HEAD2      byte
    unpackers  map[byte]func([]byte) ([]interface{}, error)
    formats    map[byte]*messages.DFFormat
	zeroTimeBase bool
	prevType     byte
	offset       int
	remaining    int
	offsets    [][]int
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
		HEAD1:        0xA3,
		HEAD2:        0x95,
		unpackers:    make(map[byte]func([]byte) ([]interface{}, error)),
		formats:      make(map[byte]*messages.DFFormat),
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

    reader.dataLen = int(fileInfo.Size())

    // Read the whole file into memory
	reader.dataMap, err = syscall.Mmap(int(reader.fileHandle.Fd()), 0, reader.dataLen, syscall.PROT_READ, syscall.MAP_PRIVATE)
    if err != nil {
        panic(err)
    }
    /*if platform == "windows" {
        err = syscall.UnmapViewOfFile(uintptr(unsafe.Pointer(&reader.dataMap[0])))
        if err != nil {
            panic(err)
        }
    }*/
    reader.init(progressCallback)
    return reader, nil
}

func (reader *DFReaderBinary) init(progressCallback func(int)) {
	// Implementation of init function
	reader.offset = 0
	reader.remaining = reader.dataLen
	reader.prevType = 0
	//reader.initClock()
	reader.prevType = 0
	reader.Rewind()
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
    reader.offsets = make([][]int, 256)
    reader.counts = make([]int, 256)
    reader._count = 0
    reader.nameToID = make(map[string]byte)
    reader.idToName = make(map[byte]string)
    typeInstances := make(map[byte]map[string]bool)

    for i := 0; i < 256; i++ {
		reader.offsets[i] = []int{}
		reader.counts[i] = 0
	}

    fmtType := 0x80
    fmtuType := byte(0)

    ofs := 0
    pct := 0

    HEAD1 := reader.HEAD1
    HEAD2 := reader.HEAD2

    lengths := make([]int, 256)
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

        if reader.formats[mtype] == nil {
            if _, ok := reader.formats[mtype]; !ok {
                if reader.dataLen-ofs >= 528 || reader.dataLen < 528 {
                    fmt.Fprintf(os.Stderr, "unknown msg type 0x%02x (%d) at %d\n", mtype, mtype, ofs)
                }
                break
            }

            reader.offset = ofs

            reader.parseNext()
            fmt := reader.formats[mtype]
            lengths[mtype] = fmt.len
        } else if fmt.instanceField != nil {
            fmt := reader.formats[mtype]
            idata := reader.dataMap[ofs+3+fmt.instanceOfs : ofs+3+fmt.instanceOfs+fmt.instanceLen]
            if _, ok := typeInstances[mtype]; !ok {
                typeInstances[mtype] = make(map[string]struct{})
            }
            if _, ok := typeInstances[mtype][idata]; !ok {
                typeInstances[mtype][idata] = struct{}{}
                reader.offset = ofs
                parseNext()
            }
        }

        reader.counts[mtype]++
        mlen := lengths[mtype]

        if mtype == fmtType {
            body := reader.dataMap[ofs+3 : ofs+mlen]
            if len(body)+3 < mlen {
                break
            }
            fmt := reader.formats[mtype]
            elements := structUnpack(fmt.msgStruct, body)
            ftype := elements[0]
            mfmt := DFFormat{
                ftype:     ftype,
                name:      nullTerm(elements[2]),
                len:       elements[1],
                format:    nullTerm(elements[3]),
                columns:   nullTerm(elements[4]),
                oldFormat: reader.formats[ftype],
            }
            reader.formats[ftype] = mfmt
            reader.nameToID[mfmt.name] = mfmt.ftype
            reader.idToName[mfmt.ftype] = mfmt.name
            if mfmt.name == "FMTU" {
                fmtuType = &mfmt.ftype
            }
        }

        if fmtuType != nil && mtype == *fmtuType {
            fmt := reader.formats[mtype]
            body := reader.dataMap[ofs+3 : ofs+mlen]
            if len(body)+3 < mlen {
                break
            }
            elements := structUnpack(fmt.msgStruct, body)
            ftype := int(elements[1])
            if fmt2, ok := reader.formats[ftype]; ok {
                if _, ok := fmt.colHash["UnitIds"]; ok {
                    fmt2.setUnitIDs(nullTerm(elements[fmt.colHash["UnitIds"]]))
                }
                if _, ok := fmt.colHash["MultIds"]; ok {
                    fmt2.setMultIDs(nullTerm(elements[fmt.colHash["MultIds"]]))
                }
            }
        }

        ofs += mlen
        if progressCallback != nil {
            newPct := (100 * ofs) / d.dataLen
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

func (d *DFReaderBinary) last_timestamp() (int, error) {
    highest_offset := 0
    second_highest_offset := 0
    for i := 0; i < 256; i++ {
        if d.counts[i] == -1 {
            continue
        }
        if len(d.offsets[i]) == 0 {
            continue
        }
        ofs := d.offsets[i][len(d.offsets[i])-1]
        if ofs > highest_offset {
            second_highest_offset = highest_offset
            highest_offset = ofs
        } else if ofs > second_highest_offset {
            second_highest_offset = ofs
        }
    }
    d.offset = highest_offset
    m, err := d.recv_msg()
    if err != nil {
        return 0, err
    }
    if m == nil {
        d.offset = second_highest_offset
        m, err = d.recv_msg()
        if err != nil {
            return 0, err
        }
    }
    return m._timestamp, nil
}

func (d *DFReaderBinary) skipToType(typeSet map[string]bool) {
    if d.typeNums == nil {
        typeSet["MODE"] = true
        typeSet["MSG"] = true
        typeSet["PARM"] = true
        typeSet["STAT"] = true
        typeSet["ORGN"] = true
        typeSet["VER"] = true
        d.indexes = []int{}
        d.typeNums = []int{}
        for t := range typeSet {
            id, ok := d.nameToID[t]
            if !ok {
                continue
            }
            d.typeNums = append(d.typeNums, id)
            d.indexes = append(d.indexes, 0)
        }
    }
    smallestIndex := -1
    smallestOffset := d.dataLen
    for i := range d.typeNums {
        mtype := d.typeNums[i]
        if d.indexes[i] >= d.counts[mtype] {
            continue
        }
        ofs := d.offsets[mtype][d.indexes[i]]
        if ofs < smallestOffset {
            smallestOffset = ofs
            smallestIndex = i
        }
    }
    if smallestIndex >= 0 {
        d.indexes[smallestIndex]++
        d.offset = smallestOffset
    }
}

func (reader *DFReaderBinary) parseNext() *messages.DFMessage {
    var skipType []byte
    skipStart := 0

    for {
        if reader.dataLen - reader.offset < 3 {
            return nil
        }

        hdr := reader.dataMap[reader.offset : reader.offset+3]
        if hdr[0] == reader.HEAD1 && hdr[1] == reader.HEAD2 {
            if skipType != nil {
                if reader.remaining >= 528 {
                    skipBytes := reader.offset - skipStart
                    fmt.Printf("Skipped %d bad bytes in log at offset %d, type=%v (prev=%d)\n",
                        skipBytes, skipStart, skipType, d.prevType)
                
                skipType = nil
                }
                msgType := hdr[2]
                if _, ok := reader.formats[msgType]; ok {
                    reader.prevType = msgType
                    break
                }
            }else {
				skipType = []byte{hdr[0], hdr[1], hdr[2]}
				skipStart = reader.offset
			}
		 }else {
			if skipType == nil {
				skipType = []byte{hdr[0], hdr[1], hdr[2]}
				skipStart = reader.offset
			}
		}
		reader.offset++
		reader.remaining--
    }

    reader.offset += 3
    reader.remaining = reader.dataLen - reader.offset

    fmt := reader.formats[reader.prevType]
	if reader.remaining < 0 { //len(fmt) {
		//if reader.Verbose {
		//	fmt.Fprintf(os.Stderr, "out of data\n")
		//}
		return nil
	}

    body := reader.dataMap[reader.offset : reader.offset] // + fmt.len-3]
    var elements []interface{}
    var err error

    elements, err = reader.unpackers[reader.prevType].Unpack(body)

	if _, ok := reader.unpackers[reader.prevType]; !ok {
		//reader.unpackers[reader.prevType] = struct(fmt.msgStruct).Unpack
	}

	elements, err = reader.unpackers[reader.prevType].Unpack(body)

	if err != nil {
		if reader.remaining < 528 {
			return nil
		}
		//fmt.printf(os.Stderr, "Failed to parse %s/%s with len %d (remaining %d)\n",
		//	fmt.name, fmt.msgStruct, len(body), reader.remaining)
		return reader.parseNext()
	}

    name := fmt.name
	for _, aIndex := range fmt.aIndexes {
		val := elements[aIndex].([]byte)
		arr := array.NewIntSlice(val)
		elements[aIndex] = arr
	}

    if name == "FMT" {
        // add to formats
        // TODO: handle FMT case
    }
    reader.offset += fmt.len - 3
    reader.remaining = reader.dataLen - reader.offset
    m := messages.DFMessage{fmt, elements}
    if m.Fmt.Name == "FMTU" {
        // add to units information
        // TODO: handle FMTU case
    }
    // TODO: handle _add_msg(m)
    reader.percent = 100.0 * (float64(reader.offset) / float64(reader.dataLen))
    return &m
}

func (d *DFReader) findUnusedFormat() int {
    for i := 254; i > 1; i-- {
        if _, ok := d.formats[i]; !ok {
            return i
        }
    }
    return 0
}

func (d *DFReader) addFormat(fmt *messages.DFFormat) *messages.DFFormat {
    newType := d.findUnusedFormat()
    if newType == 0 {
        return nil
    }
    fmt.Type = newType
    d.formats[newType] = fmt
    return &fmt
}

func (d *DFReader) makeMsgbuf(fmt *messages.DFFormat, values []interface{}) []byte {
    buf := new(bytes.Buffer)
    binary.Write(buf, binary.LittleEndian, []byte{0xA3, 0x95, byte(fmt.Type)})
    for _, v := range values {
        binary.Write(buf, binary.LittleEndian, v)
    }
    return buf.Bytes()
}

func (d *DFReader) makeFormatMsgbuf(fmt *messages.DFFormat) []byte {
    fmtFmt := d.formats[0x80]
    buf := new(bytes.Buffer)
    binary.Write(buf, binary.LittleEndian, []byte{0xA3, 0x95, 0x80})
    binary.Write(buf, binary.LittleEndian, []interface{}{
        fmt.Type,
        len(fmt.MsgStruct) + 3,
        fmt.Name,
        fmt.Format,
        strings.Join(fmt.Columns, ","),
    })
    return buf.Bytes()
}