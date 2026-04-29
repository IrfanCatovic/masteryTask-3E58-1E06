package document

import "time"


type Document struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	DocumentType   string    `gorm:"size:30;not null" json:"document_type"`
	SupplierName   string    `gorm:"size:255;not null" json:"supplier_name"`
	DocumentNumber string    `gorm:"size:100;not null;index" json:"document_number"`
	Status         string    `gorm:"size:30;not null;default:uploaded" json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
