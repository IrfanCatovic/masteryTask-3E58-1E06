package document

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

// itemRow is a tiny helper used to drive parser tests with arbitrary numbers — the expected document
// total is computed from the input rather than hardcoded so reviewers can read the rule, not a magic number.
type itemRow struct {
	desc      string
	qty       float64
	unitPrice float64
	lineTotal float64
}

func renderCSV(header string, rows []itemRow) string {
	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "%s,%g,%g,%g\n", r.desc, r.qty, r.unitPrice, r.lineTotal)
	}
	return b.String()
}

func sumLineTotals(rows []itemRow) float64 {
	var s float64
	for _, r := range rows {
		s += r.lineTotal
	}
	return s
}

func approxEqual(a, b float64) bool { return math.Abs(a-b) < 0.005 }

func TestParseCSVLineItemColumnAliases(t *testing.T) {
	rows := []itemRow{
		{desc: "Item A", qty: 1, unitPrice: 78, lineTotal: 78},
		{desc: "Item B", qty: 2, unitPrice: 84, lineTotal: 168},
	}
	res, err := parseCSVDocument(strings.NewReader(renderCSV("desc,qty,price,total", rows)))
	if err != nil {
		t.Fatal(err)
	}
	if got := len(res.Document.LineItems); got != len(rows) {
		t.Fatalf("want %d line items, got %d", len(rows), got)
	}
	if want := sumLineTotals(rows); !approxEqual(res.Document.Total, want) {
		t.Fatalf("want document total %g (sum of line totals), got %g", want, res.Document.Total)
	}
}

