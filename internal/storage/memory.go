package storage

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	ErrUserAlreadyExists = errors.New("user already exists")
	ErrUserNotFound      = errors.New("user not found")
)

type User struct {
	ID   uuid.UUID
	Name string
}

type Transaction struct {
	TransactionID uuid.UUID
	UserID        uuid.UUID
	Amount        float64
	Timestamp     time.Time
	Status        string
}

type UserRepo interface {
	CreateUser(User) error
	GetUser(uuid.UUID) (User, error)
}

type TxRepo interface {
	UpsertTx(Transaction) error
	ListTx() ([]Transaction, error)
}

// MemoryStore implementa UserRepo e TxRepo
type MemoryStore struct {
	mu    sync.RWMutex
	users map[uuid.UUID]User
	txs   map[uuid.UUID]Transaction
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		users: make(map[uuid.UUID]User),
		txs:   make(map[uuid.UUID]Transaction),
	}
}

func (s *MemoryStore) CreateUser(u User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[u.ID]; ok {
		return ErrUserAlreadyExists
	}
	s.users[u.ID] = u
	return nil
}

func (s *MemoryStore) GetUser(id uuid.UUID) (User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[id]
	if !ok {
		return User{}, ErrUserNotFound
	}
	return u, nil
}

func (s *MemoryStore) UpsertTx(t Transaction) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.txs[t.TransactionID] = t
	return nil
}

func (s *MemoryStore) ListTx() ([]Transaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Transaction, 0, len(s.txs))
	for _, t := range s.txs {
		out = append(out, t)
	}
	return out, nil
}
