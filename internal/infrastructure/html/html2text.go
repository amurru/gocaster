// Package html provides utilities for converting HTML content to plain text.
package html

import (
	"fmt"

	"github.com/k3a/html2text"
)

// ConvertToText converts an HTML string to plain text, stripping tags and
// preserving link URLs as inline text. It returns an error for empty input.
func ConvertToText(htmlContent string) (string, error) {
	if htmlContent == "" {
		return "", fmt.Errorf("html2text: empty input")
	}
	return html2text.HTML2Text(htmlContent), nil
}
