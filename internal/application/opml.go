package application

import (
	"encoding/xml"

	"github.com/amurru/gocaster/internal/domain"
)

type OPML struct {
	XMLName xml.Name `xml:"opml"`
	Version string   `xml:"version,attr"`
	Head    Head     `xml:"head"`
	Body    Body     `xml:"body"`
}

type Head struct {
	Title string `xml:"title"`
}

type Body struct {
	Outlines []Outline `xml:"outline"`
}

type Outline struct {
	Type   string `xml:"type,attr"`
	Text   string `xml:"text,attr"`
	XMLURL string `xml:"xmlUrl,attr"`
}

func ExportOPML(podcasts []domain.Podcast) ([]byte, error) {
	opml := OPML{
		Version: "2.0",
		Head: Head{
			Title: "Gocaster Subscriptions",
		},
	}
	for _, podcast := range podcasts {
		outline := Outline{
			Type:   "rss",
			Text:   podcast.Title,
			XMLURL: podcast.FeedURL,
		}

		opml.Body.Outlines = append(opml.Body.Outlines, outline)
	}
	data, err := xml.MarshalIndent(opml, "", "  ")
	if err != nil {
		return nil, err
	}

	return append([]byte(xml.Header), data...), nil
}

func ImportOPML(data []byte) ([]string, error) {
	var opml OPML

	if err := xml.Unmarshal(data, &opml); err != nil {
		return nil, err
	}

	urls := make([]string, 0, len(opml.Body.Outlines))
	for _, outline := range opml.Body.Outlines {
		urls = append(urls, outline.XMLURL)
	}

	return urls, nil
}
