package fileparser

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

const (
	MetricMultiplier        = 0.01
	defaultStringSize       = 64
	alternativeStringSize4  = 4
	alternativeStringSize16 = 16
)

var (
	ErrSkipDigits = errors.New("skip digits")
	ErrIgnoreChar = errors.New("ignore character")
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
			strVal, _ := val[0].(string)
			msgStruct += strVal
			msgMults = append(msgMults, val[1])
			if c == 'a' {
				msgTypes = append(msgTypes, binary.BigEndian)
			} else {
				msgTypes = append(msgTypes, val[2])
			}
		} else {
			return nil, fmt.Errorf("DFFormat: Unsupported format char: '%c' in message %s", c, name)
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

	// if oldfmt != nil {
	//	df.SetUnitIds(oldfmt.UnitIds)
	//	df.SetMultIds(oldfmt.MultIds)
	// }

	return df, nil
}

// getUnpacker returns a function that unpacks a byte slice into a slice of interfaces.
func (df *DataFileFormat) getUnpacker() func([]byte) ([]interface{}, error) {
	return func(data []byte) ([]interface{}, error) {
		if len(data) < df.Len-3 {
			return nil, fmt.Errorf("insufficient data for message type %d", df.Typ)
		}

		elements := make([]interface{}, 0)
		reader := bytes.NewReader(data)

		for i := 1; i < len(df.MsgStruct); i++ {
			elem, err := unpackElement(reader, df.MsgStruct, &i)
			if err != nil {
				return nil, err
			}
			if elem != nil {
				elements = append(elements, elem)
			}
		}

		return elements, nil
	}
}

// unpackElement handles unpacking a single element based on the format character.
func unpackElement(reader *bytes.Reader, format string, i *int) (interface{}, error) {
	switch format[*i] {
	case 'a', 'Z':
		return readFixedSizeString(reader, defaultStringSize)
	case 'b', 'M':
		return readInt8(reader)
	case 'B':
		return readUint8(reader)
	case 'c', 'C', 'e', 'E':
		return unpackMetricElement(reader, format[*i])
	case 'd', 'f':
		return unpackFloatElement(reader, format[*i])
	case 'h', 'H', 'i', 'I', 'q', 'Q':
		return unpackIntElement(reader, format[*i])
	case 'n':
		return readFixedSizeString(reader, alternativeStringSize4)
	case 'N':
		return readFixedSizeString(reader, alternativeStringSize16)
	case 's':
		return handleStringCase(reader, format, i)
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return nil, ErrSkipDigits // Skip the digits
	case '<':
		return nil, ErrIgnoreChar // Ignore the '<' character
	default:
		return nil, fmt.Errorf("unsupported format character: %c", format[*i])
	}
}

// unpackMetricElement handles unpacking elements that need to be multiplied by a metric multiplier.
func unpackMetricElement(reader *bytes.Reader, formatChar byte) (interface{}, error) {
	var err error
	var val interface{}

	switch formatChar {
	case 'c':
		val, err = readInt16(reader)
		if err == nil {
			val = float64(val.(int16)) * MetricMultiplier
		}
	case 'C':
		val, err = readUint16(reader)
		if err == nil {
			val = float64(val.(uint16)) * MetricMultiplier
		}
	case 'e':
		val, err = readInt32(reader)
		if err == nil {
			val = float64(val.(int32)) * MetricMultiplier
		}
	case 'E':
		val, err = readUint32(reader)
		if err == nil {
			val = float64(val.(uint32)) * MetricMultiplier
		}
	default:
		return nil, fmt.Errorf("unsupported metric format character: %c", formatChar)
	}

	if err != nil {
		return nil, err
	}

	return val, nil
}

// unpackFloatElement handles unpacking float elements.
func unpackFloatElement(reader *bytes.Reader, formatChar byte) (interface{}, error) {
	switch formatChar {
	case 'd':
		return readFloat64(reader)
	case 'f':
		return readFloat32(reader)
	default:
		return nil, fmt.Errorf("unsupported float format character: %c", formatChar)
	}
}

// unpackIntElement handles unpacking integer elements.
func unpackIntElement(reader *bytes.Reader, formatChar byte) (interface{}, error) {
	switch formatChar {
	case 'h':
		return readInt16(reader)
	case 'H':
		return readUint16(reader)
	case 'i', 'L':
		return readInt32(reader)
	case 'I':
		return readUint32(reader)
	case 'q', 'Q':
		return readInt64(reader)
	default:
		return nil, fmt.Errorf("unsupported int format character: %c", formatChar)
	}
}

// handleStringCase handles parsing string sizes and reading fixed-size strings.
func handleStringCase(reader *bytes.Reader, format string, i *int) (string, error) {
	strSize := parseMsgStructStrSize(format, i)

	return readFixedSizeString(reader, strSize)
}

// parseMsgStructStringSize parses the size of the string in the message structure.
func parseMsgStructStrSize(format string, i *int) int {
	size := int(format[*i] - '0')

	if *i+1 < len(format) && format[*i+1] >= '0' && format[*i+1] <= '9' {
		size = size*base + int(format[*i+1]-'0')
		*i++
	}

	return size
}

/*
func parseMsgStructStringSize(msgStruct string, i *int) (int, error) {
	if *i+1 >= len(msgStruct) {
		return 0, fmt.Errorf("index out of bounds in msgStruct string")
	}

	var strSize int
	switch msgStruct[*i+1] {
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		strSize = int(msgStruct[*i+1] - '0')
		*i++
	default:
		strSize = 64 // Default size if not specified
	}
	return strSize, nil
} */

// Helper functions to read specific types from the reader.
func readFixedSizeString(reader *bytes.Reader, size int) (string, error) {
	str := make([]byte, size)
	if err := binary.Read(reader, binary.LittleEndian, &str); err != nil {
		return "", err
	}
	return string(str), nil
}

func readInt8(reader *bytes.Reader) (int, error) {
	var val int8
	err := binary.Read(reader, binary.LittleEndian, &val)
	return int(val), err
}

func readUint8(reader *bytes.Reader) (int, error) {
	var val uint8
	err := binary.Read(reader, binary.LittleEndian, &val)
	return int(val), err
}

func readInt16(reader *bytes.Reader) (int, error) {
	var val int16
	err := binary.Read(reader, binary.LittleEndian, &val)
	return int(val), err
}

func readUint16(reader *bytes.Reader) (int, error) {
	var val uint16
	err := binary.Read(reader, binary.LittleEndian, &val)
	return int(val), err
}

func readInt32(reader *bytes.Reader) (int, error) {
	var val int32
	err := binary.Read(reader, binary.LittleEndian, &val)
	return int(val), err
}

func readUint32(reader *bytes.Reader) (int, error) {
	var val uint32
	err := binary.Read(reader, binary.LittleEndian, &val)
	return int(val), err
}

func readInt64(reader *bytes.Reader) (int, error) {
	var val int64
	err := binary.Read(reader, binary.LittleEndian, &val)
	return int(val), err
}

func readFloat32(reader *bytes.Reader) (float64, error) {
	var val float32
	err := binary.Read(reader, binary.LittleEndian, &val)
	return float64(val), err
}

func readFloat64(reader *bytes.Reader) (float64, error) {
	var val float64
	err := binary.Read(reader, binary.LittleEndian, &val)
	return val, err
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
			if info, ok := FormatToUnpackInfo[byte(c)]; ok {
				if sfmt, ok := info[0].(string); ok {
					pre_sfmt += sfmt
				}
			}
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
	return fmt.Sprintf("DFFormat(%d,%s,%s,%s)", df.Typ, df.Name, df.Format, df.Columns)
}
