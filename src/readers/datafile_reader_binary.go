package readers

import (
	"fmt"
	"os"
	"github.com/edsrzf/mmap-go"
    "telemetry_parser/src/messages"
)

type DFReaderBinary struct {
    filehandle *os.File
    dataLen    int
    dataMap    mmap.MMap
    HEAD1      byte
    HEAD2      byte
    unpackers  map[string]interface{}
    formats    map[int]messages.DFFormat
	offset	   int
	remaining  int
	offsets    [][]int
	typeNums   []int
	timestamp  int
	counts     []int
	_count     int
	nameToID   map[string]int	
	idToName   map[int]string
	//messages map[string]interface{}
}

func NewDFReaderBinary(filename string, zeroTimeBase bool, progressCallback func(int)) (*DFReaderBinary, error) {
    reader := &DFReaderBinary{}
    file, err := os.Open(filename)
	if err != nil {
        return nil, err
    }
    reader.filehandle = file

    fileInfo, err := file.Stat()
    if err != nil {
        return nil, err
    }
    reader.dataLen = int(fileInfo.Size())

    dataMap, err := mmap.Map(file, mmap.RDONLY, 0)
    if err != nil {
        return nil, err
    }
    reader.dataMap = dataMap

    reader.HEAD1 = 0xA3
    reader.HEAD2 = 0x95
    reader.unpackers = make(map[string]interface{})
    reader.formats = make(map[int]messages.DFFormat)
    // initialize other fields

    // call other initialization methods
    // reader.initClock()
    // reader._rewind()
    // reader.initArrays(progressCallback)

    return reader, nil
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

func (d *DFReaderBinary) initArrays(progressCallback func(int)) {
    d.offsets = make([][]int, 256)
    d.counts = make([]int, 256)
    d._count = 0
    d.nameToID = make(map[string]int)
    d.idToName = make(map[int]string)
    typeInstances := make(map[int]map[string]struct{})
    fmtType := 0x80
    var fmtuType *int
    ofs := 0
    pct := 0
    HEAD1 := d.HEAD1
    HEAD2 := d.HEAD2
    lengths := make([]int, 256)
    for i := range lengths {
        lengths[i] = -1
    }

    for ofs+3 < d.dataLen {
        hdr := d.dataMap[ofs : ofs+3]
        if hdr[0] != HEAD1 || hdr[1] != HEAD2 {
            if d.dataLen-ofs >= 528 || d.dataLen < 528 {
                fmt.Fprintf(os.Stderr, "bad header 0x%02x 0x%02x at %d\n", hdr[0], hdr[1], ofs)
            }
            ofs++
            continue
        }
        mtype := int(hdr[2])
        d.offsets[mtype] = append(d.offsets[mtype], ofs)

        if lengths[mtype] == -1 {
            if _, ok := d.formats[mtype]; !ok {
                if d.dataLen-ofs >= 528 || d.dataLen < 528 {
                    fmt.Fprintf(os.Stderr, "unknown msg type 0x%02x (%d) at %d\n", mtype, mtype, ofs)
                }
                break
            }
            d.offset = ofs
            d._parseNext()
            fmt := d.formats[mtype]
            lengths[mtype] = fmt.len
        } else if fmt.instanceField != nil {
            fmt := d.formats[mtype]
            idata := d.dataMap[ofs+3+fmt.instanceOfs : ofs+3+fmt.instanceOfs+fmt.instanceLen]
            if _, ok := typeInstances[mtype]; !ok {
                typeInstances[mtype] = make(map[string]struct{})
            }
            if _, ok := typeInstances[mtype][idata]; !ok {
                typeInstances[mtype][idata] = struct{}{}
                d.offset = ofs
                parseNext()
            }
        }

        d.counts[mtype]++
        mlen := lengths[mtype]

        if mtype == fmtType {
            body := d.dataMap[ofs+3 : ofs+mlen]
            if len(body)+3 < mlen {
                break
            }
            fmt := d.formats[mtype]
            elements := structUnpack(fmt.msgStruct, body)
            ftype := elements[0]
            mfmt := DFFormat{
                ftype:     ftype,
                name:      nullTerm(elements[2]),
                len:       elements[1],
                format:    nullTerm(elements[3]),
                columns:   nullTerm(elements[4]),
                oldFormat: d.formats[ftype],
            }
            d.formats[ftype] = mfmt
            d.nameToID[mfmt.name] = mfmt.ftype
            d.idToName[mfmt.ftype] = mfmt.name
            if mfmt.name == "FMTU" {
                fmtuType = &mfmt.ftype
            }
        }

        if fmtuType != nil && mtype == *fmtuType {
            fmt := d.formats[mtype]
            body := d.dataMap[ofs+3 : ofs+mlen]
            if len(body)+3 < mlen {
                break
            }
            elements := structUnpack(fmt.msgStruct, body)
            ftype := int(elements[1])
            if fmt2, ok := d.formats[ftype]; ok {
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

    for _, count := range d.counts {
        d._count += count
    }
    d.offset = 0
}

func (d *DFReader) last_timestamp() (int, error) {
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

func (d *DFReader) skipToType(typeSet map[string]bool) {
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

func (d *DFReader) parseNext() *DFMessage {
    var skipType []byte
    skipStart := 0
    for {
        if d.dataLen-d.offset < 3 {
            return nil
        }
        hdr := d.dataMap[d.offset : d.offset+3]
        if hdr[0] == d.HEAD1 && hdr[1] == d.HEAD2 {
            if skipType != nil {
                if d.remaining >= 528 {
                    skipBytes := d.offset - skipStart
                    fmt.Printf("Skipped %d bad bytes in log at offset %d, type=%v (prev=%d)\n",
                        skipBytes, skipStart, skipType, d.prevType)
                }
                skipType = nil
            }
            msgType := int(hdr[2])
            if _, ok := d.formats[msgType]; ok {
                d.prevType = msgType
                break
            }
        }
        if skipType == nil {
            skipType = []byte{hdr[0], hdr[1], hdr[2]}
            skipStart = d.offset
        }
        d.offset++
        d.remaining--
    }
    d.offset += 3
    d.remaining = d.dataLen - d.offset
    fmt := d.formats[d.prevType]
    if d.remaining < fmt.len-3 {
        if d.verbose {
            fmt.Println("out of data")
        }
        return nil
    }
    body := d.dataMap[d.offset : d.offset+fmt.len-3]
    var elements []interface{}
    if unpacker, ok := d.unpackers[d.prevType]; ok {
        elements = unpacker(body)
    }
    if elements == nil {
        return d.parseNext()
    }
    name := fmt.name
    for _, aIndex := range fmt.aIndexes {
        // transform elements which can't be done at unpack time:
        // TODO: handle array transformation
    }
    if name == "FMT" {
        // add to formats
        // TODO: handle FMT case
    }
    d.offset += fmt.len - 3
    d.remaining = d.dataLen - d.offset
    m := DFMessage{fmt, elements}
    if m.fmt.name == "FMTU" {
        // add to units information
        // TODO: handle FMTU case
    }
    // TODO: handle _add_msg(m)
    d.percent = 100.0 * (float64(d.offset) / float64(d.dataLen))
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

func (d *DFReader) addFormat(fmt DFFormat) *DFFormat {
    newType := d.findUnusedFormat()
    if newType == 0 {
        return nil
    }
    fmt.Type = newType
    d.formats[newType] = fmt
    return &fmt
}

func (d *DFReader) makeMsgbuf(fmt DFFormat, values []interface{}) []byte {
    buf := new(bytes.Buffer)
    binary.Write(buf, binary.LittleEndian, []byte{0xA3, 0x95, byte(fmt.Type)})
    for _, v := range values {
        binary.Write(buf, binary.LittleEndian, v)
    }
    return buf.Bytes()
}

func (d *DFReader) makeFormatMsgbuf(fmt DFFormat) []byte {
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