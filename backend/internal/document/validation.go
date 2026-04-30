package document
//helper functions for validation

import (
	"math"
	"strings"
)

const (
	IssueCodeMissingField      = "MISSING_FIELD"
	IssueCodeInvalidLineTotal  = "INVALID_LINE_TOTAL"
	IssueSeverityError         = "error"
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

func moneyEquals(a, b float64) bool {
	return math.Abs(a-b) <= DefaultMoneyEpsilon
}
