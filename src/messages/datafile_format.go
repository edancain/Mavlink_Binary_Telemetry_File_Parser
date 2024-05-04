package messages

import (
	"encoding/binary"
	"fmt"
	"reflect"
	"strings"
)

var FORMAT_TO_STRUCT = map[byte][3]interface{}{
    'a': {"64s", nil, reflect.TypeOf("").Elem()},
    'b': {"b", nil, reflect.TypeOf(int(0))},
    'B': {"B", nil, reflect.TypeOf(int(0))},
    'h': {"h", nil, reflect.TypeOf(int16(0))},
    'H': {"H", nil, reflect.TypeOf(uint16(0))},
    'i': {"i", nil, reflect.TypeOf(int32(0))},
    'I': {"I", nil, reflect.TypeOf(uint32(0))},
    'f': {"f", nil, reflect.TypeOf(float32(0))},
    'n': {"4s", nil, reflect.TypeOf("").Elem()},
    'N': {"16s", nil, reflect.TypeOf("").Elem()},
    'Z': {"64s", nil, reflect.TypeOf("").Elem()},
    'c': {"h", 0.01, reflect.TypeOf(float64(0))},
    'C': {"H", 0.01, reflect.TypeOf(float64(0))},
    'e': {"i", 0.01, reflect.TypeOf(float64(0))},
    'E': {"I", 0.01, reflect.TypeOf(float64(0))},
    'L': {"i", 1.0e-7, reflect.TypeOf(float64(0))},
    'd': {"d", nil, reflect.TypeOf(float64(0))},
    'M': {"b", nil, reflect.TypeOf(int(0))},
    'q': {"q", nil, reflect.TypeOf(int64(0))}, // Backward compat
    'Q': {"Q", nil, reflect.TypeOf(int64(0))}, // Backward compat
}

func u_ord(c byte) byte {
	if c == 0 {
		return 0
	}
	return c
}

type DFFormat struct {
    Typ            byte
    Name           string
    Len            int64
    Format         string
    Columns        []string
    InstanceField *string
    UnitIds       *string
    MultIds       *string
    MsgStruct     string
    MsgTypes      []interface{}
    MsgMults      []interface{}
    MsgFmts       []byte
    Colhash        map[string]int
    AIndexes      []int
    InstanceOfs   int
    InstanceLen   int
}

func NewDFFormat(typ byte, name string, flen int64, format string, columns string, oldfmt *DFFormat) (*DFFormat, error) {
    df := &DFFormat{
        Typ:    typ,
        Name:   nullTerm(name),
        Len:    flen,
        Format: format,
    }

    if columns == "" {
        df.Columns = []string{}
    } else {
        df.Columns = strings.Split(columns, ",")
    }

    msgStruct := "<"
    msgMults := []interface{}{}
	msgTypes := []interface{}{}
	msgFmts := []byte{}

    for _, c := range format {
		if u_ord(byte(c)) == 0 {
			break
		}
		msgFmts = append(msgFmts, byte(c))
		if val, ok := FORMAT_TO_STRUCT[byte(c)]; ok {
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

	for i, col := range df.Columns {
		df.Colhash[col] = i
	}

    for i, fmt := range df.MsgFmts {
        if fmt == 'a' {
            df.AIndexes = append(df.AIndexes, i)
        }
    }

    if oldfmt != nil {
        df.SetUnitIds(oldfmt.UnitIds)
        df.SetMultIds(oldfmt.MultIds)
    }

    return df, nil
}

func (df *DFFormat) SetUnitIds(unit_ids *string) {
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
            pre_sfmt += FORMAT_TO_STRUCT[byte(c)][0].(string)
        }
        df.InstanceOfs = binary.Size(pre_sfmt)
        ifmt := df.Format[instance_idx]
        df.InstanceLen = binary.Size(ifmt)
    }
}

func (df *DFFormat) SetMultIds(mult_ids *string) {
    df.MultIds = mult_ids
}

func (df *DFFormat) String() string {
    return fmt.Sprintf("DFFormat(%s,%s,%s,%s)", df.Typ, df.Name, df.Format, df.Columns)
}


