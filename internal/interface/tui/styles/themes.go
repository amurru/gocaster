package styles

import (
	"image/color"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
)

// ThemeConfig represents a custom theme that can be loaded from TOML
type ThemeConfig struct {
	Name       string `toml:"name"`
	Background string `toml:"background"`
	Surface    string `toml:"surface"`
	SurfaceAlt string `toml:"surface_alt"`
	Border     string `toml:"border"`
	Accent     string `toml:"accent"`
	AccentSoft string `toml:"accent_soft"`
	Text       string `toml:"text"`
	Muted      string `toml:"muted"`
	Success    string `toml:"success"`
	Danger     string `toml:"danger"`
	Warning    string `toml:"warning"`
}

// ToTheme converts a ThemeConfig to a Theme
func (tc ThemeConfig) ToTheme() Theme {
	return newTheme(
		lipgloss.Color(tc.Background),
		lipgloss.Color(tc.Surface),
		lipgloss.Color(tc.SurfaceAlt),
		lipgloss.Color(tc.Border),
		lipgloss.Color(tc.Accent),
		lipgloss.Color(tc.AccentSoft),
		lipgloss.Color(tc.Text),
		lipgloss.Color(tc.Muted),
		lipgloss.Color(tc.Success),
		lipgloss.Color(tc.Danger),
		lipgloss.Color(tc.Warning),
	)
}

// predefinedThemes contains all built-in themes
var predefinedThemes = map[string]func() Theme{
	"dark-red":     NewThemeDarkRed,
	"dark-orange":  NewThemeDarkOrange,
	"dark-yellow":  NewThemeDarkYellow,
	"dark-green":   NewThemeDarkGreen,
	"dark-blue":    NewThemeDarkBlue,
	"dark-indigo":  NewThemeDarkIndigo,
	"dark-violet":  NewThemeDarkViolet,
	"light-red":    NewThemeLightRed,
	"light-orange": NewThemeLightOrange,
	"light-yellow": NewThemeLightYellow,
	"light-green":  NewThemeLightGreen,
	"light-blue":   NewThemeLightBlue,
	"light-indigo": NewThemeLightIndigo,
	"light-violet": NewThemeLightViolet,
}

// GetPredefinedThemes returns a list of all predefined theme names
func GetPredefinedThemes() []string {
	names := make([]string, 0, len(predefinedThemes))
	names = append(names, "dark-red", "dark-orange", "dark-yellow", "dark-green", "dark-blue", "dark-indigo", "dark-violet")
	names = append(names, "light-red", "light-orange", "light-yellow", "light-green", "light-blue", "light-indigo", "light-violet")
	return names
}

// GetTheme returns a theme by name, from predefined themes only
func GetTheme(name string) (Theme, bool) {
	fn, ok := predefinedThemes[name]
	if !ok {
		return Theme{}, false
	}
	return fn(), true
}

// GetAllThemes returns a list of all available theme names (predefined and custom)
// Custom themes are discovered from the provided customThemesDir
func GetAllThemes(customThemesDir string) []string {
	// Start with predefined themes
	names := GetPredefinedThemes()

	// Add custom themes if directory is provided
	if customThemesDir != "" {
		customNames := getCustomThemeNames(customThemesDir)
		names = append(names, customNames...)
	}

	return names
}

// getCustomThemeNames discovers custom theme files in the given directory
func getCustomThemeNames(customThemesDir string) []string {
	entries, err := os.ReadDir(customThemesDir)
	if err != nil {
		return []string{}
	}

	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".toml") {
			// Extract theme name by removing .toml extension
			themeName := strings.TrimSuffix(entry.Name(), ".toml")
			names = append(names, themeName)
		}
	}

	return names
}

// Dark Themes

// NewThemeDarkRed creates a dark theme with red accents
func NewThemeDarkRed() Theme {
	background := lipgloss.Color("#111315")
	surface := lipgloss.Color("#1A1D1F")
	surfaceAlt := lipgloss.Color("#232729")
	border := lipgloss.Color("#3A403F")
	accent := lipgloss.Color("#E07A5F")
	accentSoft := lipgloss.Color("#F2CC8F")
	text := lipgloss.Color("#F4F1EA")
	muted := lipgloss.Color("#B8B0A2")
	success := lipgloss.Color("#81B29A")
	danger := lipgloss.Color("#E76F51")
	warning := lipgloss.Color("#F2CC8F")

	return newTheme(background, surface, surfaceAlt, border, accent, accentSoft, text, muted, success, danger, warning)
}

