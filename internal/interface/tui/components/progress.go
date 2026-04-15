package components

import (
	"fmt"

	"github.com/amurru/gocaster/internal/interface/tui/styles"
)

func RenderLoading(theme styles.Theme, spinnerView, label string) string {
	return theme.Card.Render(fmt.Sprintf("%s %s", spinnerView, label))
}
