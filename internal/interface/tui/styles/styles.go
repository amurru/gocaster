package styles

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"

	"charm.land/lipgloss/v2"
	"github.com/BurntSushi/toml"
)

type Theme struct {
	Background      color.Color
	Surface         color.Color
	SurfaceAlt      color.Color
	Border          color.Color
	Accent          color.Color
	AccentSoft      color.Color
	Text            color.Color
	Muted           color.Color
	Success         color.Color
	Danger          color.Color
	Warning         color.Color
	App             lipgloss.Style
	Header          lipgloss.Style
	SectionTitle    lipgloss.Style
	Panel           lipgloss.Style
	PanelFocused    lipgloss.Style
	Card            lipgloss.Style
	CardSelected    lipgloss.Style
	Label           lipgloss.Style
	Body            lipgloss.Style
	MutedText       lipgloss.Style
	StatusBar       lipgloss.Style
	HelpText        lipgloss.Style
	Badge           lipgloss.Style
	BadgeNew        lipgloss.Style
	BadgePlayed     lipgloss.Style
	BadgeDownloaded lipgloss.Style
	Input           lipgloss.Style
	InputFocused    lipgloss.Style
	Modal           lipgloss.Style
	Divider         lipgloss.Style
}

// NewTheme returns the default theme (dark-red)
func NewTheme() Theme {
	return NewThemeDarkRed()
}

// LoadTheme attempts to load a theme by name, first checking predefined themes,
// then custom themes in the config folder. Falls back to dark-red if not found.
func LoadTheme(themeName string, customThemesDir string) Theme {
	// Try predefined themes first
	if theme, ok := GetTheme(themeName); ok {
		return theme
	}

	// Try custom themes if themeName is specified and customThemesDir is provided
	if themeName != "" && customThemesDir != "" {
		if theme, err := loadCustomTheme(themeName, customThemesDir); err == nil {
			return theme
		}
	}

	// Fallback to default theme silently
	return NewThemeDarkRed()
}

func (t Theme) StatusStyle(kind string) lipgloss.Style {
	style := lipgloss.NewStyle().
		Foreground(t.Text).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Padding(0, 1)

	switch kind {
	case "success":
		return style.BorderForeground(t.Success).Foreground(t.Success)
	case "error":
		return style.BorderForeground(t.Danger).Foreground(t.Danger)
	case "warning":
		return style.BorderForeground(t.Warning).Foreground(t.Warning)
	default:
		return style
	}
}

// loadCustomTheme loads a custom theme from a TOML file in the custom themes directory.
// It looks for ~/.config/gocaster/themes/{themeName}.toml and parses it.
// Returns an error if the file doesn't exist or can't be parsed.
func loadCustomTheme(themeName, customThemesDir string) (Theme, error) {
	if themeName == "" || customThemesDir == "" {
		return Theme{}, fmt.Errorf("theme name and custom themes directory required")
	}

	themePath := fmt.Sprintf("%s.toml", filepath.Join(customThemesDir, themeName))

	// Check if file exists
	if _, err := os.Stat(themePath); os.IsNotExist(err) {
		return Theme{}, fmt.Errorf("custom theme file not found: %s", themePath)
	}

	// Read and parse TOML file
	var config struct {
		Theme ThemeConfig `toml:"theme"`
	}

	if _, err := toml.DecodeFile(themePath, &config); err != nil {
		return Theme{}, fmt.Errorf("failed to parse theme file %s: %w", themePath, err)
	}

	// Validate that all required color fields are set
	if config.Theme.Background == "" || config.Theme.Text == "" {
		return Theme{}, fmt.Errorf("custom theme %s missing required fields (background, text)", themeName)
	}

	return config.Theme.ToTheme(), nil
}
