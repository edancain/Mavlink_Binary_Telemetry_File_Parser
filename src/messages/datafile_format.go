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
    _type          byte
    name           string
    len            int
    format         string
    columns        []string
    instance_field *string
    unit_ids       *string
    mult_ids       *string
    msg_struct     string
    msg_types      []interface{}
    msg_mults      []interface{}
    msg_fmts       []byte
    colhash        map[string]int
    a_indexes      []int
    instance_ofs   int
    instance_len   int
}

func NewDFFormat(typ byte, name string, flen int, format string, columns string, oldfmt *DFFormat) (*DFFormat, error) {
    df := &DFFormat{
        _type:    typ,
        name:   nullTerm(name),
        len:    flen,
        format: format,
    }

    if columns == "" {
        df.columns = []string{}
    } else {
        df.columns = strings.Split(columns, ",")
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
			df.msg_mults = append(msgMults, val[1])
			if c == 'a' {
				msgTypes = append(msgTypes, binary.BigEndian)
			} else {
				msgTypes = append(msgTypes, val[2])
			}
		} else {
			panic(fmt.Sprintf("DFFormat: Unsupported format char: '%c' in message %s", c, name))
		}
	}
	df.msg_struct = msgStruct
	df.msg_types = msgTypes
	df.msg_mults = msgMults
	df.msg_fmts = msgFmts

	for i, col := range df.columns {
		df.colhash[col] = i
	}

    for i, fmt := range df.msg_fmts {
        if fmt == 'a' {
            df.a_indexes = append(df.a_indexes, i)
        }
    }

    if oldfmt != nil {
        df.setUnitIds(oldfmt.unit_ids)
        df.setMultIds(oldfmt.mult_ids)
    }

    return df, nil
}

func (df *DFFormat) setUnitIds(unit_ids *string) {
    if unit_ids == nil {
        return
    }

    df.unit_ids = unit_ids
    instance_idx := strings.Index(*unit_ids, "#")
    if instance_idx != -1 {
        df.instance_field = &df.columns[instance_idx]
        pre_fmt := df.format[:instance_idx]
        pre_sfmt := ""
        for _, c := range pre_fmt {
            pre_sfmt += FORMAT_TO_STRUCT[byte(c)][0].(string)
        }
        df.instance_ofs = binary.Size(pre_sfmt)
        ifmt := df.format[instance_idx]
        df.instance_len = binary.Size(ifmt)
    }
}

func (df *DFFormat) setMultIds(mult_ids *string) {
    df.mult_ids = mult_ids
}

func (df *DFFormat) String() string {
    return fmt.Sprintf("DFFormat(%s,%s,%s,%s)", df._type, df.name, df.format, df.columns)
}


