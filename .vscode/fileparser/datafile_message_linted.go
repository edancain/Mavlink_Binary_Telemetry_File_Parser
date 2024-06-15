package fileparser

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

const (
	base = 10
)

type DataFileMessage struct {
	Fmt             *DataFileFormat
	Elements        []interface{}
	ApplyMultiplier bool
	FieldNames      []string
	Parent          *BinaryDataFileReader
}

func NewDFMessage(fmt *DataFileFormat, elements []interface{}, applyMultiplier bool, reader *BinaryDataFileReader) *DataFileMessage {
	return &DataFileMessage{
		Fmt:             fmt,
		Elements:        elements,
		ApplyMultiplier: applyMultiplier,
		FieldNames:      fmt.Columns,
		Parent:          reader,
	}
}

func (m *DataFileMessage) ToMap() map[string]interface{} {
	d := make(map[string]interface{})
	d["mavpackettype"] = m.Fmt.Name

	for _, field := range m.FieldNames {
		d[field], _ = m.GetAttr(field)
	}

	return d
}

func (m *DataFileMessage) GetAttr(field string) (interface{}, error) {
	index, ok := m.Fmt.Colhash[field]
	if !ok {
		return nil, fmt.Errorf("attribute %s not found", field)
	}

	value := m.Elements[index]

	// Handle specific cases based on format and message type
	if m.Fmt.MsgFmts[index] == "Z" && m.Fmt.Name == "FILE" {
		return value, nil
	}

	// Convert []byte to string if necessary
	if bytesValue, ok := value.([]byte); ok {
		value = string(bytesValue)
	}

	// Apply type-specific functions or multipliers
	if m.Fmt.Format[index] != 'M' || m.ApplyMultiplier {
		value = applyTypeFunctions(value, m.Fmt.MsgTypes[index])
	}

	// Convert to string with null termination
	if stringValue, ok := value.(string); ok {
		value = nullTerm(stringValue)
	}

	return value, nil
}

func applyTypeFunctions(value interface{}, msgType interface{}) interface{} {
	switch msgType := msgType.(type) {
	case func(interface{}) interface{}:
		return msgType(value)
	case uint32:
		switch v := value.(type) {
		case float64:
			return v * float64(msgType)
		case uint32:
			if msgType != 0 {
				return v * msgType
			}
		case int:
			return v
		}
	default:
		// Handle other types or return value as is
	}
	return value
}

func nullTerm(s string) string {
	idx := strings.Index(s, "\x00")
	if idx != -1 {
		s = s[:idx]
	}
	return s
}

func (m *DataFileMessage) SetAttr(field string, value interface{}) error {
	i, ok := m.Fmt.Colhash[field]
	if !ok {
		return fmt.Errorf("AttributeError: " + field)
	}
	if m.Fmt.MsgMults[i] != nil && m.ApplyMultiplier {
		value = value.(float64) / m.Fmt.MsgMults[i].(float64)
	}
	m.Elements[i] = value
	return nil
}

func (m *DataFileMessage) GetType() string {
	return m.Fmt.Name
}

func (m *DataFileMessage) String() string {
	var buf bytes.Buffer
	buf.WriteString(m.Fmt.Name)
	buf.WriteString(" {")
	for i, c := range m.Fmt.Columns {
		val, err := m.GetAttr(c)
		if err != nil {
			return ""
		}
		buf.WriteString(c)
		buf.WriteString(" : ")
		buf.WriteString(fmt.Sprintf("%v", val))
		if i < len(m.Fmt.Columns)-1 {
			buf.WriteString(", ")
		}
	}
	buf.WriteString("}")
	return buf.String()
}

func (m *DataFileMessage) GetMsgBuf() []byte {
	values := make([]interface{}, len(m.Fmt.Columns))
	for i := 0; i < len(m.Fmt.Columns); i++ {
		if i >= len(m.Fmt.MsgMults) {
			continue
		}
		mul := m.Fmt.MsgMults[i]
		name := m.Fmt.Columns[i]
		if name == "Mode" && contains(m.Fmt.Columns, "ModeNum") {
			name = "ModeNum"
		}
		v, err := m.GetAttr(name)
		if err != nil {
			return nil
		}
		if mul != nil {
			v = v.(float64) / mul.(float64)
			v = int(math.Round(v.(float64)))
		}
		values[i] = v
	}

	ret1 := []byte{0xA3, 0x95, byte(m.Fmt.Typ)}
	ret2, _ := struct_pack(m.Fmt.MsgStruct, values...)
	if ret2 == nil {
		return nil
	}
	return append(ret1, ret2...)
}

