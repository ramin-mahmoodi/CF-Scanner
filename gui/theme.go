package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

//go:generate go run fyne.io/fyne/v2/cmd/fyne bundle -o bundled.go fonts/Vazirmatn-Regular.ttf

type CustomTheme struct {
	isDark bool
	font   fyne.Resource
}

func NewCustomTheme(isDark bool, fontRes fyne.Resource) fyne.Theme {
	return &CustomTheme{isDark: isDark, font: fontRes}
}

func (m CustomTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	v := theme.VariantLight
	if m.isDark {
		v = theme.VariantDark
	}

	// Make card background noticeably lighter than the window background in dark theme
	if m.isDark && name == theme.ColorNameOverlayBackground {
		return color.NRGBA{R: 0x4A, G: 0x4A, B: 0x4A, A: 0xFF}
	}

	// Remove all shadows (specifically to hide the scrollbar gradient shadows)
	if name == theme.ColorNameShadow {
		return color.Transparent
	}

	// Darker, more elegant colors for Start/Stop buttons
	if name == theme.ColorNameSuccess {
		return color.NRGBA{R: 0x24, G: 0x8C, B: 0x46, A: 0xFF} // Elegant Dark Green
	}
	if name == theme.ColorNameError {
		return color.NRGBA{R: 0xC6, G: 0x28, B: 0x28, A: 0xFF} // Elegant Dark Red
	}

	if name == theme.ColorNameWarning {
		return color.NRGBA{R: 0xC8, G: 0x86, B: 0x0B, A: 0xFF} // Dark Goldenrod (Same for both themes)
	}
	return theme.DefaultTheme().Color(name, v)
}

func (m CustomTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (m CustomTheme) Font(style fyne.TextStyle) fyne.Resource {
	if m.font != nil {
		return m.font
	}
	return theme.DefaultTheme().Font(style)
}

func (m CustomTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}
