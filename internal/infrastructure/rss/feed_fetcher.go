package rss

import (
	"context"
	"fmt"
	"strings"
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

	episodes := make([]domain.Episode, 0, len(feed.Items))
	for _, item := range feed.Items {
		// Skip items without audio
		if len(item.Enclosures) == 0 {
			continue
		}

		episode := domain.Episode{
			Title:       item.Title,
			Description: item.Description,
			AudioURL:    item.Enclosures[0].URL,
		}

		// Parse published date safely
		if item.PublishedParsed != nil {
			episode.PublishedAt = *item.PublishedParsed
		}

		// Parse duration from iTunes extension if available
		if item.ITunesExt != nil && item.ITunesExt.Duration != "" {
			episode.PlaybackDuration = parseDuration(item.ITunesExt.Duration)
		}

		episodes = append(episodes, episode)
	}
	return podcast, episodes, nil
}

// parseDuration converts iTunes duration string (e.g., "2:09:56", "45:30") to seconds
func parseDuration(duration string) int {
	if duration == "" {
		return 0
	}

	parts := strings.Split(duration, ":")
	if len(parts) == 0 {
		return 0
	}

	var hours, minutes, seconds int

	switch len(parts) {
	case 1:
		// Format: SS (just seconds)
		fmt.Sscanf(duration, "%d", &seconds)
		return seconds
	case 2:
		// Format: MM:SS
		fmt.Sscanf(duration, "%d:%d", &minutes, &seconds)
		return minutes*60 + seconds
	case 3:
		// Format: HH:MM:SS
		fmt.Sscanf(duration, "%d:%d:%d", &hours, &minutes, &seconds)
		return hours*3600 + minutes*60 + seconds
	default:
		return 0
	}
}
