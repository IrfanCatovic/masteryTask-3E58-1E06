package document
//helper functions for validation

import (
	"math"
	"strings"
	"time"
)

const (
	IssueCodeMissingField      = "MISSING_FIELD"
	IssueCodeInvalidLineTotal  = "INVALID_LINE_TOTAL"
	IssueCodeInvalidTotal      = "INVALID_TOTAL"
	IssueCodeMissingCurrency    = "MISSING_CURRENCY"
	IssueCodeInvalidDateRange   = "INVALID_DATE_RANGE"
	IssueCodeInvalidDate        = "INVALID_DATE"
	IssueCodeImageOCREmpty     = "IMAGE_OCR_EMPTY"
	IssueSeverityError          = "error"
	IssueSeverityWarning        = "warning"
	DefaultMoneyEpsilon float64 = 0.01
)

// ValidateDocument checks core business rules and returns detected issues.
// For now we validate required fields and line-item math.
func ValidateDocument(doc Document) []ValidationIssue {
	issues := make([]ValidationIssue, 0)

	// Required document fields.
	if strings.TrimSpace(doc.DocumentType) == "" {
		issues = append(issues, newMissingFieldIssue("document_type", "document type is required"))
	}
	if strings.TrimSpace(doc.SupplierName) == "" {
		issues = append(issues, newMissingFieldIssue("supplier_name", "supplier name is required"))
	}
	if strings.TrimSpace(doc.DocumentNumber) == "" {
		issues = append(issues, newMissingFieldIssue("document_number", "document number is required"))
	}

	// Validate each line item's calculation.
	for _, item := range doc.LineItems {
		expected := item.Quantity * item.UnitPrice
		if !moneyEquals(expected, item.LineTotal) {
			issues = append(issues, ValidationIssue{
				DocumentID: doc.ID,
				Code:       IssueCodeInvalidLineTotal,
				Message:    "line total does not match quantity * unit_price",
				Severity:   IssueSeverityError,
				FieldName:  "line_total",
				Resolved:   false,
			})
		}
	}

	// Financial consistency checks.
	if doc.Total > 0 && strings.TrimSpace(doc.Currency) == "" {
		issues = append(issues, ValidationIssue{
			DocumentID: doc.ID,
			Code:       IssueCodeMissingCurrency,
			Message:    "currency is required when total is present",
			Severity:   IssueSeverityError,
			FieldName:  "currency",
			Resolved:   false,
		})
	}

	if doc.Subtotal != 0 || doc.TaxRate != 0 || doc.DiscountRate != 0 {
		discountAmount := doc.Subtotal * (doc.DiscountRate / 100.0)
		taxable := doc.Subtotal - discountAmount
		taxAmount := taxable * (doc.TaxRate / 100.0)
		expectedTotal := taxable + taxAmount
		if !moneyEquals(expectedTotal, doc.Total) {
			issues = append(issues, ValidationIssue{
				DocumentID: doc.ID,
				Code:       IssueCodeInvalidTotal,
				Message:    "total does not match subtotal/tax_rate/discount_rate calculation",
				Severity:   IssueSeverityError,
				FieldName:  "total",
				Resolved:   false,
			})
		}
	}

	// Date rules: when both are set, due must not be before issue (calendar day).
	if doc.IssueDate != nil && doc.DueDate != nil {
		iy, im, id := doc.IssueDate.Date()
		dy, dm, dd := doc.DueDate.Date()
		issueDay := time.Date(iy, im, id, 0, 0, 0, 0, time.UTC)
		dueDay := time.Date(dy, dm, dd, 0, 0, 0, 0, time.UTC)
		if dueDay.Before(issueDay) {
			issues = append(issues, ValidationIssue{
				DocumentID: doc.ID,
				Code:       IssueCodeInvalidDateRange,
				Message:    "due_date must be on or after issue_date",
				Severity:   IssueSeverityError,
				FieldName:  "due_date",
				Resolved:   false,
			})
		}
	}

	return issues
}

func newMissingFieldIssue(fieldName, message string) ValidationIssue {
	return ValidationIssue{
		Code:      IssueCodeMissingField,
		Message:   message,
		Severity:  IssueSeverityError,
		FieldName: fieldName,
		Resolved:  false,
	}
}

// newInvalidDateIssue is used when a raw date value can't be parsed as YYYY-MM-DD
// (or is otherwise malformed). The bad value is preserved in the message so a
// reviewer can see what was originally uploaded and correct it.
func newInvalidDateIssue(fieldName, rawValue string) ValidationIssue {
	return ValidationIssue{
		Code:      IssueCodeInvalidDate,
		Message:   fieldName + " is not a valid date (expected YYYY-MM-DD), got: " + rawValue,
		Severity:  IssueSeverityError,
		FieldName: fieldName,
		Resolved:  false,
	}
}

func moneyEquals(a, b float64) bool {
	return math.Abs(a-b) <= DefaultMoneyEpsilon //ovo je ustvari epsilon za float64, ako je razlika manja od epsilon, smatra se da su jednaki
	//a epsilon je 0.01, ako je razlika manja od 0.01, smatra se da su jednaki
}
