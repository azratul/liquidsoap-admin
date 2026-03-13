package library

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

// TrackInfo represents the information about a track obtained from Last.fm.
type TrackInfo struct {
	Name   string
	Artist string
	Album  string
	Image  string // Image URL
}

// LastFMClient requests track info from Last.fm API, with simple caching per track (artist+song).
type LastFMClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	cacheKey   string
	cacheVal   TrackInfo
}

func NewLastFMClient(apiKey, baseURL string) *LastFMClient {
	return &LastFMClient{
		apiKey:  apiKey,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 4 * time.Second,
		},
	}
}

// Fetch retrieves track information for the given artist and song from Last.fm.
func (c *LastFMClient) Fetch(artist, song string) TrackInfo {
	if c.apiKey == "" || artist == "" || song == "" {
		return TrackInfo{}
	}

	key := artist + "\x00" + song
	if key == c.cacheKey {
		return c.cacheVal
	}

	apiURL := fmt.Sprintf(
		"%s/?method=track.getInfo&api_key=%s&artist=%s&track=%s&format=json",
		c.baseURL,
		c.apiKey,
		url.QueryEscape(artist),
		url.QueryEscape(song),
	)

	resp, err := c.httpClient.Get(apiURL)
	if err != nil {
		slog.Warn("lastfm fetch", "err", err)
		return TrackInfo{}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return TrackInfo{}
	}

	var payload struct {
		Track struct {
			Name   string `json:"name"`
			Artist struct {
				Name string `json:"name"`
			} `json:"artist"`
			Album struct {
				Title string `json:"title"`
				Image []struct {
					Text string `json:"#text"`
					Size string `json:"size"`
				} `json:"image"`
			} `json:"album"`
		} `json:"track"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return TrackInfo{}
	}

	info := TrackInfo{
		Name:   payload.Track.Name,
		Artist: payload.Track.Artist.Name,
		Album:  payload.Track.Album.Title,
	}

	for _, img := range payload.Track.Album.Image {
		if img.Size == "large" && img.Text != "" {
			info.Image = img.Text
			break
		}
	}
	if info.Image == "" {
		for _, img := range payload.Track.Album.Image {
			if img.Text != "" {
				info.Image = img.Text
				break
			}
		}
	}

	c.cacheKey = key
	c.cacheVal = info
	return info
}
