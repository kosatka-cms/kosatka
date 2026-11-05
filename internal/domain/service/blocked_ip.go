package service

import (
	"context"
	"net"
	"strings"
	"sync"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// BlockedIPService provides IP blocking with in-memory cache.
type BlockedIPService interface {
	IsBlocked(clientIP string) bool
	List(ctx context.Context) ([]entity.BlockedIP, error)
	Create(ctx context.Context, ip, comment string) error
	Delete(ctx context.Context, id int) error
	RefreshCache(ctx context.Context) error
}

// DefaultBlockedIPService implements BlockedIPService.
type DefaultBlockedIPService struct {
	repo entity.BlockedIPRepository
	// cache - the blocked addresses and networks, parsed when loaded:
	// IsBlocked runs on every request.
	cache []ipPattern
	mu    sync.RWMutex
}

// ipPattern - a blocked address (ip) or network (network).
type ipPattern struct {
	ip      net.IP
	network *net.IPNet
}

// parseIPPattern - pattern (an address, or a CIDR network) parsed; false -
// neither (it blocks nothing).
func parseIPPattern(pattern string) (ipPattern, bool) {
	if strings.Contains(pattern, "/") {
		_, network, err := net.ParseCIDR(pattern)
		return ipPattern{network: network}, err == nil
	}
	ip := net.ParseIP(pattern)
	return ipPattern{ip: ip}, ip != nil
}

func (p ipPattern) match(ip net.IP) bool {
	if p.network != nil {
		return p.network.Contains(ip)
	}
	return p.ip.Equal(ip)
}

// NewBlockedIPService creates a new BlockedIPService.
func NewBlockedIPService(repo entity.BlockedIPRepository) *DefaultBlockedIPService {
	return &DefaultBlockedIPService{
		repo:  repo,
		cache: nil,
	}
}

// IsBlocked returns true if the client IP matches any blocked IP or CIDR.
func (s *DefaultBlockedIPService) IsBlocked(clientIP string) bool {
	clientIP = strings.TrimSpace(clientIP)
	if clientIP == "" {
		return false
	}

	ip := net.ParseIP(clientIP)
	if ip == nil {
		return false
	}

	s.mu.RLock()
	patterns := s.cache
	s.mu.RUnlock()

	if len(patterns) == 0 {
		return false
	}

	for _, p := range patterns {
		if p.match(ip) {
			return true
		}
	}
	return false
}

// List returns all blocked IPs from the database.
func (s *DefaultBlockedIPService) List(ctx context.Context) ([]entity.BlockedIP, error) {
	return s.repo.List(ctx)
}

// Create adds a new IP and refreshes the cache.
func (s *DefaultBlockedIPService) Create(ctx context.Context, ip, comment string) error {
	_, err := s.repo.Create(ctx, ip, comment)
	if err != nil {
		return err
	}
	return s.RefreshCache(ctx)
}

// Delete removes an IP and refreshes the cache.
func (s *DefaultBlockedIPService) Delete(ctx context.Context, id int) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	return s.RefreshCache(ctx)
}

// RefreshCache reloads the blocked IPs from the database into the cache.
func (s *DefaultBlockedIPService) RefreshCache(ctx context.Context) error {
	list, err := s.repo.List(ctx)
	if err != nil {
		return err
	}
	patterns := make([]ipPattern, 0, len(list))
	for _, b := range list {
		if p, ok := parseIPPattern(strings.TrimSpace(b.IP)); ok {
			patterns = append(patterns, p)
		}
	}
	s.mu.Lock()
	s.cache = patterns
	s.mu.Unlock()
	return nil
}
