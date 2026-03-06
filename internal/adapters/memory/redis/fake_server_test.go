package redis

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
)

// fakeServer is a minimal Redis server for unit testing.
type fakeServer struct {
	ln    net.Listener
	mu    sync.Mutex
	store map[string]string
	done  chan struct{}
}

func newFakeServer(t *testing.T) *fakeServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("fake server listen: %v", err)
	}
	s := &fakeServer{
		ln:    ln,
		store: make(map[string]string),
		done:  make(chan struct{}),
	}
	go s.serve()
	return s
}

func (s *fakeServer) Addr() string { return s.ln.Addr().String() }

func (s *fakeServer) Close() {
	close(s.done)
	s.ln.Close()
}

func (s *fakeServer) serve() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *fakeServer) handle(conn net.Conn) {
	defer conn.Close()
	rd := bufio.NewReader(conn)
	for {
		args, err := readRESPArray(rd)
		if err != nil {
			return
		}
		if len(args) == 0 {
			continue
		}
		cmd := strings.ToUpper(args[0])
		switch cmd {
		case "SET":
			if len(args) < 3 {
				conn.Write([]byte("-ERR wrong number of arguments\r\n"))
				continue
			}
			s.mu.Lock()
			s.store[args[1]] = args[2]
			s.mu.Unlock()
			conn.Write([]byte("+OK\r\n"))

		case "GET":
			if len(args) < 2 {
				conn.Write([]byte("-ERR wrong number of arguments\r\n"))
				continue
			}
			s.mu.Lock()
			val, ok := s.store[args[1]]
			s.mu.Unlock()
			if !ok {
				conn.Write([]byte("$-1\r\n"))
			} else {
				conn.Write([]byte(fmt.Sprintf("$%d\r\n%s\r\n", len(val), val)))
			}

		case "KEYS":
			pattern := "*"
			if len(args) >= 2 {
				pattern = args[1]
			}
			s.mu.Lock()
			var keys []string
			for k := range s.store {
				if matchPattern(pattern, k) {
					keys = append(keys, k)
				}
			}
			s.mu.Unlock()
			resp := fmt.Sprintf("*%d\r\n", len(keys))
			for _, k := range keys {
				resp += fmt.Sprintf("$%d\r\n%s\r\n", len(k), k)
			}
			conn.Write([]byte(resp))

		default:
			conn.Write([]byte(fmt.Sprintf("-ERR unknown command '%s'\r\n", cmd)))
		}
	}
}

// readRESPArray reads one RESP array from the reader.
func readRESPArray(rd *bufio.Reader) ([]string, error) {
	line, err := rd.ReadString('\n')
	if err != nil {
		return nil, err
	}
	line = strings.TrimRight(line, "\r\n")
	if len(line) == 0 || line[0] != '*' {
		return nil, fmt.Errorf("expected *, got %q", line)
	}
	var count int
	fmt.Sscanf(line, "*%d", &count)
	args := make([]string, 0, count)
	for i := 0; i < count; i++ {
		// $N
		if _, err := rd.ReadString('\n'); err != nil {
			return nil, err
		}
		// data
		data, err := rd.ReadString('\n')
		if err != nil {
			return nil, err
		}
		args = append(args, strings.TrimRight(data, "\r\n"))
	}
	return args, nil
}

// matchPattern matches a simple Redis glob pattern (only supports * prefix/suffix).
func matchPattern(pattern, key string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(key, pattern[:len(pattern)-1])
	}
	if strings.HasPrefix(pattern, "*") {
		return strings.HasSuffix(key, pattern[1:])
	}
	return pattern == key
}
