// Package dcs provides distributed configuration store interface and implementations
package dcs

import (
	"context"
	"time"
)

// EventType represents the type of watch event
type EventType int

const (
	EventPut EventType = iota
	EventDelete
	EventError
)

// WatchEvent represents an event from watching a key
type WatchEvent struct {
	Type  EventType
	Key   string
	Value []byte
	Error error
}

// LeaderInfo contains information about the current leader
type LeaderInfo struct {
	NodeID    string    `json:"node_id"`
	Hostname  string    `json:"hostname"`
	Timestamp time.Time `json:"timestamp"`
}

// Member represents a cluster member
type Member struct {
	NodeID   string `json:"node_id"`
	Hostname string `json:"hostname"`
	APIAddr  string `json:"api_addr"`
	Role     string `json:"role"`
}

// DCS defines the interface for distributed configuration store
type DCS interface {
	// Connection management
	Connect(ctx context.Context) error
	Close() error
	IsConnected() bool

	// Leader election
	AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error)
	RenewLock(ctx context.Context, key string) error
	ReleaseLock(ctx context.Context, key string) error
	GetLeader(ctx context.Context, key string) (*LeaderInfo, error)

	// State storage
	Set(ctx context.Context, key string, value []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
	Watch(ctx context.Context, key string) (<-chan WatchEvent, error)

	// Cluster members
	RegisterMember(ctx context.Context, member *Member) error
	GetMembers(ctx context.Context) ([]*Member, error)
	UpdateMember(ctx context.Context, member *Member) error
}

// Backoff calculates exponential backoff interval
type Backoff struct {
	BaseInterval time.Duration
	MaxInterval  time.Duration
	attempt      int
}

// NewBackoff creates a new Backoff with default values
func NewBackoff() *Backoff {
	return &Backoff{
		BaseInterval: time.Second,
		MaxInterval:  30 * time.Second,
		attempt:      0,
	}
}

// Next returns the next backoff interval
func (b *Backoff) Next() time.Duration {
	interval := b.BaseInterval * (1 << b.attempt)
	if interval > b.MaxInterval {
		interval = b.MaxInterval
	}
	b.attempt++
	return interval
}

// Reset resets the backoff counter
func (b *Backoff) Reset() {
	b.attempt = 0
}

// CalculateBackoff calculates the backoff interval for a given attempt
func CalculateBackoff(attempt int, baseInterval, maxInterval time.Duration) time.Duration {
	interval := baseInterval * (1 << attempt)
	if interval > maxInterval {
		return maxInterval
	}
	return interval
}
