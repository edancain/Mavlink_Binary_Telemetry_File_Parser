package messages

import (
	"github.com/edancain/telemetry_parser/telemetry_parser/src/readers"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
)

type GPS struct {
	GWk     int
	GMS     int
	TimeUS  int
	Week    float64
	TimeMS  float64
	T       float64
	GPSTime float64
}


type DFMessage struct {
    Fmt             *DFFormat
    Elements        []interface{}
    ApplyMultiplier bool
    FieldNames      []string
    Parent          *readers.DFReaderBinary
    TimeStamp       float64
    TimeMS          float64
    StartTime float64
}

func NewDFMessage(fmt *DFFormat, elements []interface{}, applyMultiplier bool, reader *readers.DFReaderBinary) *DFMessage {
    return &DFMessage{
        Fmt:             fmt,
        Elements:        elements,
        ApplyMultiplier: applyMultiplier,
        FieldNames:      fmt.Columns,
        Parent:          reader,
        //TimeStamp:       timestamp,
        //TimeMS:          timeMS,
        StartTime: 0,
    }
}

func (df *DFMessage) ToDict() map[string]interface{} {
    d := map[string]interface{}{
        "mavpackettype": df.Fmt.Name,
    }

    for _, field := range df.FieldNames {
        d[field] = df.GetAttr(field)
    }

    return d
}

func (df *DFMessage) ToMap() map[string]interface{} {
	m := make(map[string]interface{})
	m["mavpackettype"] = df.Fmt.Name

	for _, field := range df.FieldNames {
		m[field] = df.GetAttr(field)
	}

	return m
}

func (df *DFMessage) GetAttr(field string) interface{} {
    i, ok := df.Fmt.Colhash[field]
    if !ok {
        panic(errors.New("AttributeError: " + field))
    }

    if df.Fmt.MsgFmts[i] == 'Z' && df.Fmt.Name == "FILE" {
        return df.Elements[i]
    }

    if str, ok := df.Elements[i].(string); ok {
		if df.Fmt.MsgFmts[i] != 'M' || df.ApplyMultiplier {
			return str
		}
		return nil
	}

    var v = df.Elements[i]
    if df.Fmt.Format[i] == 'a' {
        return v
    }

    if df.Fmt.MsgTypes[i] == reflect.TypeOf("").Elem() {
        v = nullTerm(v.(string))
    }
    
    if df.Fmt.MsgTypes[i] != nil && df.ApplyMultiplier {
        v = v.(float64) * df.Fmt.MsgTypes[i].(float64)
    }

    return v
}

func nullTerm(s string) string {
    // Find the index of the first null byte
    nullIndex := strings.IndexByte(s, 0)
    if nullIndex != -1 {
        // Return the string up to the null byte
        return s[:nullIndex]
    }
    // If no null byte found, return the original string
    return s
}

func (df *DFMessage) SetAttr(field string, value interface{}) {
    if field[0] >= 'A' && field[0] <= 'Z' && df.Fmt.Colhash[field] != 0 {
        i := df.Fmt.Colhash[field]
        if df.Fmt.MsgMults[i] != 0 && df.ApplyMultiplier {
            value = value.(float64) / float64(df.Fmt.MsgMults[i].(float64))
        }
        df.Elements[i] = value
    } 
}

func (df *DFMessage) GetType() string {
    return df.Fmt.Name
}

func (df *DFMessage) String() string {
    ret := fmt.Sprintf("%s {", df.Fmt.Name)
    colCount := 0

    for _, c := range df.Fmt.Columns {
        val := df.GetAttr(c)
        if v, ok := val.(float64); ok && math.IsNaN(v) {
            val = "qnan"
        }

        ret += fmt.Sprintf("%s : %s, ", c, val)
        colCount++
    }

    if colCount != 0 {
        ret = ret[:len(ret)-2]
    }

    return ret + "}"
}

func (df *DFMessage) GetMsgBuf() []byte {
    var values []interface{}
	for i := range df.Fmt.Columns {
		if i >= len(df.Fmt.MsgMults) {
			continue
		}
		mul := df.Fmt.MsgMults[i]
		name := df.Fmt.Columns[i]
		if name == "Mode" && contains(df.Fmt.Columns, "ModeNum") {
			name = "ModeNum"
		}
		v := df.GetAttr(name)
		switch vt := v.(type) {
		case string:
			v = []byte(vt)
		}

		if mul != nil {
			v = int(float64(v.(int)) / mul.(float64))
		}
		values = append(values, v)
	}

	ret1 := []byte{0xA3, 0x95, byte(df.Fmt.Typ)}
	ret2 := structPack(values...)
	return append(ret1, ret2...)
}

func structPack(values ...interface{}) []byte {
	var buf []byte
	for _, v := range values {
		switch vt := v.(type) {
		case int:
			buf = append(buf, uint8(vt))
		case int32:
			b := make([]byte, 4)
			binary.LittleEndian.PutUint32(b, uint32(vt))
			buf = append(buf, b...)
		case int64:
			b := make([]byte, 8)
			binary.LittleEndian.PutUint64(b, uint64(vt))
			buf = append(buf, b...)
		case float64:
			b := make([]byte, 8)
			binary.LittleEndian.PutUint64(b, math.Float64bits(vt))
			buf = append(buf, b...)
		case []byte:
			buf = append(buf, vt...)
		default:
			panic(fmt.Sprintf("unsupported type: %T", vt))
		}
	}
	return buf
}

func contains(arr []string, str string) bool {
	for _, v := range arr {
		if v == str {
			return true
		}
	}
	return false
}

func (df *DFMessage) GetFieldNames() []string {
    return df.FieldNames
}

func (df *DFMessage) GetItem(key string) (*DFMessage, error) {
    if *df.Fmt.InstanceField == "" {
        return nil, errors.New("IndexError")
    }

    k := fmt.Sprintf("%s[%s]", df.Fmt.Name, key)
    if _, ok := df.Parent.Messages[k]; !ok {
        return nil, errors.New("IndexError")
    }

    return df.Parent.Messages[k], nil
}