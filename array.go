package main

import (
	"bytes"
	"database/sql/driver"
	"encoding/csv"
	"strconv"
)

type StringArray []string

func (strs StringArray) Value() (driver.Value, error) {
	buf := new(bytes.Buffer)
	buf.WriteString(`{`)
	for i, s := range strs {
		if i > 0 {
			buf.WriteString(`,`)
		}
		buf.WriteString(strconv.Quote(s))
	}
	buf.WriteString(`}`)
	return buf.Bytes(), nil
}

func (strs *StringArray) Scan(src interface{}) error {
	bs := []byte(src.([]uint8))
	bs = bs[1 : len(bs)-1]
	if len(bs) == 0 {
		return nil
	}
	fields, err := csv.NewReader(bytes.NewReader(bs)).Read()
	if err != nil {
		return err
	}
	*strs = StringArray(fields)
	return nil
}

type Int64Array []int64

var _ driver.Valuer = (Int64Array)(nil)

func (ints Int64Array) Value() (driver.Value, error) {
	buf := new(bytes.Buffer)
	buf.WriteString("{")
	for i, s := range ints {
		if i > 0 {
			buf.WriteString(",")
		}
		buf.WriteString(strconv.FormatInt(s, 10))
	}
	buf.WriteString("}")
	bs := buf.Bytes()
	return bs, nil
}

func (ints *Int64Array) Scan(src interface{}) error {
	bs := []byte(src.([]uint8))
	bs = bs[1 : len(bs)-1]
	if len(bs) == 0 {
		return nil
	}
	fields, err := csv.NewReader(bytes.NewReader(bs)).Read()
	if err != nil {
		return err
	}
	var is []int64
	for _, field := range fields {
		i, err := strconv.ParseInt(field, 10, 64)
		if err != nil {
			return err
		}
		is = append(is, i)
	}
	*ints = Int64Array(is)
	return nil
}
