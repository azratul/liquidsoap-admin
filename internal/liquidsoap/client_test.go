package liquidsoap

import (
	"bufio"
	"net"
	"strings"
	"testing"
)

// closeConn, as a script entry or suffix, makes the fake server close the
// connection (after writing the prefix, if any) instead of ending with "END".
const closeConn = "\x00CLOSE"

// startServer starts a fake Liquidsoap command server and returns a Client
// pointed at it. Each command received consumes the next entry of script and
// writes it verbatim as the response.
func startServer(t *testing.T, script ...string) *Client {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })

	responses := make(chan string, len(script))
	for _, s := range script {
		responses <- s
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				sc := bufio.NewScanner(conn)
				for sc.Scan() {
					select {
					case resp := <-responses:
						if data, doClose := strings.CutSuffix(resp, closeConn); doClose {
							conn.Write([]byte(data))
							return
						}
						conn.Write([]byte(resp))
					default:
						return
					}
				}
			}()
		}
	}()

	c := NewClient(ln.Addr().String())
	t.Cleanup(c.Close)
	return c
}

func TestCommandAcceptsResponseTerminatedByEND(t *testing.T) {
	c := startServer(t, "world\nEND\n")

	resp, err := c.Command("hello")
	if err != nil {
		t.Fatal(err)
	}
	if resp != "world" {
		t.Fatalf("resp %q, want %q", resp, "world")
	}
}

func TestCommandReconnectsAfterEOFBeforeEND(t *testing.T) {
	// The server drops the connection with a timeout message and no END:
	// the client must discard it, reconnect and retry transparently.
	c := startServer(t,
		"Connection timed out.. Bye!\n"+closeConn,
		"0d 01h 02m 03s\nEND\n",
	)

	resp, err := c.Command("uptime")
	if err != nil {
		t.Fatal(err)
	}
	if resp != "0d 01h 02m 03s" {
		t.Fatalf("resp %q, want uptime", resp)
	}
}

func TestCommandFailsWhenRetryAlsoDies(t *testing.T) {
	c := startServer(t,
		"Connection timed out.. Bye!\n"+closeConn,
		"Connection timed out.. Bye!\n"+closeConn,
	)

	if _, err := c.Command("uptime"); err == nil {
		t.Fatal("expected error when both attempts end without END")
	}
}

func TestOnAirParsesFourFields(t *testing.T) {
	c := startServer(t, "Bloc Party|Like Eating Glass|/music/a.mp3|song\nEND\n")

	np, err := c.OnAir()
	if err != nil {
		t.Fatal(err)
	}
	want := NowPlaying{
		Artist:      "Bloc Party",
		Title:       "Like Eating Glass",
		Path:        "/music/a.mp3",
		ContentType: "song",
	}
	if np != want {
		t.Fatalf("np %+v, want %+v", np, want)
	}
}

func TestOnAirRejectsIncompleteResponse(t *testing.T) {
	// A well-terminated response that is not artist|title|filename|type
	// must never become a valid NowPlaying.
	c := startServer(t, "Connection timed out.. Bye!\nEND\n")

	if _, err := c.OnAir(); err == nil {
		t.Fatal("expected error for malformed on_air response")
	}
}
