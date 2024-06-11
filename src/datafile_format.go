package src

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
)

var FormatToUnpackInfo = map[byte][3]interface{}{
	'a': {"64s", nil, string("")},
	'b': {"b", nil, int(0)},
	'B': {"B", nil, int(0)},
	'c': {"h", 0.01, float64(0)},
	'C': {"H", 0.01, float64(0)},
	'd': {"d", nil, float64(0)},
	'e': {"i", 0.01, float64(0)},
	'E': {"I", 0.01, float64(0)},
	'f': {"f", nil, float32(0)},
	'h': {"h", nil, int16(0)},
	'H': {"H", nil, uint16(0)},
	'i': {"i", nil, int32(0)},
	'I': {"I", nil, uint32(0)},
	'L': {"i", 1.0e-7, float64(0)},
	'M': {"b", nil, int(0)},
	'n': {"4s", nil, string("")},
	'N': {"16s", nil, string("")},
	'q': {"q", nil, int64(0)}, 
	'Q': {"Q", nil, int64(0)}, 
	'Z': {"64s", nil, string("")},
}

func u_ord(c byte) byte {
	if c == 0 {
		return 0
	}
	return c
}

type DataFileFormat struct {
	Typ           int
	Name          string
	Len           int
	Format        string
	Columns       []string
	InstanceField *string
	UnitIds       *string
	MultIds       *string
	MsgStruct     string
	MsgTypes      []interface{}
	MsgMults      []interface{}
	MsgFmts       []string
	Colhash       map[string]int
	AIndexes      []int
	InstanceOfs   int
	InstanceLen   int
}

func NewDataFileFormat(typ int, name string, flen int, format string, columns []string, oldfmt *DataFileFormat) (*DataFileFormat, error) {
	df := &DataFileFormat{
		Typ:     typ,
		Name:    nullTerm(name),
		Len:     flen,
		Format:  format,
		Columns: columns,
	}

	msgStruct := "<"
	msgMults := []interface{}{}
	msgTypes := []interface{}{}
	msgFmts := []string{}

	for _, c := range format {
		if u_ord(byte(c)) == 0 {
			break
		}
		msgFmts = append(msgFmts, string(c))
		if val, ok := FormatToUnpackInfo[byte(c)]; ok {
			msgStruct += val[0].(string)
			df.MsgMults = append(msgMults, val[1])
			if c == 'a' {
				msgTypes = append(msgTypes, binary.BigEndian)
			} else {
				msgTypes = append(msgTypes, val[2])
			}
		} else {
			panic(fmt.Sprintf("DFFormat: Unsupported format char: '%c' in message %s", c, name))
		}
	}
	df.MsgStruct = msgStruct
	df.MsgTypes = msgTypes
	df.MsgMults = msgMults
	df.MsgFmts = msgFmts

	df.Colhash = make(map[string]int)
	for i, column := range columns {
		df.Colhash[column] = i
	}

	df.AIndexes = []int{}
	for i, msgFmt := range df.MsgFmts {
		if msgFmt == "a" {
			df.AIndexes = append(df.AIndexes, i)
		}
	}

	//if oldfmt != nil {
	//	df.SetUnitIds(oldfmt.UnitIds)
	//	df.SetMultIds(oldfmt.MultIds)
	//}

	return df, nil
}

