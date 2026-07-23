package rss

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/amurru/gocaster/internal/application"
	"github.com/amurru/gocaster/internal/domain"
)

func TestNewFeedFetcher(t *testing.T) {
	f := NewFeedFetcher()
	if f == nil {
		t.Fatal("NewFeedFetcher() returned nil")
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		// Existing formats (preserved).
		{"HH:MM:SS format", "2:09:56", 7796},
		{"MM:SS format", "45:30", 2730},
		{"Empty string", "", 0},
		{"Single value", "30", 30},
		{"Zero duration", "0:00:00", 0},

		// Seconds as integer (issue #8 acceptance criteria).
		{"seconds 3600", "3600", 3600},

		// ISO 8601 durations (issue #8) — common in iTunes-extension tags.
		{"ISO PT1H30M", "PT1H30M", 5400},
		{"ISO PT45M", "PT45M", 2700},
		{"ISO PT45M30S", "PT45M30S", 2730},
		{"ISO PT30S", "PT30S", 30},
		{"ISO PT1H30M45S", "PT1H30M45S", 5445},
		{"ISO with days P1DT2H", "P1DT2H", 86400 + 7200},

		// Malformed input must fall through to 0, not panic (issue #8).
		{"garbage letters", "abc", 0},
		{"HH:MM:SS garbage", "xx:yy:zz", 0},
		{"ISO malformed trailing digits", "PT1H30", 0},
		{"ISO malformed unknown unit", "PT1X", 0},
		{"single colon", ":", 0},
		{"four colon parts", "1:2:3:4", 0},

		// Trailing garbage must be rejected (strconv.Atoi, not permissive
		// fmt.Sscanf). "30abc" should be 0, not 30.
		{"trailing garbage seconds", "30abc", 0},
		{"trailing garbage MM:SS", "45:30xyz", 0},
		{"trailing garbage HH:MM:SS", "1:2:3junk", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseDuration(tt.input)
			if result != tt.expected {
				t.Errorf("parseDuration(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

const validRSSFeed = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd">
  <channel>
    <title>Test Podcast</title>
    <description>A test podcast feed</description>
    <item>
      <title>Episode 1</title>
      <description>First episode</description>
      <enclosure url="https://example.com/ep1.mp3" type="audio/mpeg" length="12345"/>
      <pubDate>Mon, 01 Jan 2024 12:00:00 +0000</pubDate>
      <itunes:duration>3600</itunes:duration>
    </item>
    <item>
      <title>Episode 2 - No Audio</title>
      <description>Skipped - no enclosure</description>
    </item>
    <item>
      <title>Episode 3</title>
      <description>Third episode with ISO duration</description>
      <enclosure url="https://example.com/ep3.mp3" type="audio/mpeg" length="67890"/>
      <pubDate>Tue, 02 Jan 2024 12:00:00 +0000</pubDate>
      <itunes:duration>PT45M30S</itunes:duration>
    </item>
  </channel>
</rss>`

const atomFeed = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Atom Podcast</title>
  <entry>
    <title>Atom Episode 1</title>
    <content>First atom episode</content>
    <link rel="enclosure" href="https://example.com/atom1.mp3" type="audio/mpeg"/>
    <published>2024-01-15T10:00:00Z</published>
  </entry>
</feed>`

func newTestServer(feedBody string, statusCode int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		if feedBody != "" {
			w.Write([]byte(feedBody))
		}
	}))
}

func TestParseValidRSS(t *testing.T) {
	srv := newTestServer(validRSSFeed, http.StatusOK)
	defer srv.Close()

	f := NewFeedFetcher()
	podcast, episodes, _, err := f.Parse(context.Background(), srv.URL, domain.FeedHeaders{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if podcast.Title != "Test Podcast" {
		t.Errorf("podcast title = %q, want %q", podcast.Title, "Test Podcast")
	}
	if podcast.Description != "A test podcast feed" {
		t.Errorf("podcast description = %q, want %q", podcast.Description, "A test podcast feed")
	}
	if podcast.FeedURL != srv.URL {
		t.Errorf("podcast FeedURL = %q, want %q", podcast.FeedURL, srv.URL)
	}

	if len(episodes) != 2 {
		t.Fatalf("expected 2 episodes (one without enclosure skipped), got %d", len(episodes))
	}

	if episodes[0].Title != "Episode 1" {
		t.Errorf("episode[0] title = %q, want %q", episodes[0].Title, "Episode 1")
	}
	if episodes[0].AudioURL != "https://example.com/ep1.mp3" {
		t.Errorf("episode[0] AudioURL = %q, want %q", episodes[0].AudioURL, "https://example.com/ep1.mp3")
	}
	if episodes[0].PlaybackDuration != 3600 {
		t.Errorf("episode[0] PlaybackDuration = %d, want 3600", episodes[0].PlaybackDuration)
	}
	if episodes[0].PublishedAt.IsZero() {
		t.Error("episode[0] PublishedAt should not be zero")
	}

	if episodes[1].Title != "Episode 3" {
		t.Errorf("episode[1] title = %q, want %q", episodes[1].Title, "Episode 3")
	}
	if episodes[1].PlaybackDuration != 2730 {
		t.Errorf("episode[1] PlaybackDuration = %d, want 2730", episodes[1].PlaybackDuration)
	}
}

func TestParseValidAtomFeed(t *testing.T) {
	srv := newTestServer(atomFeed, http.StatusOK)
	defer srv.Close()

	f := NewFeedFetcher()
	podcast, episodes, _, err := f.Parse(context.Background(), srv.URL, domain.FeedHeaders{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if podcast.Title != "Atom Podcast" {
		t.Errorf("podcast title = %q, want %q", podcast.Title, "Atom Podcast")
	}
	if len(episodes) != 1 {
		t.Fatalf("expected 1 episode, got %d", len(episodes))
	}
	if episodes[0].Title != "Atom Episode 1" {
		t.Errorf("episode title = %q, want %q", episodes[0].Title, "Atom Episode 1")
	}
	if episodes[0].AudioURL != "https://example.com/atom1.mp3" {
		t.Errorf("episode AudioURL = %q, want %q", episodes[0].AudioURL, "https://example.com/atom1.mp3")
	}
}

func TestParseEmptyFeed(t *testing.T) {
	emptyFeed := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Empty Podcast</title>
  </channel>
</rss>`
	srv := newTestServer(emptyFeed, http.StatusOK)
	defer srv.Close()

	f := NewFeedFetcher()
	podcast, episodes, _, err := f.Parse(context.Background(), srv.URL, domain.FeedHeaders{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if podcast.Title != "Empty Podcast" {
		t.Errorf("podcast title = %q, want %q", podcast.Title, "Empty Podcast")
	}
	if len(episodes) != 0 {
		t.Errorf("expected 0 episodes, got %d", len(episodes))
	}
}

func TestParseMalformedXML(t *testing.T) {
	srv := newTestServer("this is not xml at all", http.StatusOK)
	defer srv.Close()

	f := NewFeedFetcher()
	_, _, _, err := f.Parse(context.Background(), srv.URL, domain.FeedHeaders{})
	if err == nil {
		t.Fatal("expected error for malformed XML, got nil")
	}
}

func TestParseServerReturnsNonXMLContentType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body>Not a feed</body></html>"))
	}))
	defer srv.Close()

	f := NewFeedFetcher()
	_, _, _, err := f.Parse(context.Background(), srv.URL, domain.FeedHeaders{})
	_ = err
}

func TestParseServerReturns404(t *testing.T) {
	srv := newTestServer("", http.StatusNotFound)
	defer srv.Close()

	f := NewFeedFetcher()
	_, _, _, err := f.Parse(context.Background(), srv.URL, domain.FeedHeaders{})
	if err == nil {
		t.Fatal("expected error for 404 response, got nil")
	}
}

func TestParseServerReturns500(t *testing.T) {
	srv := newTestServer("", http.StatusInternalServerError)
	defer srv.Close()

	f := NewFeedFetcher()
	_, _, _, err := f.Parse(context.Background(), srv.URL, domain.FeedHeaders{})
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
}

func TestParseContextCancellation(t *testing.T) {
	srv := newTestServer(validRSSFeed, http.StatusOK)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	f := NewFeedFetcher()
	_, _, _, err := f.Parse(ctx, srv.URL, domain.FeedHeaders{})
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

func TestParseSlowServerTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(15 * time.Second)
	}))
	defer srv.Close()

	ctx := context.Background()
	f := NewFeedFetcher()
	_, _, _, err := f.Parse(ctx, srv.URL, domain.FeedHeaders{})
	if err == nil {
		t.Fatal("expected error for slow server, got nil")
	}
}

func TestParseItemsWithoutPublishedDate(t *testing.T) {
	feed := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>No Dates Podcast</title>
    <item>
      <title>No Date Episode</title>
      <enclosure url="https://example.com/nodate.mp3" type="audio/mpeg"/>
    </item>
  </channel>
</rss>`
	srv := newTestServer(feed, http.StatusOK)
	defer srv.Close()

	f := NewFeedFetcher()
	podcast, episodes, _, err := f.Parse(context.Background(), srv.URL, domain.FeedHeaders{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if podcast.Title != "No Dates Podcast" {
		t.Errorf("podcast title = %q, want %q", podcast.Title, "No Dates Podcast")
	}
	if len(episodes) != 1 {
		t.Fatalf("expected 1 episode, got %d", len(episodes))
	}
	if !episodes[0].PublishedAt.IsZero() {
		t.Error("expected zero PublishedAt for item without pubDate")
	}
	if episodes[0].PlaybackDuration != 0 {
		t.Errorf("PlaybackDuration = %d, want 0", episodes[0].PlaybackDuration)
	}
}

func TestParseItemsWithITunesExtension(t *testing.T) {
	feed := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd">
  <channel>
    <title>Duration Podcast</title>
    <item>
      <title>HH:MM:SS Duration</title>
      <enclosure url="https://example.com/1.mp3" type="audio/mpeg"/>
      <itunes:duration>1:30:00</itunes:duration>
    </item>
    <item>
      <title>ISO Duration</title>
      <enclosure url="https://example.com/2.mp3" type="audio/mpeg"/>
      <itunes:duration>PT2H15M</itunes:duration>
    </item>
    <item>
      <title>Empty Duration</title>
      <enclosure url="https://example.com/3.mp3" type="audio/mpeg"/>
      <itunes:duration></itunes:duration>
    </item>
  </channel>
</rss>`
	srv := newTestServer(feed, http.StatusOK)
	defer srv.Close()

	f := NewFeedFetcher()
	_, episodes, _, err := f.Parse(context.Background(), srv.URL, domain.FeedHeaders{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(episodes) != 3 {
		t.Fatalf("expected 3 episodes, got %d", len(episodes))
	}
	if episodes[0].PlaybackDuration != 5400 {
		t.Errorf("episode[0] PlaybackDuration = %d, want 5400", episodes[0].PlaybackDuration)
	}
	if episodes[1].PlaybackDuration != 8100 {
		t.Errorf("episode[1] PlaybackDuration = %d, want 8100", episodes[1].PlaybackDuration)
	}
	if episodes[2].PlaybackDuration != 0 {
		t.Errorf("episode[2] PlaybackDuration = %d, want 0 (empty duration)", episodes[2].PlaybackDuration)
	}
}

func TestIsISODuration(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"PT1H", true},
		{"P1DT2H", true},
		{"-PT1H", true},
		{"+PT30M", true},
		{"P", false},
		{"PT", true},
		{"3600", false},
		{"45:30", false},
		{"abc", false},
		{"  PT1H  ", true},
		{"+P1D", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isISODuration(tt.input)
			if result != tt.expected {
				t.Errorf("isISODuration(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseISODurationDirect(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
		ok       bool
	}{
		{"PT30S", "PT30S", 30, true},
		{"PT1H", "PT1H", 3600, true},
		{"PT1H30M", "PT1H30M", 5400, true},
		{"PT1H30M45S", "PT1H30M45S", 5445, true},
		{"P1DT2H", "P1DT2H", 86400 + 7200, true},
		{"negative PT1H", "-PT1H", -3600, true},
		{"positive +PT30M", "+PT30M", 1800, true},
		{"P without T", "P1D", 86400, true},
		{"just P", "P", 0, false},
		{"empty string", "", 0, false},
		{"T without P prefix", "T1H", 0, false},
		{"unknown unit", "PT1X", 0, false},
		{"trailing digits no unit", "PT1H30", 0, false},
		{"unit without number", "PH", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseISODuration(tt.input)
			if ok != tt.ok {
				t.Errorf("parseISODuration(%q) ok = %v, want %v", tt.input, ok, tt.ok)
			}
			if got != tt.expected {
				t.Errorf("parseISODuration(%q) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}

func TestScanISOUnitsEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		units    map[byte]int
		expected int
		ok       bool
	}{
		{"weeks", "2W", map[byte]int{'W': 7 * 86400}, 2 * 7 * 86400, true},
		{"empty string", "", map[byte]int{'H': 3600}, 0, true},
		{"digits only no unit", "123", map[byte]int{'H': 3600}, 0, false},
		{"unit before digit", "H1", map[byte]int{'H': 3600}, 0, false},
		{"unknown character", "1X", map[byte]int{'H': 3600}, 0, false},
		{"multi digit", "12H", map[byte]int{'H': 3600}, 12 * 3600, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := scanISOUnits(tt.input, tt.units)
			if ok != tt.ok {
				t.Errorf("scanISOUnits(%q) ok = %v, want %v", tt.input, ok, tt.ok)
			}
			if got != tt.expected {
				t.Errorf("scanISOUnits(%q) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseMultipleEnclosures(t *testing.T) {
	feed := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Multi Enclosure Podcast</title>
    <item>
      <title>Multi Enclosure Episode</title>
      <enclosure url="https://example.com/first.mp3" type="audio/mpeg" length="1000"/>
      <enclosure url="https://example.com/second.mp3" type="audio/mpeg" length="2000"/>
    </item>
  </channel>
</rss>`
	srv := newTestServer(feed, http.StatusOK)
	defer srv.Close()

	f := NewFeedFetcher()
	_, episodes, _, err := f.Parse(context.Background(), srv.URL, domain.FeedHeaders{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(episodes) != 1 {
		t.Fatalf("expected 1 episode, got %d", len(episodes))
	}
	if episodes[0].AudioURL != "https://example.com/first.mp3" {
		t.Errorf("AudioURL = %q, want first enclosure URL", episodes[0].AudioURL)
	}
}

func TestParseAllItemsSkippedNoEnclosures(t *testing.T) {
	feed := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>All Skipped Podcast</title>
    <item>
      <title>Item 1 - no enclosure</title>
    </item>
    <item>
      <title>Item 2 - no enclosure</title>
    </item>
  </channel>
</rss>`
	srv := newTestServer(feed, http.StatusOK)
	defer srv.Close()

	f := NewFeedFetcher()
	podcast, episodes, _, err := f.Parse(context.Background(), srv.URL, domain.FeedHeaders{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if podcast.Title != "All Skipped Podcast" {
		t.Errorf("podcast title = %q, want %q", podcast.Title, "All Skipped Podcast")
	}
	if len(episodes) != 0 {
		t.Errorf("expected 0 episodes (all skipped), got %d", len(episodes))
	}
}

func TestParseInvalidURL(t *testing.T) {
	f := NewFeedFetcher()
	_, _, _, err := f.Parse(context.Background(), "http://localhost:0/nonexistent", domain.FeedHeaders{})
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}

func TestParseFeedWithEncodingIssues(t *testing.T) {
	feed := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Sp&#233;cial &#201;pisode</title>
    <item>
      <title>&amp; &lt; &gt; Characters</title>
      <description>Description with &quot;quotes&quot;</description>
      <enclosure url="https://example.com/special.mp3" type="audio/mpeg"/>
    </item>
  </channel>
</rss>`
	srv := newTestServer(feed, http.StatusOK)
	defer srv.Close()

	f := NewFeedFetcher()
	podcast, episodes, _, err := f.Parse(context.Background(), srv.URL, domain.FeedHeaders{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(podcast.Title, "cial") {
		t.Errorf("podcast title = %q, expected HTML entities decoded", podcast.Title)
	}
	if len(episodes) != 1 {
		t.Fatalf("expected 1 episode, got %d", len(episodes))
	}
}

func TestParseFeedDescription(t *testing.T) {
	feed := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Desc Podcast</title>
    <description>Podcast description here</description>
    <item>
      <title>Ep 1</title>
      <description>Episode description</description>
      <enclosure url="https://example.com/ep1.mp3" type="audio/mpeg"/>
    </item>
  </channel>
</rss>`
	srv := newTestServer(feed, http.StatusOK)
	defer srv.Close()

	f := NewFeedFetcher()
	podcast, episodes, _, err := f.Parse(context.Background(), srv.URL, domain.FeedHeaders{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if podcast.Description != "Podcast description here" {
		t.Errorf("podcast Description = %q, want %q", podcast.Description, "Podcast description here")
	}
	if episodes[0].Description != "Episode description" {
		t.Errorf("episode Description = %q, want %q", episodes[0].Description, "Episode description")
	}
}

func TestParseReturnsETagAndLastModified(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"abc123"`)
		w.Header().Set("Last-Modified", "Mon, 01 Jan 2024 12:00:00 GMT")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(validRSSFeed))
	}))
	defer srv.Close()

	f := NewFeedFetcher()
	_, _, headers, err := f.Parse(context.Background(), srv.URL, domain.FeedHeaders{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if headers.ETag != `"abc123"` {
		t.Errorf("ETag = %q, want %q", headers.ETag, `"abc123"`)
	}
	if headers.LastModified != "Mon, 01 Jan 2024 12:00:00 GMT" {
		t.Errorf("LastModified = %q, want %q", headers.LastModified, "Mon, 01 Jan 2024 12:00:00 GMT")
	}
}

func TestParseSendsIfNoneMatchHeader(t *testing.T) {
	var receivedIfNoneMatch string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedIfNoneMatch = r.Header.Get("If-None-Match")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(validRSSFeed))
	}))
	defer srv.Close()

	f := NewFeedFetcher()
	conditional := domain.FeedHeaders{ETag: `"old-etag"`}
	_, _, _, err := f.Parse(context.Background(), srv.URL, conditional)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedIfNoneMatch != `"old-etag"` {
		t.Errorf("If-None-Match = %q, want %q", receivedIfNoneMatch, `"old-etag"`)
	}
}

func TestParseSendsIfModifiedSinceHeader(t *testing.T) {
	var receivedIfModifiedSince string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedIfModifiedSince = r.Header.Get("If-Modified-Since")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(validRSSFeed))
	}))
	defer srv.Close()

	f := NewFeedFetcher()
	conditional := domain.FeedHeaders{LastModified: "Mon, 01 Jan 2024 12:00:00 GMT"}
	_, _, _, err := f.Parse(context.Background(), srv.URL, conditional)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedIfModifiedSince != "Mon, 01 Jan 2024 12:00:00 GMT" {
		t.Errorf("If-Modified-Since = %q, want %q", receivedIfModifiedSince, "Mon, 01 Jan 2024 12:00:00 GMT")
	}
}

func TestParse304ReturnsErrFeedNotModified(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"same-etag"`)
		w.Header().Set("Last-Modified", "Mon, 01 Jan 2024 12:00:00 GMT")
		w.WriteHeader(http.StatusNotModified)
	}))
	defer srv.Close()

	f := NewFeedFetcher()
	conditional := domain.FeedHeaders{ETag: `"same-etag"`}
	podcast, episodes, headers, err := f.Parse(context.Background(), srv.URL, conditional)
	if !errors.Is(err, application.ErrFeedNotModified) {
		t.Fatalf("expected ErrFeedNotModified, got %v", err)
	}
	if podcast != nil {
		t.Errorf("expected nil podcast, got %v", podcast)
	}
	if episodes != nil {
		t.Errorf("expected nil episodes, got %v", episodes)
	}
	if headers.ETag != `"same-etag"` {
		t.Errorf("ETag = %q, want %q", headers.ETag, `"same-etag"`)
	}
	if headers.LastModified != "Mon, 01 Jan 2024 12:00:00 GMT" {
		t.Errorf("LastModified = %q, want %q", headers.LastModified, "Mon, 01 Jan 2024 12:00:00 GMT")
	}
}

func TestParseDoesNotSendHeadersWhenEmpty(t *testing.T) {
	var receivedIfNoneMatch, receivedIfModifiedSince string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedIfNoneMatch = r.Header.Get("If-None-Match")
		receivedIfModifiedSince = r.Header.Get("If-Modified-Since")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(validRSSFeed))
	}))
	defer srv.Close()

	f := NewFeedFetcher()
	_, _, _, err := f.Parse(context.Background(), srv.URL, domain.FeedHeaders{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedIfNoneMatch != "" {
		t.Errorf("expected no If-None-Match header, got %q", receivedIfNoneMatch)
	}
	if receivedIfModifiedSince != "" {
		t.Errorf("expected no If-Modified-Since header, got %q", receivedIfModifiedSince)
	}
}
