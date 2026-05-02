package document

import (
	"testing"
	"time"
)

func TestValidateDocument_DateRange_DueBeforeIssue(t *testing.T) {
	issue := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	due := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	doc := Document{
		DocumentType:   "invoice",
		SupplierName:   "ACME",
		DocumentNumber: "INV-1",
		IssueDate:      &issue,
		DueDate:        &due,
	}

	issues := ValidateDocument(doc)
	if !issueCodesContain(issues, IssueCodeInvalidDateRange) {
		t.Fatalf("expected code %q among issues, got %#v", IssueCodeInvalidDateRange, issues)
	}
}

func TestValidateDocument_DateRange_SameDay_NoInvalidRange(t *testing.T) {
	d := time.Date(2026, 5, 10, 8, 0, 0, 0, time.UTC)
	doc := Document{
		DocumentType:   "invoice",
		SupplierName:   "ACME",
		DocumentNumber: "INV-2",
		IssueDate:      &d,
		DueDate:        &d,
	}

	issues := ValidateDocument(doc)
	if issueCodesContain(issues, IssueCodeInvalidDateRange) {
		t.Fatalf("did not expect %q, got %#v", IssueCodeInvalidDateRange, issues)
	}
}

func TestValidateDocument_DateRange_DueAfterIssue_NoInvalidRange(t *testing.T) {
	issue := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	due := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)
	doc := Document{
		DocumentType:   "invoice",
		SupplierName:   "ACME",
		DocumentNumber: "INV-3",
		IssueDate:      &issue,
		DueDate:        &due,
	}

	issues := ValidateDocument(doc)
	if issueCodesContain(issues, IssueCodeInvalidDateRange) {
		t.Fatalf("did not expect %q, got %#v", IssueCodeInvalidDateRange, issues)
	}
}

func TestValidateDocument_MissingSupplier(t *testing.T) {
	doc := Document{
		DocumentType:   "invoice",
		SupplierName:   "",
		DocumentNumber: "X-1",
	}

	issues := ValidateDocument(doc)
	if !issueCodesContain(issues, IssueCodeMissingField) {
		t.Fatalf("expected code %q among issues, got %#v", IssueCodeMissingField, issues)
	}
}

func TestValidateDocument_InvalidLineTotal(t *testing.T) {
	doc := Document{
		DocumentType:   "invoice",
		SupplierName:   "ACME",
		DocumentNumber: "INV-4",
		LineItems: []LineItem{
			{Description: "Item", Quantity: 2, UnitPrice: 10, LineTotal: 19},
		},
	}

	issues := ValidateDocument(doc)
	if !issueCodesContain(issues, IssueCodeInvalidLineTotal) {
		t.Fatalf("expected code %q among issues, got %#v", IssueCodeInvalidLineTotal, issues)
	}
}

func issueCodesContain(issues []ValidationIssue, code string) bool {
	for _, i := range issues {
		if i.Code == code {
			return true
		}
	}
	return false
}