// NewThemeDarkOrange creates a dark theme with orange accents
func NewThemeDarkOrange() Theme {
	background := lipgloss.Color("#0F0E0E")
	surface := lipgloss.Color("#1A1816")
	surfaceAlt := lipgloss.Color("#242320")
	border := lipgloss.Color("#3A3530")
	accent := lipgloss.Color("#FF8C42")
	accentSoft := lipgloss.Color("#FFB380")
	text := lipgloss.Color("#F5F1EC")
	muted := lipgloss.Color("#B8AFA0")
	success := lipgloss.Color("#6BB6A0")
	danger := lipgloss.Color("#FF6B5B")
	warning := lipgloss.Color("#FFB380")

	return newTheme(background, surface, surfaceAlt, border, accent, accentSoft, text, muted, success, danger, warning)
}

// NewThemeDarkYellow creates a dark theme with yellow accents
func NewThemeDarkYellow() Theme {
	background := lipgloss.Color("#131210")
	surface := lipgloss.Color("#1D1A16")
	surfaceAlt := lipgloss.Color("#272320")
	border := lipgloss.Color("#3D3A34")
	accent := lipgloss.Color("#F4D03F")
	accentSoft := lipgloss.Color("#FEE565")
	text := lipgloss.Color("#F5F3EE")
	muted := lipgloss.Color("#B9B5A8")
	success := lipgloss.Color("#6FBF99")
	danger := lipgloss.Color("#FF6B5B")
	warning := lipgloss.Color("#FEE565")

	return newTheme(background, surface, surfaceAlt, border, accent, accentSoft, text, muted, success, danger, warning)
}

// NewThemeDarkGreen creates a dark theme with green accents
func NewThemeDarkGreen() Theme {
	background := lipgloss.Color("#0F1210")
	surface := lipgloss.Color("#1A1D1B")
	surfaceAlt := lipgloss.Color("#232723")
	border := lipgloss.Color("#3A3F3B")
	accent := lipgloss.Color("#52B788")
	accentSoft := lipgloss.Color("#74C69D")
	text := lipgloss.Color("#F4F5F1")
	muted := lipgloss.Color("#B8BFAA")
	success := lipgloss.Color("#52B788")
	danger := lipgloss.Color("#FF6B5B")
	warning := lipgloss.Color("#FFD60A")

	return newTheme(background, surface, surfaceAlt, border, accent, accentSoft, text, muted, success, danger, warning)
}

// NewThemeDarkBlue creates a dark theme with blue accents
func NewThemeDarkBlue() Theme {
	background := lipgloss.Color("#0F1419")
	surface := lipgloss.Color("#1A1F29")
	surfaceAlt := lipgloss.Color("#242C38")
	border := lipgloss.Color("#3A4556")
	accent := lipgloss.Color("#1F77F3")
	accentSoft := lipgloss.Color("#4A9FFF")
	text := lipgloss.Color("#F4F6FA")
	muted := lipgloss.Color("#B8C5D6")
	success := lipgloss.Color("#52B788")
	danger := lipgloss.Color("#FF6B5B")
	warning := lipgloss.Color("#FFD60A")

	return newTheme(background, surface, surfaceAlt, border, accent, accentSoft, text, muted, success, danger, warning)
}

// NewThemeDarkIndigo creates a dark theme with indigo accents
func NewThemeDarkIndigo() Theme {
	background := lipgloss.Color("#0E0F1A")
	surface := lipgloss.Color("#191B2F")
	surfaceAlt := lipgloss.Color("#23263E")
	border := lipgloss.Color("#3A3D54")
	accent := lipgloss.Color("#5B5FF0")
	accentSoft := lipgloss.Color("#8B8FFF")
	text := lipgloss.Color("#F4F5FA")
	muted := lipgloss.Color("#B8BADB")
	success := lipgloss.Color("#52B788")
	danger := lipgloss.Color("#FF6B5B")
	warning := lipgloss.Color("#FFD60A")

	return newTheme(background, surface, surfaceAlt, border, accent, accentSoft, text, muted, success, danger, warning)
}

