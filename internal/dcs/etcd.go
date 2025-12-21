package dcs

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

// EtcdDCS implements DCS interface using etcd
type EtcdDCS struct {
	client    *clientv3.Client
	session   *concurrency.Session
	mutex     *concurrency.Mutex
	config    EtcdConfig
	connected bool
	leaseID   clientv3.LeaseID
	prefix    string
	mu        sync.RWMutex
}

// EtcdConfig holds etcd connection configuration
type EtcdConfig struct {
	Endpoints   []string
	Username    string
	Password    string
	DialTimeout time.Duration
	Prefix      string
}

// NewEtcdDCS creates a new etcd DCS instance
func NewEtcdDCS(config EtcdConfig) *EtcdDCS {
	if config.DialTimeout == 0 {
		config.DialTimeout = 5 * time.Second
	}
	if config.Prefix == "" {
		config.Prefix = "/mysql-ha"
	}
	return &EtcdDCS{
		config: config,
		prefix: config.Prefix,
	}
}

// Connect establishes connection to etcd
func (e *EtcdDCS) Connect(ctx context.Context) error {
	cfg := clientv3.Config{
		Endpoints:   e.config.Endpoints,
		DialTimeout: e.config.DialTimeout,
		Username:    e.config.Username,
		Password:    e.config.Password,
	}

	client, err := clientv3.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to etcd: %w", err)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(ctx, e.config.DialTimeout)
	defer cancel()

	_, err = client.Status(ctx, e.config.Endpoints[0])
	if err != nil {
		client.Close()
		return fmt.Errorf("failed to get etcd status: %w", err)
	}

	e.mu.Lock()
	e.client = client
	e.connected = true
	e.mu.Unlock()

	return nil
}

// Close closes the etcd connection
func (e *EtcdDCS) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.session != nil {
		e.session.Close()
	}
	if e.client != nil {
		e.connected = false
		return e.client.Close()
	}
	return nil
}

// IsConnected returns true if connected to etcd
func (e *EtcdDCS) IsConnected() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.connected
}

// AcquireLock attempts to acquire a distributed lock
func (e *EtcdDCS) AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.client == nil {
		return false, fmt.Errorf("not connected to etcd")
	}

	// Create session with TTL
	session, err := concurrency.NewSession(e.client, concurrency.WithTTL(int(ttl.Seconds())))
	if err != nil {
		return false, fmt.Errorf("failed to create session: %w", err)
	}

	mutex := concurrency.NewMutex(session, e.prefix+"/leader/"+key)

	// Try to acquire lock with timeout
	err = mutex.TryLock(ctx)
	if err != nil {
		session.Close()
		if err == concurrency.ErrLocked {
			return false, nil
		}
		return false, fmt.Errorf("failed to acquire lock: %w", err)
	}

	e.session = session
	e.mutex = mutex
	return true, nil
}

// RenewLock renews the lock lease
func (e *EtcdDCS) RenewLock(ctx context.Context, key string) error {
	e.mu.RLock()
	session := e.session
	e.mu.RUnlock()

	if session == nil {
		return fmt.Errorf("no active session")
	}

	// Session automatically keeps alive, but we can manually refresh
	_, err := e.client.KeepAliveOnce(ctx, session.Lease())
	if err != nil {
		return fmt.Errorf("failed to renew lock: %w", err)
	}
	return nil
}

// ReleaseLock releases the distributed lock
func (e *EtcdDCS) ReleaseLock(ctx context.Context, key string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.mutex != nil {
		if err := e.mutex.Unlock(ctx); err != nil {
			return fmt.Errorf("failed to release lock: %w", err)
		}
		e.mutex = nil
	}

	if e.session != nil {
		e.session.Close()
		e.session = nil
	}

	return nil
}

// GetLeader returns information about the current leader
func (e *EtcdDCS) GetLeader(ctx context.Context, key string) (*LeaderInfo, error) {
	data, err := e.Get(ctx, e.prefix+"/leader/"+key+"/info")
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}

	var info LeaderInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("failed to unmarshal leader info: %w", err)
	}
	return &info, nil
}

