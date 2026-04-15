package rss

import (
	"context"
	"time"

	"github.com/amurru/gocaster/internal/domain"
	"github.com/mmcdole/gofeed"
)

type FeedFetcher struct{}

func NewFeedFetcher() *FeedFetcher {
	return &FeedFetcher{}
}

func (f *FeedFetcher) Parse(url string) (*domain.Podcast, []domain.Episode, error) {
	fp := gofeed.NewParser()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	feed, err := fp.ParseURLWithContext(url, ctx)
	if err != nil {
		return nil, nil, err
	}

	// Map gofeed structs to domain entities
	podcast := &domain.Podcast{
		Title:       feed.Title,
		FeedURL:     url,
		Description: feed.Description,
	}
	episodes := make([]domain.Episode, len(feed.Items))
	for i, item := range feed.Items {
		episodes[i] = domain.Episode{
			Title:       item.Title,
			Description: item.Description,
			AudioURL:    item.Enclosures[0].URL,
			PublishedAt: *item.PublishedParsed,
			// PlaybackDuration: item.ITunesExt.Duration,
		}
	}
	return podcast, episodes, nil
}