// NewThemeDarkViolet creates a dark theme with violet accents
func NewThemeDarkViolet() Theme {
	background := lipgloss.Color("#13091A")
	surface := lipgloss.Color("#1E1229")
	surfaceAlt := lipgloss.Color("#2A1738")
	border := lipgloss.Color("#3F2A52")
	accent := lipgloss.Color("#C77DFF")
	accentSoft := lipgloss.Color("#E0AAFF")
	text := lipgloss.Color("#F5F3FA")
	muted := lipgloss.Color("#BAB3CC")
	success := lipgloss.Color("#52B788")
	danger := lipgloss.Color("#FF6B5B")
	warning := lipgloss.Color("#FFD60A")

	return newTheme(background, surface, surfaceAlt, border, accent, accentSoft, text, muted, success, danger, warning)
}

// Light Themes

// NewThemeLightRed creates a light theme with red accents
func NewThemeLightRed() Theme {
	background := lipgloss.Color("#FEFDFB")
	surface := lipgloss.Color("#F8F5F0")
	surfaceAlt := lipgloss.Color("#EDE8E0")
	border := lipgloss.Color("#D4CCC2")
	accent := lipgloss.Color("#D64045")
	accentSoft := lipgloss.Color("#E8707F")
	text := lipgloss.Color("#2C2622")
	muted := lipgloss.Color("#6B6560")
	success := lipgloss.Color("#5A9F7D")
	danger := lipgloss.Color("#D64045")
	warning := lipgloss.Color("#E8707F")

	return newTheme(background, surface, surfaceAlt, border, accent, accentSoft, text, muted, success, danger, warning)
}

// NewThemeLightOrange creates a light theme with orange accents
func NewThemeLightOrange() Theme {
	background := lipgloss.Color("#FFFBF5")
	surface := lipgloss.Color("#F8F3EB")
	surfaceAlt := lipgloss.Color("#EDDDD0")
	border := lipgloss.Color("#D4C0AE")
	accent := lipgloss.Color("#E8720C")
	accentSoft := lipgloss.Color("#F5A564")
	text := lipgloss.Color("#3C2D1F")
	muted := lipgloss.Color("#7A6B5D")
	success := lipgloss.Color("#5A9F7D")
	danger := lipgloss.Color("#E8720C")
	warning := lipgloss.Color("#F5A564")

	return newTheme(background, surface, surfaceAlt, border, accent, accentSoft, text, muted, success, danger, warning)
}

// NewThemeLightYellow creates a light theme with yellow accents
func NewThemeLightYellow() Theme {
	background := lipgloss.Color("#FFFEF9")
	surface := lipgloss.Color("#F9F8F1")
	surfaceAlt := lipgloss.Color("#EDEBE0")
	border := lipgloss.Color("#D4D0BD")
	accent := lipgloss.Color("#D4AF37")
	accentSoft := lipgloss.Color("#E8C76B")
	text := lipgloss.Color("#3C3824")
	muted := lipgloss.Color("#7A7360")
	success := lipgloss.Color("#5A9F7D")
	danger := lipgloss.Color("#D64045")
	warning := lipgloss.Color("#E8C76B")

	return newTheme(background, surface, surfaceAlt, border, accent, accentSoft, text, muted, success, danger, warning)
}

// NewThemeLightGreen creates a light theme with green accents
func NewThemeLightGreen() Theme {
	background := lipgloss.Color("#F8FCF9")
	surface := lipgloss.Color("#F0F6F1")
	surfaceAlt := lipgloss.Color("#DFE9E2")
	border := lipgloss.Color("#C8D9CE")
	accent := lipgloss.Color("#2D6A4F")
	accentSoft := lipgloss.Color("#52B788")
	text := lipgloss.Color("#2B3D2C")
	muted := lipgloss.Color("#6B7D6F")
	success := lipgloss.Color("#2D6A4F")
	danger := lipgloss.Color("#D64045")
	warning := lipgloss.Color("#E8C76B")

	return newTheme(background, surface, surfaceAlt, border, accent, accentSoft, text, muted, success, danger, warning)
}

// NewThemeLightBlue creates a light theme with blue accents
func NewThemeLightBlue() Theme {
	background := lipgloss.Color("#F5F8FC")
	surface := lipgloss.Color("#EBF1F7")
	surfaceAlt := lipgloss.Color("#DCE5ED")
	border := lipgloss.Color("#C7D7E8")
	accent := lipgloss.Color("#0066CC")
	accentSoft := lipgloss.Color("#3399FF")
	text := lipgloss.Color("#1A2D4C")
	muted := lipgloss.Color("#6B7F99")
	success := lipgloss.Color("#2D6A4F")
	danger := lipgloss.Color("#D64045")
	warning := lipgloss.Color("#E8C76B")

	return newTheme(background, surface, surfaceAlt, border, accent, accentSoft, text, muted, success, danger, warning)
}

