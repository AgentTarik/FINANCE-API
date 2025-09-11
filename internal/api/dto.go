package api

import "time"

// Entrada para criar usuário
type CreateUserRequest struct {
	ID   string `json:"id"   validate:"required,uuid4"`        // UUID v4
	Name string `json:"name" validate:"required,min=4,max=80"` // nome
}

// Resposta de usuário
type UserResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Entrada para criar transação
type CreateTransactionRequest struct {
	TransactionID string  `json:"transaction_id" validate:"required,uuid4"`                              // UUID v4
	UserID        string  `json:"user_id"        validate:"required,uuid4"`                              // UUID v4
	Amount        float64 `json:"amount"         validate:"required"`                                    // valor
	Timestamp     string  `json:"timestamp"      validate:"required,datetime=2006-01-02T15:04:05Z07:00"` // RFC3339
}

// Saída de transação
type Transaction struct {
	TransactionID string    `json:"transaction_id"`
	UserID        string    `json:"user_id"`
	Amount        float64   `json:"amount"`
	Timestamp     time.Time `json:"timestamp"`
	Status        string    `json:"status"` // queued | processed | failed
}
