package entity

import (
	"context"
	"time"
)

// BlockedIP represents an IP or CIDR range to block
type BlockedIP struct {
	ID        int
	IP        string // single IP (192.168.1.1) or CIDR (192.168.0.0/24)
	Comment   string // free-text note, e.g. why this was blocked
	CreatedAt time.Time
}

// BlockedIPRepository defines the interface for blocked IP data access
type BlockedIPRepository interface {
	List(ctx context.Context) ([]BlockedIP, error)
	Create(ctx context.Context, ip, comment string) (*BlockedIP, error)
	Delete(ctx context.Context, id int) error
}
