package messages

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"unicode"
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

type TimeMsg struct {
	StartTime float64
}

type DFMessage struct {
    Fmt             *Format
    Elements        []interface{}
    ApplyMultiplier bool
    FieldNames      []string
    Parent          *Parent
    TimeStamp       float64
    TimeMS          float64
}

type Format struct {
    Name       string
    Columns    []string
    ColHash    map[string]int
    MsgFmts    []string
    MsgTypes   []interface{}
    Format     []string
    MsgMults   []float64
    MsgStruct  string
    Type       byte
    InstanceField string
}

type Parent struct {
    Messages map[string]*DFMessage
}

func NewDFMessage(fmt *Format, elements []interface{}, applyMultiplier bool, parent *Parent, timestamp float64, timeMS float64) *DFMessage {
    return &DFMessage{
        Fmt:             fmt,
        Elements:        elements,
        ApplyMultiplier: applyMultiplier,
        FieldNames:      fmt.Columns,
        Parent:          parent,
        TimeStamp:       timestamp,
        TimeMS:          timeMS,
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

func (df *DFMessage) GetAttr(field string) interface{} {
    i, ok := df.Fmt.ColHash[field]
    if !ok {
        panic(errors.New("AttributeError: " + field))
    }

    if df.Fmt.MsgFmts[i] == "Z" && df.Fmt.Name == "FILE" {
        return df.Elements[i]
    }

    var v interface{}
    switch df.Elements[i].(type) {
    case []byte:
        v = string(df.Elements[i].([]byte))
    default:
        v = df.Elements[i]
    }

    if df.Fmt.Format[i] == "a" {
        return v
    }

    if df.Fmt.Format[i] != "M" || df.ApplyMultiplier {
        v = df.Fmt.MsgTypes[i]
    }

    if df.Fmt.MsgTypes[i] == reflect.TypeOf("") {
        v = nullTerm(v.(string))
    }

    if (i >= 0 && i < len(df.Fmt.MsgMults) && df.Fmt.MsgMults[i] != 0) && df.ApplyMultiplier {
        v = v.(float64) * df.Fmt.MsgMults[i]
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
    if !unicode.IsUpper(rune(field[0])) || df.Fmt.ColHash[field] == 0 {
        df.Elements[df.Fmt.ColHash[field]] = value
    } else {
        i := df.Fmt.ColHash[field]
        if df.Fmt.MsgMults[i] != 0 && df.ApplyMultiplier {
            value = value.(float64) / df.Fmt.MsgMults[i]
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
    values := make([]interface{}, 0, len(df.Fmt.Columns))

    for i := range df.Fmt.Columns {
        if i >= len(df.Fmt.MsgMults) {
            continue
        }

        mul := df.Fmt.MsgMults[i]
        name := df.Fmt.Columns[i]
        if name == "Mode" && df.Fmt.Columns[i] == "ModeNum" {
            name = "ModeNum"
        }

        v := df.GetAttr(name)
        if mul != 0 {
            v = int(math.Round(v.(float64) / mul))
        }

        values = append(values, v)
    }

    ret1 := []byte{0xA3, 0x95, df.Fmt.Type}
    ret2 := make([]byte, binary.Size(values))
    for i, v := range values {
        val := uint64(v.(float64))
        binary.BigEndian.PutUint64(ret2[i*8:], val)
    }

    return append(ret1, ret2...)
}

func (df *DFMessage) GetFieldNames() []string {
    return df.FieldNames
}

func (df *DFMessage) GetItem(key string) (*DFMessage, error) {
    if df.Fmt.InstanceField == "" {
        return nil, errors.New("IndexError")
    }

    k := fmt.Sprintf("%s[%s]", df.Fmt.Name, key)
    if _, ok := df.Parent.Messages[k]; !ok {
        return nil, errors.New("IndexError")
    }

    return df.Parent.Messages[k], nil
}