func struct_pack(format string, values ...interface{}) ([]byte, error) {
	var buf bytes.Buffer

	for i := 0; i < len(format); i++ {
		var err error

		switch format[i] {
		case 'B', 'b':
			err = writeByte(&buf, values)
		case 'H':
			err = writeUint16(&buf, values)
		case 'h':
			err = writeInt16(&buf, values)
		case 'I':
			err = writeUint32(&buf, values)
		case 'i':
			err = writeInt32(&buf, values)
		case 'Q':
			err = writeUint64(&buf, values)
		case 'q':
			err = writeInt64(&buf, values)
		case 'f':
			err = writeFloat32(&buf, values)
		case 'd':
			err = writeFloat64(&buf, values)
		default:
			if format[i] >= '0' && format[i] <= '9' {
				i = parseSizeSpecifier(&buf, format, i, values)
				if err != nil {
					return nil, err
				}
			} else {
				return nil, fmt.Errorf("unsupported format character: %c", format[i])
			}
		}

		if err != nil {
			return nil, fmt.Errorf("binary write error: %w", err)
		}

		values = values[1:]
	}

	return buf.Bytes(), nil
}

func writeByte(buf *bytes.Buffer, values []interface{}) error {
	val, ok := values[0].(uint8)
	if !ok {
		return fmt.Errorf("writeByte: expected uint8, got %T", values[0])
	}

	if err := buf.WriteByte(val); err != nil {
		return fmt.Errorf("writeByte: error writing byte to buffer: %w", err)
	}

	return nil
}

func writeUint16(buf *bytes.Buffer, values []interface{}) error {
	val, _ := values[0].(uint16)
	return binary.Write(buf, binary.LittleEndian, val)
}

func writeInt16(buf *bytes.Buffer, values []interface{}) error {
	val, _ := values[0].(int16)
	return binary.Write(buf, binary.LittleEndian, val)
}

func writeUint32(buf *bytes.Buffer, values []interface{}) error {
	val, _ := values[0].(uint32)
	return binary.Write(buf, binary.LittleEndian, val)
}

func writeInt32(buf *bytes.Buffer, values []interface{}) error {
	val, _ := values[0].(int32)
	return binary.Write(buf, binary.LittleEndian, val)
}

func writeUint64(buf *bytes.Buffer, values []interface{}) error {
	val, _ := values[0].(uint64)
	return binary.Write(buf, binary.LittleEndian, val)
}

func writeInt64(buf *bytes.Buffer, values []interface{}) error {
	val, _ := values[0].(int64)
	return binary.Write(buf, binary.LittleEndian, val)
}

func writeFloat32(buf *bytes.Buffer, values []interface{}) error {
	val, _ := values[0].(float32)
	return binary.Write(buf, binary.LittleEndian, val)
}

func writeFloat64(buf *bytes.Buffer, values []interface{}) error {
	val, _ := values[0].(float64)
	return binary.Write(buf, binary.LittleEndian, val)
}

func parseSizeSpecifier(buf *bytes.Buffer, format string, i int, values []interface{}) int {
	size := int(format[i] - '0')

	if i+1 < len(format) && format[i+1] >= '0' && format[i+1] <= '9' {
		size = size*base + int(format[i+1]-'0')
		i++
	}

	if i+1 < len(format) && format[i+1] == 's' {
		i++
		val, _ := values[0].(string)
		buf.WriteString(val[:size])
	} else {
		val, _ := values[0].([]byte)
		buf.Write(val[:size])
	}

	return i
}

func contains(arr []string, str string) bool {
	for _, v := range arr {
		if v == str {
			return true
		}
	}
	return false
}

func (m *DataFileMessage) GetFieldnames() []string {
	return m.FieldNames
}

func (m *DataFileMessage) GetItem(key int) (*DataFileMessage, error) {
	if m.Fmt.InstanceField == nil {
		return nil, fmt.Errorf("IndexError")
	}
	k := fmt.Sprintf("%s[%d]", m.Fmt.Name, key)
	if _, ok := m.Parent.Messages[k]; !ok {
		return nil, fmt.Errorf("IndexError")
	}
	return m.Parent.Messages[k], nil
}

func (m *DataFileMessage) GetMessage() string {
	for i, field := range m.FieldNames {
		if field == "Message" {
			// Check if the element is of type []uint8
			if byteArray, ok := m.Elements[i].([]uint8); ok {
				// Convert the []uint8 to a string
				str := string(byteArray)
				// Trim the string at the first null character
				str = nullTerm(str)
				return str
			}
		}
	}
	return ""
}

func (m *DataFileMessage) GetMode() int {
	for i, field := range m.FieldNames {
		if field == "Mode" {
			return m.Elements[i].(int)
		}

		if field == "ModeNum" {
			return m.Elements[i].(int)
		}
	}
	return -1
}
