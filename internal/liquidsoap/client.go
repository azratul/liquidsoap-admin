package liquidsoap

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// Client maintains a persistent TCP connection to the Liquidsoap command server.
// It uses a mutex to serialize commands: the server is not multiplexed.
type Client struct {
	addr    string
	mu      sync.Mutex
	conn    net.Conn
	timeout time.Duration
}

func NewClient(addr string) *Client {
	return &Client{
		addr:    addr,
		timeout: 5 * time.Second,
	}
}

// Command sends a command and returns the full response (without the "END" line).
// If the connection is down, it attempts to reconnect once before failing.
func (c *Client) Command(cmd string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.ensureConnected(); err != nil {
		return "", err
	}

	resp, err := c.sendCommand(cmd)
	if err != nil {
		// single reconnection attempt
		c.closeConn()
		if connErr := c.ensureConnected(); connErr != nil {
			return "", fmt.Errorf("reconnection failed: %w", connErr)
		}
		resp, err = c.sendCommand(cmd)
		if err != nil {
			c.closeConn()
			return "", err
		}
	}
	return resp, nil
}

func (c *Client) sendCommand(cmd string) (string, error) {
	c.conn.SetDeadline(time.Now().Add(c.timeout))

	if _, err := fmt.Fprintf(c.conn, "%s\n", cmd); err != nil {
		return "", fmt.Errorf("write %q: %w", cmd, err)
	}

	var buf bytes.Buffer
	scanner := bufio.NewScanner(c.conn)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "END" {
			break
		}
		if buf.Len() > 0 {
			buf.WriteByte('\n')
		}
		buf.WriteString(line)
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read %q: %w", cmd, err)
	}

	return buf.String(), nil
}

func (c *Client) ensureConnected() error {
	if c.conn != nil {
		return nil
	}
	conn, err := net.DialTimeout("tcp", c.addr, c.timeout)
	if err != nil {
		return fmt.Errorf("connect %s: %w", c.addr, err)
	}
	c.conn = conn
	return nil
}

func (c *Client) closeConn() {
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

// Close explicitly closes the connection.
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closeConn()
}

// Ping verifies that the server is responding.
func (c *Client) Ping() error {
	resp, err := c.Command("help")
	if err != nil {
		return err
	}
	if strings.Contains(resp, "ERROR") {
		return fmt.Errorf("liquidsoap responded with error: %s", resp)
	}
	return nil
}
