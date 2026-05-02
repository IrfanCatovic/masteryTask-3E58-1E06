package document

import "time"


type Document struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	DocumentType   string    `gorm:"size:30;not null" json:"document_type"`
	SupplierName   string    `gorm:"size:255;not null" json:"supplier_name"`
	DocumentNumber string    `gorm:"size:100;not null;index" json:"document_number"`
	Status         string    `gorm:"size:30;not null;default:uploaded" json:"status"`
	IssueDate      *time.Time `gorm:"type:date" json:"issue_date,omitempty"`
	DueDate        *time.Time `gorm:"type:date" json:"due_date,omitempty"`
	Currency       string    `gorm:"size:10" json:"currency,omitempty"`
	Subtotal       float64   `gorm:"type:numeric(12,2);default:0" json:"subtotal"`
	TaxRate        float64   `gorm:"type:numeric(8,4);default:0" json:"tax_rate"`
	DiscountRate   float64   `gorm:"type:numeric(8,4);default:0" json:"discount_rate"`
	Total          float64   `gorm:"type:numeric(12,2);default:0" json:"total"`
	LineItems      []LineItem       `json:"line_items,omitempty"`
	Issues         []ValidationIssue `json:"issues,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// LineItem represents one row from an invoice/PO (description, qty, price).
type LineItem struct {
	ID          uint    `gorm:"primaryKey" json:"id"`
	DocumentID  uint    `gorm:"not null;index" json:"document_id"`
	Description string  `gorm:"size:255;not null" json:"description"`
	Quantity    float64 `gorm:"type:numeric(12,3);not null" json:"quantity"`
	UnitPrice   float64 `gorm:"type:numeric(12,2);not null" json:"unit_price"`
	LineTotal   float64 `gorm:"type:numeric(12,2);not null" json:"line_total"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ValidationIssue stores one detected problem for a document.
type ValidationIssue struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	DocumentID uint      `gorm:"not null;index" json:"document_id"`
	Code       string    `gorm:"size:50;not null;index" json:"code"`
	Message    string    `gorm:"type:text;not null" json:"message"`
	Severity   string    `gorm:"size:20;not null;default:warning" json:"severity"`
	FieldName  string    `gorm:"size:100" json:"field_name,omitempty"`
	Resolved   bool      `gorm:"not null;default:false" json:"resolved"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
