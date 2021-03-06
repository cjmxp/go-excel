package internal

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	convert "github.com/szyhf/go-convert"

	"github.com/szyhf/go-excel/internal/twenty_six"
)

type Read struct {
	connecter *Connect
	decoder   *xml.Decoder
	title     *Row
	schameMap map[reflect.Type]*Schema
}

func (this *Read) Next() bool {
	for t, err := this.decoder.Token(); err == nil; t, err = this.decoder.Token() {
		switch token := t.(type) {
		case xml.StartElement:
			switch token.Name.Local {
			case ROW:
				return true
			}
		}
	}
	return false
}

// Read current row into an object by its pointer
func (this *Read) Read(i interface{}) error {
	t := reflect.TypeOf(i)
	switch t.Kind() {
	case reflect.Slice, reflect.Chan, reflect.Array, reflect.Map, reflect.Ptr:
		t = t.Elem()
	default:
		return fmt.Errorf("%T should be pointer.", i)
	}

	s := this.getSchame(t)
	v := reflect.ValueOf(i)
	if v.IsNil() {
		v.Set(reflect.New(t))
	}
	v = v.Elem()

	return this.readToValue(s, v)
}

func (this *Read) Close() error {
	if this.decoder != nil {
		this.decoder = nil
	}
	this.connecter = nil
	this.title = nil
	this.schameMap = nil
	return nil
}

// Read all rows
func (this *Read) ReadAll(container interface{}) error {
	val := reflect.ValueOf(container)
	typ := reflect.Indirect(val).Type()

	if val.Kind() != reflect.Ptr {
		return ErrInvalidConatiner
	}
	if typ.Kind() != reflect.Slice {
		return ErrInvalidConatiner
	}

	elemTyp := typ.Elem()
	elemSchema := newSchema(elemTyp)

	for this.Next() {
		elmVal := SliceNextElem(val.Elem())
		err := this.readToValue(elemSchema, elmVal)
		if err != nil {
			return err
		}
	}
	return nil
}

func (this *Read) readToValue(s *Schema, v reflect.Value) (err error) {
	defer func() {
		if rc := recover(); rc != nil {
			err = fmt.Errorf("%s", rc)
		}
	}()

	tempCell := &xlsxC{}
	fieldsMap := this.title.MapToFields(s)
	for t, err := this.decoder.Token(); err == nil; t, err = this.decoder.Token() {
		switch token := t.(type) {
		case xml.StartElement:
			if token.Name.Local == "c" {
				tempCell.R = ""
				tempCell.T = ""
				for _, a := range token.Attr {
					switch a.Name.Local {
					case "r":
						tempCell.R = a.Value
					case "t":
						tempCell.T = a.Value
					}
				}
			}
		case xml.EndElement:
			if token.Name.Local == ROW {
				// fill default value to column not read.
				for _, notFilledFields := range fieldsMap {
					for _, fieldCnf := range notFilledFields {
						fieldValue := v.Field(fieldCnf.FieldIndex)
						// fmt.Printf("Fill %s = %v with default: %s", v.Type().Field(fieldCnf.FieldIndex).Name, fieldValue.Interface(), fieldCnf.DefaultValue)
						err = fieldCnf.ScanDefault(fieldValue)
						if err != nil {
							return err
						}
					}
				}
				// 结束当前行
				return err
			}
		case xml.CharData:
			trimedColumnName := strings.TrimRight(tempCell.R, ALL_NUMBER)
			columnIndex := twentySix.ToDecimalism(trimedColumnName)
			var valStr string
			if tempCell.T == S {
				// get string from shared
				valStr = this.connecter.getSharedString(convert.MustInt(string(token)))
			} else {
				valStr = string(token)
			}

			fields, ok := fieldsMap[columnIndex]
			if !ok {
				err = fmt.Errorf("unexpect column at \"%s\"", trimedColumnName)
				break
			}
			for _, fieldCnf := range fields {
				fieldValue := v.Field(fieldCnf.FieldIndex)
				err = fieldCnf.Scan(valStr, fieldValue)
				if err != nil && len(valStr) > 0 {
					return err
				}
			}
			if err == nil {
				delete(fieldsMap, columnIndex)
			}
		}
	}

	return errors.New("No row")
}

func (this *Read) getSchame(t reflect.Type) *Schema {
	s, ok := this.schameMap[t]
	if !ok {
		s = newSchema(t)
		this.schameMap[t] = s
	}
	return s
}

func newReader(cn *Connect, workSheetFileReader io.Reader, titleRowIndex, skip int) (Reader, error) {
	rd, err := newBaseReaderByWorkSheetFile(cn, workSheetFileReader)
	if err != nil {
		return nil, err
	}
	// consider title row
	var i = 0
	// <= because Next() have to put the pointer to the Index row.
	for ; i <= titleRowIndex; i++ {
		if !rd.Next() {
			return rd, nil
		}
	}
	rd.title, err = newRowAsMap(rd)

	// consider skip
	// -1 because Next() need to put the pointer one row before first data row.
	for i = 0; i < skip-1; i++ {
		if !rd.Next() {
			return rd, nil
		}
	}
	rd.schameMap = make(map[reflect.Type]*Schema)
	return rd, err
}

// Make a base reader to sheet
func newBaseReaderByWorkSheetFile(cn *Connect, rc io.Reader) (*Read, error) {
	decoder := xml.NewDecoder(rc)
	// step into root [xml.StartElement] token
	func(decoder *xml.Decoder) {
		for t, err := decoder.Token(); err == nil; t, err = decoder.Token() {
			// [xml.ProcInst]
			// [xml.CharData]
			// [xml.StartElement]
			switch t.(type) {
			case xml.StartElement:
				return
			}
		}
	}(decoder)

	func(decoder *xml.Decoder) {
		// use func block to break to 'for' range
		for t, err := decoder.Token(); err == nil; t, err = decoder.Token() {
			// fmt.Printf("%+v\n\n", t)
			switch token := t.(type) {
			case xml.StartElement:
				switch token.Name.Local {
				case SHEET_DATA:
					return
				default:
					decoder.Skip()
				}
			}
		}
	}(decoder)

	rd := &Read{
		connecter: cn,
		decoder:   decoder,
	}

	return rd, nil
}
