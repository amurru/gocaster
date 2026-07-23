package rss

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/amurru/gocaster/internal/application"
	"github.com/amurru/gocaster/internal/domain"
	"github.com/mmcdole/gofeed"
)

type FeedFetcher struct {
	client *http.Client
}

func NewFeedFetcher() *FeedFetcher {
	return &FeedFetcher{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (f *FeedFetcher) Parse(ctx context.Context, url string, conditional domain.FeedHeaders) (*domain.Podcast, []domain.Episode, domain.FeedHeaders, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, domain.FeedHeaders{}, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("User-Agent", "gocaster/1.0")
	if conditional.ETag != "" {
		req.Header.Set("If-None-Match", conditional.ETag)
	}
	if conditional.LastModified != "" {
		req.Header.Set("If-Modified-Since", conditional.LastModified)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, nil, domain.FeedHeaders{}, err
	}
	defer resp.Body.Close()

	headers := domain.FeedHeaders{
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
	}

	if resp.StatusCode == http.StatusNotModified {
		return nil, nil, headers, application.ErrFeedNotModified
	}

	if resp.StatusCode != http.StatusOK {
		return nil, nil, domain.FeedHeaders{}, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, domain.FeedHeaders{}, fmt.Errorf("read body: %w", err)
	}

	fp := gofeed.NewParser()
	feed, err := fp.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, nil, domain.FeedHeaders{}, err
	}

	podcast := &domain.Podcast{
		Title:       feed.Title,
		FeedURL:     url,
		Description: feed.Description,
	}

	episodes := make([]domain.Episode, 0, len(feed.Items))
	for _, item := range feed.Items {
		if len(item.Enclosures) == 0 {
			continue
		}

		episode := domain.Episode{
			Title:       item.Title,
			Description: item.Description,
			AudioURL:    item.Enclosures[0].URL,
		}

		if item.PublishedParsed != nil {
			episode.PublishedAt = *item.PublishedParsed
		}

		if item.ITunesExt != nil && item.ITunesExt.Duration != "" {
			episode.PlaybackDuration = parseDuration(item.ITunesExt.Duration)
		}

		episodes = append(episodes, episode)
	}
	return podcast, episodes, headers, nil
}

// parseDuration converts an iTunes duration string to seconds. Supported
// formats:
//   - seconds as an integer ("3600")
//   - MM:SS ("45:30") or HH:MM:SS ("2:09:56")
//   - ISO 8601 durations as used in iTunes-extension tags: "PT1H30M",
//     "PT45M30S", "PT30S", and optionally a days component ("P1DT2H").
//
// Malformed or empty input returns 0 rather than panicking.
func parseDuration(duration string) int {
	if duration == "" {
		return 0
	}

	// ISO 8601 duration: starts with 'P' (and contains a 'T' separating the
	// date and time parts). iTunes feeds use this for <itunes:duration>.
	if isISODuration(duration) {
		if secs, ok := parseISODuration(duration); ok {
			return secs
		}
		return 0
	}

	parts := strings.Split(duration, ":")

	switch len(parts) {
	case 1:
		// Format: seconds as an integer. strconv.Atoi (unlike fmt.Sscanf) rejects
		// trailing garbage like "30abc", which should map to 0 not 30.
		seconds, err := strconv.Atoi(duration)
		if err != nil {
			return 0
		}
		return seconds
	case 2:
		// Format: MM:SS — each part must be a pure integer.
		minutes, err1 := strconv.Atoi(parts[0])
		seconds, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil {
			return 0
		}
		return minutes*60 + seconds
	case 3:
		// Format: HH:MM:SS — each part must be a pure integer.
		hours, err1 := strconv.Atoi(parts[0])
		minutes, err2 := strconv.Atoi(parts[1])
		seconds, err3 := strconv.Atoi(parts[2])
		if err1 != nil || err2 != nil || err3 != nil {
			return 0
		}
		return hours*3600 + minutes*60 + seconds
	default:
		return 0
	}
}

// isISODuration reports whether s looks like an ISO 8601 duration (begins with
// 'P' or '-P'/'+P' and contains a 'T').
func isISODuration(s string) bool {
	s = strings.TrimSpace(s)
	i := 0
	if i < len(s) && (s[i] == '-' || s[i] == '+') {
		i++
	}
	return i < len(s) && s[i] == 'P' && strings.Contains(s, "T")
}

// parseISODuration parses an ISO 8601 duration like "PT1H30M", "PT45M30S",
// "PT30S", or "P1DT2H" into seconds. The bool result is false if the input is
// not a recognized ISO 8601 duration.
func parseISODuration(s string) (int, bool) {
	s = strings.TrimSpace(s)
	negative := false
	if len(s) > 0 && (s[0] == '-' || s[0] == '+') {
		negative = s[0] == '-'
		s = s[1:]
	}
	if !strings.HasPrefix(s, "P") {
		return 0, false
	}
	s = s[1:]

	// Split into the date part (before T) and the time part (after T).
	var datePart, timePart string
	if idx := strings.Index(s, "T"); idx >= 0 {
		datePart, timePart = s[:idx], s[idx+1:]
	} else {
		datePart = s
	}

	var total int
	var ok bool

	// Date part supports days (and weeks). Years/months are deliberately not
	// converted (ambiguous duration), so their presence makes the duration
	// unparseable to a fixed second count.
	if datePart != "" {
		d, good := scanISOUnits(datePart, map[byte]int{'W': 7 * 86400, 'D': 86400})
		if !good {
			return 0, false
		}
		total += d
		ok = true
	}
	if timePart != "" {
		t, good := scanISOUnits(timePart, map[byte]int{'H': 3600, 'M': 60, 'S': 1})
		if !good {
			return 0, false
		}
		total += t
		ok = true
	}

	if !ok {
		return 0, false
	}
	if negative {
		total = -total
	}
	return total, true
}

// scanISOUnits walks a sequence of "<number><unit>" pairs (e.g. "1H30M") and
// sums each number multiplied by its unit's per-unit seconds. Returns
// (sum, false) if any character is not a digit or a recognized unit, or if a
// unit appears without a preceding number.
func scanISOUnits(s string, units map[byte]int) (int, bool) {
	total := 0
	num := 0
	seenDigit := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			num = num*10 + int(c-'0')
			seenDigit = true
			continue
		}
		perUnit, ok := units[c]
		if !ok || !seenDigit {
			return 0, false
		}
		total += num * perUnit
		num = 0
		seenDigit = false
	}
	// Trailing digits with no unit is malformed.
	if seenDigit {
		return 0, false
	}
	return total, true
}
