package messages

import (
    "encoding/binary"
    "fmt"
    "strings"
)

type FormatToStruct struct {
    format string
    mul    float64
    typ    string
}

var formatToStruct = map[string]FormatToStruct{
    "a": {"64s", 0, "string"},
    "b": {"b", 0, "int"},
    "B": {"B", 0, "int"},
    "h": {"h", 0, "int"},
    "H": {"H", 0, "int"},
    "i": {"i", 0, "int"},
    "I": {"I", 0, "int"},
    "f": {"f", 0, "float"},
    "n": {"4s", 0, "string"},
    "N": {"16s", 0, "string"},
    "Z": {"64s", 0, "string"},
    "c": {"h", 0.01, "float"},
    "C": {"H", 0.01, "float"},
    "e": {"i", 0.01, "float"},
    "E": {"I", 0.01, "float"},
    "L": {"i", 1.0e-7, "float"},
    "d": {"d", 0, "float"},
    "M": {"b", 0, "int"},
    "q": {"q", 0, "int64"},
    "Q": {"Q", 0, "int64"},
}

type DFFormat struct {
    typ            string
    name           string
    len            int
    format         string
    columns        []string
    instance_field *string
    unit_ids       *string
    mult_ids       *string
    msg_struct     string
    msg_types      []string
    msg_mults      []float64
    msg_fmts       []string
    colhash        map[string]int
    a_indexes      []int
    instance_ofs   int
    instance_len   int
}

func NewDFFormat(typ, name string, flen int, format string, columns string, oldfmt *DFFormat) (*DFFormat, error) {
    df := &DFFormat{
        typ:    typ,
        name:   nullTerm(name),
        len:    flen,
        format: format,
    }

    if columns == "" {
        df.columns = []string{}
    } else {
        df.columns = strings.Split(columns, ",")
    }

    df.msg_struct = "<"
    df.msg_mults = []float64{}
    df.msg_types = []string{}
    df.msg_fmts = []string{}

    for _, c := range format {
        if c == 0 {
            break
        }

        fs, ok := formatToStruct[string(c)]
        if !ok {
            return nil, fmt.Errorf("unsupported format char: '%s' in message %s", string(c), name)
        }

        df.msg_fmts = append(df.msg_fmts, string(c))
        df.msg_struct += fs.format
        df.msg_mults = append(df.msg_mults, fs.mul)
        df.msg_types = append(df.msg_types, fs.typ)
    }

    df.colhash = make(map[string]int)
    for i, col := range df.columns {
        df.colhash[col] = i
    }

    df.a_indexes = []int{}
    for i, fmt := range df.msg_fmts {
        if fmt == "a" {
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
            fs := formatToStruct[string(c)]
            pre_sfmt += fs.format
        }
        df.instance_ofs = binary.Size(pre_sfmt)
        ifmt := df.format[instance_idx]
        df.instance_len = binary.Size(string(ifmt))
    }
}

func (df *DFFormat) setMultIds(mult_ids *string) {
    df.mult_ids = mult_ids
}

func (df *DFFormat) String() string {
    return fmt.Sprintf("DFFormat(%s,%s,%s,%s)", df.typ, df.name, df.format, df.columns)
}


