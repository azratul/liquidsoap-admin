package liquidsoap

import (
	"fmt"
	"strings"
)

// QueueEntry represents a track in the Liquidsoap queue.
type QueueEntry struct {
	RID    string
	Title  string
	Artist string
	Path   string
}

// NowPlaying represents the current state of the radio.
type NowPlaying struct {
	Artist      string
	Title       string
	Path        string
	ContentType string // "song", "jingle", "live", "fallback"
}

// IsLive reports whether a live source is active.
func (n NowPlaying) IsLive() bool {
	return n.ContentType == "live"
}

// IsEmpty reports whether no metadata is available.
func (n NowPlaying) IsEmpty() bool {
	return n.Artist == "" && n.Title == ""
}

// --- Queue ---

// Push adds a file to the queue. Returns the RID assigned by Liquidsoap.
func (c *Client) Push(queueName, path string) (string, error) {
	resp, err := c.Command(fmt.Sprintf("%s.push %s", queueName, path))
	if err != nil {
		return "", fmt.Errorf("push: %w", err)
	}
	rid := strings.TrimSpace(resp)
	if rid == "" || strings.HasPrefix(rid, "ERROR") {
		return "", fmt.Errorf("liquidsoap rejected the push: %q", resp)
	}
	return rid, nil
}

// QueueRIDs lista los RIDs de la cola primaria.
func (c *Client) QueueRIDs(queueName string) ([]string, error) {
	resp, err := c.Command(queueName + ".queue")
	if err != nil {
		return nil, fmt.Errorf("queue list: %w", err)
	}
	resp = strings.TrimSpace(resp)
	if resp == "" {
		return nil, nil
	}
	return strings.Fields(resp), nil
}

// Metadata returns the metadata of a RID as a key→value map.
func (c *Client) Metadata(rid string) (map[string]string, error) {
	resp, err := c.Command("request.metadata " + rid)
	if err != nil {
		return nil, fmt.Errorf("metadata %s: %w", rid, err)
	}

	meta := make(map[string]string)
	for _, line := range strings.Split(resp, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		k, v, found := strings.Cut(line, "=")
		if found {
			meta[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"`)
		}
	}
	return meta, nil
}

// Remove removes a RID from the queue.
func (c *Client) Remove(queueName, rid string) error {
	resp, err := c.Command(fmt.Sprintf("%s.remove %s", queueName, rid))
	if err != nil {
		return fmt.Errorf("remove %s: %w", rid, err)
	}
	if strings.HasPrefix(strings.TrimSpace(resp), "ERROR") {
		return fmt.Errorf("liquidsoap could not remove %s: %s", rid, resp)
	}
	return nil
}

// Flush empties the queue and skips the current track.
// In Liquidsoap 2.x the command is flush_and_skip (flush alone does not exist).
func (c *Client) Flush(queueName string) error {
	_, err := c.Command(queueName + ".flush_and_skip")
	if err != nil {
		return fmt.Errorf("flush: %w", err)
	}
	return nil
}

// Skip skips the current song in the queue.
func (c *Client) Skip(queueName string) error {
	_, err := c.Command(queueName + ".skip")
	if err != nil {
		return fmt.Errorf("skip: %w", err)
	}
	return nil
}

// QueueEntries returns the full queue with resolved metadata for each RID.
// If metadata for an individual entry fails, it is still included with only the RID.
func (c *Client) QueueEntries(queueName string) ([]QueueEntry, error) {
	rids, err := c.QueueRIDs(queueName)
	if err != nil {
		return nil, err
	}

	entries := make([]QueueEntry, 0, len(rids))
	for _, rid := range rids {
		entry := QueueEntry{RID: rid}
		if meta, err := c.Metadata(rid); err == nil {
			entry.Title  = meta["title"]
			entry.Artist = meta["artist"]
			entry.Path   = meta["filename"]
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// Uptime returns the Liquidsoap process uptime string (e.g. "0d 02h 34m 12s").
func (c *Client) Uptime() (string, error) {
	resp, err := c.Command("uptime")
	if err != nil {
		return "", fmt.Errorf("uptime: %w", err)
	}
	return strings.TrimSpace(resp), nil
}

// --- Now playing ---

// OnAir queries the custom command "radio.on_air" registered in the .liq script.
// Expected response format: "artist|title|filename|sc_content_type"
func (c *Client) OnAir() (NowPlaying, error) {
	resp, err := c.Command("radio.on_air")
	if err != nil {
		return NowPlaying{}, fmt.Errorf("on_air: %w", err)
	}

	resp = strings.TrimSpace(resp)
	if strings.HasPrefix(resp, "ERROR") {
		return NowPlaying{}, fmt.Errorf("on_air: %s", resp)
	}

	parts := strings.Split(resp, "|")
	if len(parts) != 4 {
		return NowPlaying{}, fmt.Errorf("on_air: malformed response %q", resp)
	}

	return NowPlaying{
		Artist:      parts[0],
		Title:       parts[1],
		Path:        parts[2],
		ContentType: parts[3],
	}, nil
}
