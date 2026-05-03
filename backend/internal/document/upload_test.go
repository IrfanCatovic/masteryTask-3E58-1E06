package document

import (
	"strings"
	"testing"
)

func TestParseCSVLineItemColumnAliases(t *testing.T) {
	input := "desc,qty,price,total\nItem,1,78,78\nItem,2,84,168\n"
	res, err := parseCSVDocument(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Document.LineItems) != 2 {
		t.Fatalf("want 2 line items, got %d", len(res.Document.LineItems))
	}
}

func TestParseTXTTableWithAliases(t *testing.T) {
	input := "Invoice TXT-1\nTotal: 246 EUR\ndesc qty price total\nItem 1 78 78\nItem 2 84 168\n"
	res, err := parseTXTDocument(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Document.LineItems) != 2 {
		t.Fatalf("want 2 line items from plain-text table, got %d", len(res.Document.LineItems))
	}
}