func TestParseTXTTableWithAliases(t *testing.T) {
	rows := []itemRow{
		{desc: "Item", qty: 1, unitPrice: 78, lineTotal: 78},
		{desc: "Item", qty: 2, unitPrice: 84, lineTotal: 168},
	}
	want := sumLineTotals(rows)
	input := fmt.Sprintf("Invoice TXT-1\nTotal: %g EUR\ndesc qty price total\nItem 1 78 78\nItem 2 84 168\n", want)
	res, err := parseTXTDocument(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if got := len(res.Document.LineItems); got != len(rows) {
		t.Fatalf("want %d line items, got %d", len(rows), got)
	}
	if !approxEqual(res.Document.Total, want) {
		t.Fatalf("want document total %g, got %g", want, res.Document.Total)
	}
	if res.Document.Currency != "EUR" {
		t.Fatalf("want currency EUR, got %q", res.Document.Currency)
	}
}

// TestParseTXTTotalNotOverwrittenByFallback locks in the bug fix where the key/value fallback used
// to call strconv.ParseFloat on "406 EUR", get 0, and clobber the correct doc.Total.
func TestParseTXTTotalNotOverwrittenByFallback(t *testing.T) {
	input := "Invoice TXT-7\nTotal: 406 EUR\n"
	res, err := parseTXTDocument(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if !approxEqual(res.Document.Total, 406) {
		t.Fatalf("Total must survive fallback parse, want 406, got %g", res.Document.Total)
	}
	if res.Document.Currency != "EUR" {
		t.Fatalf("Currency must be set from same line, got %q", res.Document.Currency)
	}
	if res.Document.DocumentNumber != "TXT-7" {
		t.Fatalf("DocumentNumber must come from title line, got %q", res.Document.DocumentNumber)
	}
}

// TestParseTableByPositions covers OCR/PDF style headers that use ambiguous column names (RATE +
// PRICE both look like "unit price" to alias mapping); the positional fallback assigns the three
// numeric columns to qty/unit/total left-to-right.
func TestParseTableByPositions(t *testing.T) {
	input := strings.Join([]string{
		"INVOICE",
		"INVOICE NO. INV-2024-1",
		"Total Due: $3960.00",
		"CATEGORY    RATE    QUANTITY    PRICE",
		"Flyer Design    300    3    900",
		"Business Card    200    3    600",
		"Logo Design    400    2    800",
	}, "\n")
	res, err := parseTXTDocument(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if got := len(res.Document.LineItems); got != 3 {
		t.Fatalf("want 3 positional line items, got %d", got)
	}
	first := res.Document.LineItems[0]
	if first.Description != "Flyer Design" || !approxEqual(first.Quantity, 3) ||
		!approxEqual(first.UnitPrice, 300) || !approxEqual(first.LineTotal, 900) {
		t.Fatalf("first item mismatched, got %+v", first)
	}
	if res.Document.DocumentType != "invoice" {
		t.Fatalf("DocumentType should be inferred from text, got %q", res.Document.DocumentType)
	}
	if res.Document.DocumentNumber != "INV-2024-1" {
		t.Fatalf("DocumentNumber should be picked up by regex, got %q", res.Document.DocumentNumber)
	}
	if !approxEqual(res.Document.Total, 3960) {
		t.Fatalf("Total Due value should populate doc.Total, got %g", res.Document.Total)
	}
	if res.Document.Currency != "USD" {
		t.Fatalf("$ symbol should map to USD, got %q", res.Document.Currency)
	}
}

func TestExtractFloatTolerant(t *testing.T) {
	cases := []struct {
		in    string
		want  float64
		hasOK bool
	}{
		{"$3,960.00", 3960, true},
		{"1.234,56", 1234.56, true},
		{"EUR 406", 406, true},
		{"406,00 €", 406, true},
		{"", 0, false},
		{"no number here", 0, false},
		{"-12.5", -12.5, true},
	}
	for _, tc := range cases {
		got, ok := extractFloat(tc.in)
		if ok != tc.hasOK {
			t.Fatalf("ok mismatch for %q: want %v got %v", tc.in, tc.hasOK, ok)
		}
		if ok && !approxEqual(got, tc.want) {
			t.Fatalf("value mismatch for %q: want %g got %g", tc.in, tc.want, got)
		}
	}
}

func TestDetectCurrency(t *testing.T) {
	cases := map[string]string{
		"$3960.00":           "USD",
		"406 EUR":            "EUR",
		"12,50€":             "EUR",
		"GBP 100":            "GBP",
		"unspecified amount": "",
	}
	for in, want := range cases {
		if got := detectCurrency(in); got != want {
			t.Fatalf("detectCurrency(%q) want %q got %q", in, want, got)
		}
	}
}

// PDF/OCR pipelines often flatten "From: ACME  Number: INV-1  Date: 2024-01-01" into a single line.
// The parser must split it on inline labels so each field reaches the right document property
// instead of being absorbed into the first label's value.
func TestParseTXTSplitsInlineLabels(t *testing.T) {
	txt := "From: ACME Corp  Number: INV-9001  Date: 2024-05-01\nTotal: 250.00 EUR\n"
	res, err := parseTXTDocument(strings.NewReader(txt))
	if err != nil {
		t.Fatal(err)
	}
	if res.Document.SupplierName != "ACME Corp" {
		t.Fatalf("supplier_name want %q got %q", "ACME Corp", res.Document.SupplierName)
	}
	if res.Document.DocumentNumber != "INV-9001" {
		t.Fatalf("document_number want %q got %q", "INV-9001", res.Document.DocumentNumber)
	}
	if res.Document.IssueDate == nil {
		t.Fatalf("issue_date should be parsed from Date: 2024-05-01")
	}
	if !approxEqual(res.Document.Total, 250) {
		t.Fatalf("total want 250 got %g", res.Document.Total)
	}
}

// Compound labels like "Total Due" must survive inline splitting — otherwise OCR/PDF text that
// glues several "Label: value" pairs onto one line would orphan the first half of the label.
func TestParseTXTKeepsCompoundLabels(t *testing.T) {
	txt := "Total Due: $3960.00 Currency: USD\n"
	res, err := parseTXTDocument(strings.NewReader(txt))
	if err != nil {
		t.Fatal(err)
	}
	if !approxEqual(res.Document.Total, 3960) {
		t.Fatalf("Total Due want 3960 got %g", res.Document.Total)
	}
	if res.Document.Currency != "USD" {
		t.Fatalf("Currency want USD got %q", res.Document.Currency)
	}
}

// PDF text extraction often splits "Flyer Design" into two text spans, so a body row ends up
// with one MORE column than the 4-column header. The positional detector must still recover
// description="Flyer Design" by treating everything before the trailing (qty, unit, total)
// numeric trio as the description.
func TestParseTableMultiWordDescriptionExtraColumns(t *testing.T) {
	input := strings.Join([]string{
		"Description    Qty    Unit Price    Total",
		"Flyer Design    3    300    900",
		"Logo Pack    2    150    300",
	}, "\n")
	res, err := parseTXTDocument(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if got := len(res.Document.LineItems); got != 2 {
		t.Fatalf("want 2 line items, got %d", got)
	}
	if first := res.Document.LineItems[0]; first.Description != "Flyer Design" ||
		!approxEqual(first.Quantity, 3) || !approxEqual(first.UnitPrice, 300) ||
		!approxEqual(first.LineTotal, 900) {
		t.Fatalf("first item mismatched, got %+v", first)
	}
	if !approxEqual(res.Document.Total, 1200) {
		t.Fatalf("Total should sum line totals, want 1200 got %g", res.Document.Total)
	}
}

// Headerless fallback: when no row reads as a clean 4-column header, we still want to recover
// line items from rows that end in three numerics where qty * unit ≈ total. This guards against
// real-world PDFs whose extracted text has no recognisable column titles.
func TestParseTableHeaderlessFallback(t *testing.T) {
	input := strings.Join([]string{
		"Some preamble text",
		"Date 2024-01-01",
		"Widget A    2    50    100",
		"Widget B    3    25    75",
		"Subtotal: 175",
	}, "\n")
	res, err := parseTXTDocument(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if got := len(res.Document.LineItems); got != 2 {
		t.Fatalf("want 2 fallback line items, got %d", got)
	}
	if got := res.Document.LineItems[1]; got.Description != "Widget B" || !approxEqual(got.LineTotal, 75) {
		t.Fatalf("second item mismatched, got %+v", got)
	}
}

// Random number triplets must NOT be promoted to line items: if qty*unit doesn't match total
// for the majority of candidates, the trailing-numerics detector returns nothing.
func TestParseTableHeaderlessRejectsRandomTriplets(t *testing.T) {
	input := strings.Join([]string{
		"Address Line 1 23 5",
		"Phone 1 800 555",
		"Random 7 11 13",
	}, "\n")
	res, err := parseTXTDocument(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if got := len(res.Document.LineItems); got != 0 {
		t.Fatalf("random triplets must not become line items, got %d", got)
	}
}

// A line whose first non-colon row is a table header (e.g. "Description Qty Unit Price Total")
// must NOT be promoted to "<type> <number>" — that's how PDFs ended up with
// document_type="description" / document_number="qty".
func TestParseTXTTitleSkipsTableHeader(t *testing.T) {
	txt := "Description Qty Unit Price Total\nType: invoice\nNumber: INV-42\nWidget 2 10 20\n"
	res, err := parseTXTDocument(strings.NewReader(txt))
	if err != nil {
		t.Fatal(err)
	}
	if res.Document.DocumentType != "invoice" {
		t.Fatalf("document_type want invoice got %q", res.Document.DocumentType)
	}
	if res.Document.DocumentNumber != "INV-42" {
		t.Fatalf("document_number want INV-42 got %q", res.Document.DocumentNumber)
	}
}
