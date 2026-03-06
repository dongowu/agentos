// Package redis provides a memory.Provider backed by Redis using a minimal RESP client.
package redis

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/dongowu/agentos/internal/memory"
)

// Redis implements memory.Provider using the Redis protocol.
type Redis struct {
	mu     sync.Mutex
	conn   net.Conn
	rd     *bufio.Reader
	prefix string
}

// Option configures a Redis provider.
type Option func(*Redis)

// WithPrefix sets a key prefix for all operations.
func WithPrefix(p string) Option {
	return func(r *Redis) {
		r.prefix = p
	}
}

// New connects to a Redis server at addr and returns a provider.
func New(addr string, opts ...Option) (*Redis, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("redis dial: %w", err)
	}
	r := &Redis{
		conn: conn,
		rd:   bufio.NewReader(conn),
	}
	for _, o := range opts {
		o(r)
	}
	return r, nil
}

// Close closes the underlying connection.
func (r *Redis) Close() error {
	return r.conn.Close()
}

// Put stores a value with SET.
func (r *Redis) Put(_ context.Context, key string, value []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.doSimple("SET", r.prefix+key, string(value))
}

// Get retrieves a value with GET. Returns an error if the key does not exist.
func (r *Redis) Get(_ context.Context, key string) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	reply, err := r.doBulk("GET", r.prefix+key)
	if err != nil {
		return nil, err
	}
	if reply == nil {
		return nil, errors.New("key not found: " + key)
	}
	return reply, nil
}

// Search lists keys matching prefix*, fetches values, and filters by substring.
func (r *Redis) Search(_ context.Context, query string, k int) ([]memory.SearchResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	keys, err := r.doArray("KEYS", r.prefix+"*")
	if err != nil {
		return nil, fmt.Errorf("redis KEYS: %w", err)
	}

	queryLower := strings.ToLower(query)
	var results []memory.SearchResult
	for _, key := range keys {
		val, err := r.doBulk("GET", key)
		if err != nil || val == nil {
			continue
		}
		if strings.Contains(strings.ToLower(string(val)), queryLower) {
			score := float64(len(query)) / float64(len(val))
			if score > 1 {
				score = 1
			}
			results = append(results, memory.SearchResult{
				Key:     key,
				Content: val,
				Score:   score,
			})
		}
		if len(results) >= k {
			break
		}
	}
	return results, nil
}

// --- Minimal RESP protocol ---

// writeCommand writes a RESP array command.
func (r *Redis) writeCommand(args ...string) error {
	cmd := fmt.Sprintf("*%d\r\n", len(args))
	for _, a := range args {
		cmd += fmt.Sprintf("$%d\r\n%s\r\n", len(a), a)
	}
	_, err := r.conn.Write([]byte(cmd))
	return err
}

// readLine reads a single RESP line.
func (r *Redis) readLine() (string, error) {
	line, err := r.rd.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// doSimple sends a command and expects a +OK or similar simple reply.
func (r *Redis) doSimple(args ...string) error {
	if err := r.writeCommand(args...); err != nil {
		return err
	}
	line, err := r.readLine()
	if err != nil {
		return err
	}
	if len(line) > 0 && line[0] == '-' {
		return errors.New(line[1:])
	}
	return nil
}

// doBulk sends a command and reads a bulk string reply. Returns nil for $-1.
func (r *Redis) doBulk(args ...string) ([]byte, error) {
	if err := r.writeCommand(args...); err != nil {
		return nil, err
	}
	line, err := r.readLine()
	if err != nil {
		return nil, err
	}
	if len(line) > 0 && line[0] == '-' {
		return nil, errors.New(line[1:])
	}
	if line == "$-1" {
		return nil, nil
	}
	// Read the bulk data line.
	data, err := r.readLine()
	if err != nil {
		return nil, err
	}
	return []byte(data), nil
}

// doArray sends a command and reads an array of bulk strings.
func (r *Redis) doArray(args ...string) ([]string, error) {
	if err := r.writeCommand(args...); err != nil {
		return nil, err
	}
	line, err := r.readLine()
	if err != nil {
		return nil, err
	}
	if len(line) > 0 && line[0] == '-' {
		return nil, errors.New(line[1:])
	}

	var count int
	fmt.Sscanf(line, "*%d", &count)
	result := make([]string, 0, count)
	for i := 0; i < count; i++ {
		// Read $N line.
		if _, err := r.readLine(); err != nil {
			return nil, err
		}
		// Read data line.
		data, err := r.readLine()
		if err != nil {
			return nil, err
		}
		result = append(result, data)
	}
	return result, nil
}