// Set stores a value in etcd
func (e *EtcdDCS) Set(ctx context.Context, key string, value []byte) error {
	if e.client == nil {
		return fmt.Errorf("not connected to etcd")
	}

	// Ensure key starts with / for proper path joining
	fullKey := e.prefix
	if key != "" {
		if key[0] != '/' {
			fullKey = fullKey + "/" + key
		} else {
			fullKey = fullKey + key
		}
	}

	_, err := e.client.Put(ctx, fullKey, string(value))
	if err != nil {
		return fmt.Errorf("failed to set key %s: %w", fullKey, err)
	}
	return nil
}

// Get retrieves a value from etcd
func (e *EtcdDCS) Get(ctx context.Context, key string) ([]byte, error) {
	if e.client == nil {
		return nil, fmt.Errorf("not connected to etcd")
	}

	// Ensure key starts with / for proper path joining
	fullKey := e.prefix
	if key != "" {
		if key[0] != '/' {
			fullKey = fullKey + "/" + key
		} else {
			fullKey = fullKey + key
		}
	}

	resp, err := e.client.Get(ctx, fullKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get key %s: %w", fullKey, err)
	}

	if len(resp.Kvs) == 0 {
		return nil, nil
	}

	return resp.Kvs[0].Value, nil
}

// Delete removes a key from etcd
func (e *EtcdDCS) Delete(ctx context.Context, key string) error {
	if e.client == nil {
		return fmt.Errorf("not connected to etcd")
	}

	_, err := e.client.Delete(ctx, e.prefix+key)
	if err != nil {
		return fmt.Errorf("failed to delete key: %w", err)
	}
	return nil
}

// Watch watches for changes to a key
func (e *EtcdDCS) Watch(ctx context.Context, key string) (<-chan WatchEvent, error) {
	if e.client == nil {
		return nil, fmt.Errorf("not connected to etcd")
	}

	events := make(chan WatchEvent, 10)
	watchChan := e.client.Watch(ctx, e.prefix+key, clientv3.WithPrefix())

	go func() {
		defer close(events)
		for {
			select {
			case <-ctx.Done():
				return
			case resp, ok := <-watchChan:
				if !ok {
					return
				}
				if resp.Err() != nil {
					events <- WatchEvent{Type: EventError, Error: resp.Err()}
					continue
				}
				for _, ev := range resp.Events {
					event := WatchEvent{
						Key:   string(ev.Kv.Key),
						Value: ev.Kv.Value,
					}
					switch ev.Type {
					case clientv3.EventTypePut:
						event.Type = EventPut
					case clientv3.EventTypeDelete:
						event.Type = EventDelete
					}
					events <- event
				}
			}
		}
	}()

	return events, nil
}

// RegisterMember registers a cluster member
func (e *EtcdDCS) RegisterMember(ctx context.Context, member *Member) error {
	data, err := json.Marshal(member)
	if err != nil {
		return fmt.Errorf("failed to marshal member: %w", err)
	}
	return e.Set(ctx, "/members/"+member.NodeID, data)
}

// GetMembers returns all cluster members
func (e *EtcdDCS) GetMembers(ctx context.Context) ([]*Member, error) {
	if e.client == nil {
		return nil, fmt.Errorf("not connected to etcd")
	}

	resp, err := e.client.Get(ctx, e.prefix+"/members/", clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("failed to get members: %w", err)
	}

	var members []*Member
	for _, kv := range resp.Kvs {
		var member Member
		if err := json.Unmarshal(kv.Value, &member); err != nil {
			continue
		}
		members = append(members, &member)
	}

	return members, nil
}

// UpdateMember updates a cluster member's information
func (e *EtcdDCS) UpdateMember(ctx context.Context, member *Member) error {
	return e.RegisterMember(ctx, member)
}
