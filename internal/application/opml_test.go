package application

import (
	"strings"
	"testing"

	"github.com/amurru/gocaster/internal/domain"
)

func TestExportOPML(t *testing.T) {
	podcasts := []domain.Podcast{
		{
			Title:   "Go Time",
			FeedURL: "https://changelog.com/gotime/feed",
		},
		{
			Title:   "Syntax",
			FeedURL: "https://syntax.fm/rss",
		},
	}

	data, err := ExportOPML(podcasts)
	if err != nil {
		t.Fatalf("ExportOPML returned error: %v", err)
	}

	xml := string(data)

	tests := []string{
		`<?xml version="1.0" encoding="UTF-8"?>`,
		`<opml version="2.0">`,
		`<title>Gocaster Subscriptions</title>`,
		`text="Go Time"`,
		`xmlUrl="https://changelog.com/gotime/feed"`,
		`text="Syntax"`,
		`xmlUrl="https://syntax.fm/rss"`,
	}

	for _, want := range tests {
		if !strings.Contains(xml, want) {
			t.Errorf("expected XML to contain %q", want)
		}
	}
}

func TestImportOPML(t *testing.T) {
	data := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
  <head>
    <title>Gocaster Subscriptions</title>
  </head>
  <body>
    <outline type="rss" text="Go Time" xmlUrl="https://changelog.com/gotime/feed"/>
    <outline type="rss" text="Syntax" xmlUrl="https://syntax.fm/rss"/>
  </body>
</opml>`)

	urls, err := ImportOPML(data)
	if err != nil {
		t.Fatalf("ImportOPML returned error: %v", err)
	}

	if len(urls) != 2 {
		t.Fatalf("expected 2 urls, got %d", len(urls))
	}

	if urls[0] != "https://changelog.com/gotime/feed" {
		t.Errorf("unexpected first url: %s", urls[0])
	}

	if urls[1] != "https://syntax.fm/rss" {
		t.Errorf("unexpected second url: %s", urls[1])
	}
}

func TestOPMLRoundTrip(t *testing.T) {
	podcasts := []domain.Podcast{
		{
			Title:   "Go Time",
			FeedURL: "https://changelog.com/gotime/feed",
		},
		{
			Title:   "Syntax",
			FeedURL: "https://syntax.fm/rss",
		},
	}

	data, err := ExportOPML(podcasts)
	if err != nil {
		t.Fatalf("ExportOPML returned error: %v", err)
	}

	urls, err := ImportOPML(data)
	if err != nil {
		t.Fatalf("ImportOPML returned error: %v", err)
	}

	if len(urls) != len(podcasts) {
		t.Fatalf("expected %d urls, got %d", len(podcasts), len(urls))
	}

	for i := range podcasts {
		if urls[i] != podcasts[i].FeedURL {
			t.Errorf("expected %q, got %q", podcasts[i].FeedURL, urls[i])
		}
	}
}