func (df *DataFileFormat) getUnpacker() func([]byte) ([]interface{}, error) {
	return func(data []byte) ([]interface{}, error) {
		if len(data) < df.Len-3 {
			return nil, fmt.Errorf("insufficient data for message type %d", df.Typ)
		}

		elements := make([]interface{}, 0)
		reader := bytes.NewReader(data)

		for i := 1; i < len(df.MsgStruct); i++ {
			switch df.MsgStruct[i] {
			case 'a', 'Z':
				var str [64]byte
				if err := binary.Read(reader, binary.LittleEndian, &str); err != nil {
					return nil, err
				}
				elements = append(elements, str[:])
			case 'b', 'M':
				var val int8
				if err := binary.Read(reader, binary.LittleEndian, &val); err != nil {
					return nil, err
				}
				elements = append(elements, int(val))
			case 'B':
				var val uint8
				if err := binary.Read(reader, binary.LittleEndian, &val); err != nil {
					return nil, err
				}
				elements = append(elements, int(val))
			case 'c':
				var val int16
				if err := binary.Read(reader, binary.LittleEndian, &val); err != nil {
					return nil, err
				}
				elements = append(elements, float64(val)*0.01)
			case 'C':
				var val uint16
				if err := binary.Read(reader, binary.LittleEndian, &val); err != nil {
					return nil, err
				}
				elements = append(elements, float64(val)*0.01)
			case 'd':
				var val float64
				if err := binary.Read(reader, binary.LittleEndian, &val); err != nil {
					return nil, err
				}
				elements = append(elements, val)
			case 'e':
				var val int32
				if err := binary.Read(reader, binary.LittleEndian, &val); err != nil {
					return nil, err
				}
				elements = append(elements, float64(val)*0.01)
			case 'E':
				var val uint32
				if err := binary.Read(reader, binary.LittleEndian, &val); err != nil {
					return nil, err
				}
				elements = append(elements, float64(val)*0.01)
			case 'f':
				var val float32
				if err := binary.Read(reader, binary.LittleEndian, &val); err != nil {
					return nil, err
				}
				elements = append(elements, float64(val))
			case 'h':
				var val int16
				if err := binary.Read(reader, binary.LittleEndian, &val); err != nil {
					return nil, err
				}
				elements = append(elements, int(val))
			case 'H':
				var val uint16
				if err := binary.Read(reader, binary.LittleEndian, &val); err != nil {
					return nil, err
				}
				elements = append(elements, int(val))
			case 'i', 'L':
				var val int32
				if err := binary.Read(reader, binary.LittleEndian, &val); err != nil {
					return nil, err
				}
				elements = append(elements, int(val))
			case 'I':
				var val uint32
				if err := binary.Read(reader, binary.LittleEndian, &val); err != nil {
					return nil, err
				}
				elements = append(elements, int(val))
			case 'n':
				var str [4]byte
				if err := binary.Read(reader, binary.LittleEndian, &str); err != nil {
					return nil, err
				}
				elements = append(elements, str[:])
			case 'N':
				var str [16]byte
				if err := binary.Read(reader, binary.LittleEndian, &str); err != nil {
					return nil, err
				}
				elements = append(elements, str[:])
			case 'q', 'Q':
				var val int64
				if err := binary.Read(reader, binary.LittleEndian, &val); err != nil {
					return nil, err
				}
				elements = append(elements, val)
			case 's':
				var strSize int
				switch df.MsgStruct[i+1] {
				case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
					strSize = int(df.MsgStruct[i+1] - '0')
					i++
				default:
					strSize = 64 // Default size if not specified
				}

				var strBytes [64]byte // Assuming maximum length of 64 bytes for the string
				if err := binary.Read(reader, binary.LittleEndian, &strBytes); err != nil {
					return nil, err
				}
				str := string(strBytes[:strSize])
				elements = append(elements, str)
			case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
				// Skip the digits
			case '<':
				// Ignore the '<' character
			default:
				return nil, fmt.Errorf("unsupported format character: %c", df.MsgStruct[i])
			}
		}

		return elements, nil
	}
}

func (df *DataFileFormat) SetUnitIds(unit_ids *string) {
	if unit_ids == nil {
		return
	}

	df.UnitIds = unit_ids
	instance_idx := strings.Index(*unit_ids, "#")
	if instance_idx != -1 {
		df.InstanceField = &df.Columns[instance_idx]
		pre_fmt := df.Format[:instance_idx]
		pre_sfmt := ""
		for _, c := range pre_fmt {
			pre_sfmt += FormatToUnpackInfo[byte(c)][0].(string)
		}
		df.InstanceOfs = binary.Size(pre_sfmt)
		ifmt := df.Format[instance_idx]
		df.InstanceLen = binary.Size(ifmt)
	}
}

func (df *DataFileFormat) SetMultIds(mult_ids *string) {
	df.MultIds = mult_ids
}

func (df *DataFileFormat) String() string {
	return fmt.Sprintf("DFFormat(%s,%s,%s,%s)", string(df.Typ), df.Name, df.Format, df.Columns)
}