// NewThemeLightIndigo creates a light theme with indigo accents
func NewThemeLightIndigo() Theme {
	background := lipgloss.Color("#F6F5FB")
	surface := lipgloss.Color("#EDE9F5")
	surfaceAlt := lipgloss.Color("#DDD5EA")
	border := lipgloss.Color("#C8BDDB")
	accent := lipgloss.Color("#4C3D99")
	accentSoft := lipgloss.Color("#7C6BB0")
	text := lipgloss.Color("#2D1F45")
	muted := lipgloss.Color("#7B6B8F")
	success := lipgloss.Color("#2D6A4F")
	danger := lipgloss.Color("#D64045")
	warning := lipgloss.Color("#E8C76B")

	return newTheme(background, surface, surfaceAlt, border, accent, accentSoft, text, muted, success, danger, warning)
}

// NewThemeLightViolet creates a light theme with violet accents
func NewThemeLightViolet() Theme {
	background := lipgloss.Color("#FAF6FC")
	surface := lipgloss.Color("#F1EBF5")
	surfaceAlt := lipgloss.Color("#DFD2EB")
	border := lipgloss.Color("#C8BADB")
	accent := lipgloss.Color("#6A0DAD")
	accentSoft := lipgloss.Color("#9D4EDD")
	text := lipgloss.Color("#36213E")
	muted := lipgloss.Color("#7B6B8F")
	success := lipgloss.Color("#2D6A4F")
	danger := lipgloss.Color("#D64045")
	warning := lipgloss.Color("#E8C76B")

	return newTheme(background, surface, surfaceAlt, border, accent, accentSoft, text, muted, success, danger, warning)
}

// newTheme is a helper function to create a Theme with the given colors
func newTheme(background, surface, surfaceAlt, border, accent, accentSoft, text, muted, success, danger, warning color.Color) Theme {
	return Theme{
		Background: background,
		Surface:    surface,
		SurfaceAlt: surfaceAlt,
		Border:     border,
		Accent:     accent,
		AccentSoft: accentSoft,
		Text:       text,
		Muted:      muted,
		Success:    success,
		Danger:     danger,
		Warning:    warning,
		App: lipgloss.NewStyle().
			Foreground(text).
			Padding(1, 2),
		Header: lipgloss.NewStyle().
			Foreground(text).
			Padding(0, 1).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(border).
			Bold(true),
		SectionTitle: lipgloss.NewStyle().
			Foreground(accentSoft).
			Bold(true),
		Panel: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(border).
			Padding(1),
		PanelFocused: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accent).
			Padding(1),
		Card: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(border).
			Padding(1),
		CardSelected: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accent).
			Padding(1),
		Label: lipgloss.NewStyle().
			Foreground(accentSoft).
			Bold(true),
		Body: lipgloss.NewStyle().
			Foreground(text),
		MutedText: lipgloss.NewStyle().
			Foreground(muted),
		StatusBar: lipgloss.NewStyle().
			Foreground(text).
			Border(lipgloss.NormalBorder(), true, false, false, false).
			BorderForeground(border).
			Padding(0, 1),
		HelpText: lipgloss.NewStyle().
			Foreground(muted),
		Badge: lipgloss.NewStyle().
			Foreground(accentSoft).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accentSoft).
			Padding(0, 1),
		BadgeNew: lipgloss.NewStyle().
			Foreground(success).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(success).
			Padding(0, 1).
			Bold(true),
		BadgePlayed: lipgloss.NewStyle().
			Foreground(muted).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(muted).
			Padding(0, 1),
		BadgeDownloaded: lipgloss.NewStyle().
			Foreground(success).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(success).
			Padding(0, 1),
		Input: lipgloss.NewStyle().
			Foreground(text).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(border).
			Padding(0, 1),
		InputFocused: lipgloss.NewStyle().
			Foreground(text).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accent).
			Padding(0, 1),
		Modal: lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(accent).
			Padding(1, 2),
		Divider: lipgloss.NewStyle().
			Foreground(border),
	}
}
