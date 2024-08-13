package fileparser

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

/*
This code defines a structure and methods for handling data file formats, for
binary data files. It includes functionality for unpacking various data types, handling
different format characters, and managing metadata about the file format such as units and multipliers.
*/

const (
	MetricMultiplier        = 0.01
	defaultStringSize       = 64
	alternativeStringSize4  = 4
	alternativeStringSize16 = 16
)

// Error variables for specific parsing situations
var (
	ErrSkipDigits = errors.New("skip digits")
	ErrIgnoreChar = errors.New("ignore character")
)

// map format characters to their corresponding unpacking information
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

// returns the byte value of a character, or 0 if the character is null
func u_ord(c byte) byte {
	if c == 0 {
		return 0
	}
	return c
}

// DataFileFormat represents the structure of a data file format
type DataFileFormat struct {
	Typ            int
	Name           string
	Len            int
	Format         string
	Columns        []string
	InstanceField  *string
	UnitIds        *string
	MultIds        *string
	MessageStruct  string
	MessageTypes   []interface{}
	MessageMults   []interface{}
	MessageFormats []string
	ColumnHash     map[string]int
	AIndexes       []int
	InstanceOffset int
	InstanceLength int
}

// creates a new DataFileFormat instance
func NewDataFileFormat(typ int, name string, fileLength int, format string, columns []string, oldformat *DataFileFormat) (*DataFileFormat, error) {
	df := &DataFileFormat{
		Typ:     typ,
		Name:    nullTerm(name),
		Len:     fileLength,
		Format:  format,
		Columns: columns,
	}

	messageStruct := "<"
	messageMults := []interface{}{}
	messageTypes := []interface{}{}
	messageFormats := []string{}

	for _, c := range format {
		// this code is essentially building up several data structures (messageFormats, messageStruct,
		// messageMults, and messageTypes) based on the input format string. Each of these structures holds
		// different aspects of how to interpret and unpack the data:

		// messageFormats holds the raw format characters.
		// messageStruct builds a string representation of the structure (used for binary unpacking).
		// messageMults holds multipliers for each field (if any).
		// messageTypes holds type information for each field.
		if u_ord(byte(c)) == 0 {
			break
		}
		messageFormats = append(messageFormats, string(c))
		if val, ok := FormatToUnpackInfo[byte(c)]; ok {
			strVal, _ := val[0].(string)
			messageStruct += strVal
			messageMults = append(messageMults, val[1])

			if c == 'a' {
				// Endianness refers to the order in which bytes are arranged into larger numerical values
				// when stored in memory or transmitted over a network. Big-endian (BE): The most significant
				// byte is stored first (at the lowest memory address). It's often used with functions in the
				// binary package, like binary.Read() or binary.Write(), to ensure data is interpreted correctly.
				messageTypes = append(messageTypes, binary.BigEndian)
			} else {
				messageTypes = append(messageTypes, val[2])
			}
		} else {
			return nil, fmt.Errorf("DFFormat: Unsupported format char: '%c' in message %s", c, name)
		}
	}

	df.MessageStruct = messageStruct
	df.MessageTypes = messageTypes
	df.MessageMults = messageMults
	df.MessageFormats = messageFormats

	df.ColumnHash = make(map[string]int)
	for i, column := range columns {
		df.ColumnHash[column] = i
	}

	df.AIndexes = []int{}
	for i, msgFmt := range df.MessageFormats {
		if msgFmt == "a" {
			df.AIndexes = append(df.AIndexes, i)
		}
	}

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
		df.MessageStruct = "<" + df.Format

		for i := 1; i < len(df.MessageStruct); i++ {
			//elements, err := reader.unpackers[msgType](body)
			elem, err := unpackElement(reader, df.MessageStruct, &i)
			if err != nil {
				continue
			}
			if elem != nil {
				elements = append(elements, elem)
			}
		}

		return elements, nil
	}
}

// handles unpacking a single element based on the format character.
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
	case 'h', 'H', 'i', 'I', 'L', 'q', 'Q':
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

// handles unpacking elements that need to be multiplied by a metric multiplier.
func unpackMetricElement(reader *bytes.Reader, formatChar byte) (interface{}, error) {
	var err error
	var val interface{}

	switch formatChar {
	case 'c':
		var temp int16
		err = binary.Read(reader, binary.LittleEndian, &temp)
		if err == nil {
			val = float64(temp) * MetricMultiplier
		}
	case 'C':
		var temp int
		temp, err = readUint16(reader)
		if err == nil {
			val = float64(temp) * MetricMultiplier
		}
	case 'e':
		var temp int
		temp, err = readInt32(reader)
		if err == nil {
			val = float64(temp) * MetricMultiplier
		}
	case 'E':
		var temp int
		temp, err = readUint32(reader)
		if err == nil {
			val = float64(temp) * MetricMultiplier
		}
	default:
		return nil, fmt.Errorf("unsupported metric format character: %c", formatChar)
	}

	if err != nil {
		return nil, err
	}

	return val, nil
}

// handles unpacking float elements.
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

// handles unpacking integer elements.
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

// handles parsing string sizes and reading fixed-size strings.
func handleStringCase(reader *bytes.Reader, format string, i *int) (string, error) {
	strSize := parseMsgStructStrSize(format, i)

	return readFixedSizeString(reader, strSize)
}

// parses the size of the string in the message structure.
func parseMsgStructStrSize(format string, i *int) int {
	size := int(format[*i] - '0')

	if *i+1 < len(format) && format[*i+1] >= '0' && format[*i+1] <= '9' {
		size = size*base + int(format[*i+1]-'0')
		*i++
	}

	return size
}

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

// This function is crucial for handling data formats that include an instance field.
// It sets up the necessary information to correctly parse and interpret instance-specific
// data within the larger data structure. The instance field is likely used to distinguish
// between multiple instances of the same type of data within a single record or message.
func (dataFormat *DataFileFormat) SetUnitIds(unitIdentifiers *string) {
	if unitIdentifiers == nil {
		return
	}

	dataFormat.UnitIds = unitIdentifiers
	instanceIndex := strings.Index(*unitIdentifiers, "#")

	if instanceIndex != -1 {
		dataFormat.InstanceField = &dataFormat.Columns[instanceIndex]
		prefixFormat := dataFormat.Format[:instanceIndex]
		prefixStringFormat := ""

		for _, c := range prefixFormat {
			if info, ok := FormatToUnpackInfo[byte(c)]; ok {
				if stringFormat, ok := info[0].(string); ok {
					prefixStringFormat += stringFormat
				}
			}
		}

		dataFormat.InstanceOffset = binary.Size(prefixStringFormat)
		instance_format := dataFormat.Format[instanceIndex]
		dataFormat.InstanceLength = binary.Size(instance_format)
	}
}

func (df *DataFileFormat) SetMultIds(mult_ids *string) {
	df.MultIds = mult_ids
}
