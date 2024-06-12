package src

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strings"
)

/* type GPS struct {
	GWk     int
	GMS     int
	TimeUS  int
	Week    float64
	TimeMS  float64
	T       float64
	GPSTime float64
	Lat     float64
	Lon     float64
	Alt     float64
} */

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
		d[field] = m.GetAttr(field)
	}

	return d
}

func (m *DataFileMessage) GetAttr(field string) interface{} {
	i, ok := m.Fmt.Colhash[field]
	if !ok {
		return nil
	}
	if m.Fmt.MsgFmts[i] == "Z" && m.Fmt.Name == "FILE" {
		return m.Elements[i]
	}
	var v interface{}
	switch elem := m.Elements[i].(type) {
	case []byte:
		v = string(elem)
	default:
		v = elem
	}
	if m.Fmt.Format[i] == 'a' {
		// pass
	} else if m.Fmt.Format[i] != 'M' || m.ApplyMultiplier {
		if fn, ok := m.Fmt.MsgTypes[i].(func(interface{}) interface{}); ok {
			v = fn(v)
		} else if u, ok := m.Fmt.MsgTypes[i].(uint32); ok {
			switch v := v.(type) {
			case float64:
				v = v * float64(u)
			case uint32:
				if u != 0 {
					v = v * u
				}
			case int:
				v = v
			default:
				panic(fmt.Sprintf("unexpected type: %T", v))
			}
		}
	}
	if _, ok := v.(string); ok {
		v = nullTerm(v.(string))
	}

	if m.Fmt.MsgMults != nil && len(m.Fmt.MsgMults) > i && m.ApplyMultiplier {
		if mult, ok := m.Fmt.MsgMults[i].(float64); ok {
			switch v := v.(type) {
			case float64:
				v = v * mult
			case uint32:
				v = uint32(float64(v) * mult)
			default:
				panic(fmt.Sprintf("unexpected type: %T", v))
			}
		} else {
			panic(fmt.Sprintf("unexpected type for multiplier: %T", m.Fmt.MsgMults[i]))
		}
	}
	return v
}

func nullTerm(s string) string {
	idx := strings.Index(s, "\x00")
	if idx != -1 {
		s = s[:idx]
	}
	return s
}

func (m *DataFileMessage) SetAttr(field string, value interface{}) {
	i, ok := m.Fmt.Colhash[field]
	if !ok {
		panic(errors.New("AttributeError: " + field))
	}
	if m.Fmt.MsgMults[i] != nil && m.ApplyMultiplier {
		value = value.(float64) / m.Fmt.MsgMults[i].(float64)
	}
	m.Elements[i] = value
}

func (df *DataFileMessage) GetType() string {
	return df.Fmt.Name
}

func (m *DataFileMessage) String() string {
	var buf bytes.Buffer
	buf.WriteString(m.Fmt.Name)
	buf.WriteString(" {")
	for i, c := range m.Fmt.Columns {
		val := m.GetAttr(c)
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
		v := m.GetAttr(name)
		if mul != nil {
			v = v.(float64) / mul.(float64)
			v = int(math.Round(v.(float64)))
		}
		values[i] = v
	}

	ret1 := []byte{0xA3, 0x95, byte(m.Fmt.Typ)}
	ret2 := struct_pack(m.Fmt.MsgStruct, values...)
	if ret2 == nil {
		return nil
	}
	return append(ret1, ret2...)
}

func struct_pack(format string, values ...interface{}) []byte {
	var buf bytes.Buffer
	for i := 0; i < len(format); i++ {
		switch format[i] {
		case 'B', 'b':
			val := values[0].(uint8)
			buf.WriteByte(val)
			values = values[1:]
		case 'H':
			val := values[0].(uint16)
			binary.Write(&buf, binary.LittleEndian, val)
			values = values[1:]
		case 'h':
			val := values[0].(int16)
			binary.Write(&buf, binary.LittleEndian, val)
			values = values[1:]
		case 'I':
			val := values[0].(uint32)
			binary.Write(&buf, binary.LittleEndian, val)
			values = values[1:]
		case 'i':
			val := values[0].(int32)
			binary.Write(&buf, binary.LittleEndian, val)
			values = values[1:]
		case 'Q':
			val := values[0].(uint64)
			binary.Write(&buf, binary.LittleEndian, val)
			values = values[1:]
		case 'q':
			val := values[0].(int64)
			binary.Write(&buf, binary.LittleEndian, val)
			values = values[1:]
		case 'f':
			val := values[0].(float32)
			binary.Write(&buf, binary.LittleEndian, val)
			values = values[1:]
		case 'd':
			val := values[0].(float64)
			binary.Write(&buf, binary.LittleEndian, val)
			values = values[1:]
		default:
			if format[i] >= '0' && format[i] <= '9' {
				size := int(format[i] - '0')
				if i+1 < len(format) && format[i+1] >= '0' && format[i+1] <= '9' {
					size = size*10 + int(format[i+1]-'0')
					i++
				}
				if i+1 < len(format) && format[i+1] == 's' {
					i++
					val := values[0].(string)
					buf.WriteString(val[:size])
					values = values[1:]
				} else {
					val := values[0].([]byte)
					buf.Write(val[:size])
					values = values[1:]
				}
			} else {
				panic(fmt.Errorf("unsupported format character: %c", format[i]))
			}
		}
	}
	return buf.Bytes()
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

func (m *DataFileMessage) GetItem(key int) *DataFileMessage {
	if m.Fmt.InstanceField == nil {
		panic(errors.New("IndexError"))
	}
	k := fmt.Sprintf("%s[%d]", m.Fmt.Name, key)
	if _, ok := m.Parent.Messages[k]; !ok {
		panic(errors.New("IndexError"))
	}
	return m.Parent.Messages[k]
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











