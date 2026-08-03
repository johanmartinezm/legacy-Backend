package domain

import (
	"time"

	"github.com/google/uuid"
)

type TransactionStatus string

const (
	TxStatusPending  TransactionStatus = "PENDING"
	TxStatusApproved TransactionStatus = "APPROVED"
	TxStatusDeclined TransactionStatus = "DECLINED"
	TxStatusFailed   TransactionStatus = "FAILED"
)

type ReferenceType string

const (
	RefTypeEvent ReferenceType = "EVENT"
	RefTypeCart  ReferenceType = "CART"
)

type Transaction struct {
	ID                uuid.UUID         `json:"id"`
	UserID            uuid.UUID         `json:"user_id"`
	ReferenceType     ReferenceType     `json:"reference_type"`
	ReferenceID       uuid.UUID         `json:"reference_id"`
	Amount            float64           `json:"amount"`
	CredibancoOrderID string            `json:"credibanco_order_id,omitempty"`
	Status            TransactionStatus `json:"status"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
}
