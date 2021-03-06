package internal

import (
	"os"
	"reflect"
	"testing"
)

const workbookFilepath = "./testdata/simple/xl/workbook.xml"
const sharedStringsFilepath = "./testdata/simple/xl/sharedStrings.xml"

func TestReadWorkbook(t *testing.T) {
	f, err := os.Open(workbookFilepath)
	if err != nil {
		t.Error(err)
	}
	wb, err := readWorkbookXML(f)
	if err != nil {
		t.Error(err)
	}
	expectWb := &xlsxWorkbook{
		Sheets: xlsxSheets{
			Sheet: []xlsxSheet{
				xlsxSheet{
					Name:    "simple",
					SheetID: "1",
				},
			},
		},
	}
	if !reflect.DeepEqual(wb, expectWb) {
		t.Errorf("unexpect workbook: %+v", *wb)
	}
}

func TestReadSharedStrings(t *testing.T) {
	f, err := os.Open(sharedStringsFilepath)
	if err != nil {
		t.Error(err)
	}
	res := readSharedStringsXML(f)
	expectRes := []string{"ID", "Name", "Slice", "Foo", "Hello|World"}
	if !reflect.DeepEqual(res, expectRes) {
		t.Errorf("Unexpect res: %+v", res)
	}
}
