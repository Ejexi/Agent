package tui

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// BrailleOffset is the Unicode code point for the empty Braille pattern.
const BrailleOffset = 0x2800

// BrailleMasks maps sub-pixel positions to Braille dot bit masks.
var BrailleMasks = [4][2]int{
	{0x01, 0x08},
	{0x02, 0x10},
	{0x04, 0x20},
	{0x40, 0x80},
}

// RenderImageToBraille converts a PNG/JPEG image to a Braille-art string.
func RenderImageToBraille(path string, width, height int) (string, error) {
	reader, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	img, _, err := image.Decode(reader)
	if err != nil {
		return "", err
	}

	bounds := img.Bounds()
	imgW, imgH := bounds.Dx(), bounds.Dy()

	scaleX := float64(imgW) / float64(width*2)
	scaleY := float64(imgH) / float64(height*4)

	baseStyle := lipgloss.NewStyle().Padding(0).Margin(0)
	fgStyle := baseStyle.Foreground(Theme.Text)

	var builder strings.Builder

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			var mask int

			for by := 0; by < 4; by++ {
				for bx := 0; bx < 2; bx++ {
					subX := float64(x*2 + bx)
					subY := float64(y*4 + by)

					x0 := int(subX * scaleX)
					y0 := int(subY * scaleY)
					x1 := int((subX + 1) * scaleX)
					y1 := int((subY + 1) * scaleY)

					if x1 >= imgW {
						x1 = imgW - 1
					}
					if y1 >= imgH {
						y1 = imgH - 1
					}
					if x1 < x0 {
						x1 = x0
					}
					if y1 < y0 {
						y1 = y0
					}

					var weightedLumSum float64
					pixelCount := (x1 - x0 + 1) * (y1 - y0 + 1)

					for iy := y0; iy <= y1; iy++ {
						for ix := x0; ix <= x1; ix++ {
							r, g, b, a := img.At(ix, iy).RGBA()
							aF := float64(a) / 65535.0
							lum := 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)
							weightedLumSum += aF * lum
						}
					}

					if pixelCount > 0 {
						avgWeightedLum := weightedLumSum / float64(pixelCount)
						if avgWeightedLum > 12000 {
							mask |= BrailleMasks[by][bx]
						}
					}
				}
			}

			char := string(rune(BrailleOffset + mask))
			if mask > 0 {
				builder.WriteString(fgStyle.Render(char))
			} else {
				builder.WriteString(baseStyle.Render(" "))
			}
		}
		builder.WriteString("\n")
	}
	return builder.String(), nil
}

// RenderImageToHalfBlocks converts an image to a colored string using
// half-block characters (▀▄█). This version uses area-averaging for smoothness
// and applies a premium theme-aware color mapping.
func RenderImageToHalfBlocks(path string, width int) (string, error) {
	reader, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	img, _, err := image.Decode(reader)
	if err != nil {
		return "", err
	}

	bounds := img.Bounds()
	imgW, imgH := bounds.Dx(), bounds.Dy()

	// Terminals usually have a 1:2 or 1:2.1 aspect ratio (width:height).
	// Since each half-block is 1x2 pixels, a square image in pixels should
	// be represented as a square in character cells.
	aspect := float64(imgH) / float64(imgW)
	height := int(float64(width) * aspect * 0.5 * 2.1) // 0.5 because 2 pixels per row, 2.1 for char aspect
	if height < 1 {
		height = 1
	}

	scaleX := float64(imgW) / float64(width)
	scaleY := float64(imgH) / float64(height*2)

	var builder strings.Builder

	// OpenCode-inspired palette: Grey to White
	greyCol := lipgloss.AdaptiveColor{Light: "#777777", Dark: "#919191"}
	whiteCol := lipgloss.AdaptiveColor{Light: "#333333", Dark: "#FFFFFF"}

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Area sample for top pixel
			_, tA := sampleArea(img, float64(x)*scaleX, float64(y*2)*scaleY, scaleX, scaleY)
			// Area sample for bottom pixel
			_, bA := sampleArea(img, float64(x)*scaleX, float64(y*2+1)*scaleY, scaleX, scaleY)

			if tA < 0.1 && bA < 0.1 {
				builder.WriteString(" ")
				continue
			}

			// Calculate a horizontal gradient ratio
			gradRatio := float64(x) / float64(width)

			// OpenCode transition: smooth blend from grey to white around the midpoint
			color := interpolateAdaptive(greyCol, whiteCol, gradRatio)

			// Map luminance to "glow" - brighter areas get higher brightness
			// Since our theme colors are already curated, we just use them.

			style := lipgloss.NewStyle()

			if tA >= 0.5 && bA >= 0.5 {
				builder.WriteString(style.Foreground(color).Render("█"))
			} else if tA >= 0.5 {
				builder.WriteString(style.Foreground(color).Render("▀"))
			} else if bA >= 0.5 {
				builder.WriteString(style.Foreground(color).Render("▄"))
			} else {
				// Very light pixels or semi-transparent edges
				builder.WriteString(" ")
			}
		}
		builder.WriteString("\n")
	}

	return strings.TrimSuffix(builder.String(), "\n"), nil
}

// sampleArea calculates average luminance and alpha in a rectangular area.
func sampleArea(img image.Image, x0, y0, w, h float64) (float64, float64) {
	var sumLum, sumA float64
	count := 0.0

	for iy := int(y0); iy < int(y0+h); iy++ {
		for ix := int(x0); ix < int(x0+w); ix++ {
			r, g, b, a := img.At(ix, iy).RGBA()
			lum := (0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) / 65535.0
			sumLum += lum
			sumA += float64(a) / 65535.0
			count++
		}
	}

	if count == 0 {
		return 0, 0
	}
	return sumLum / count, sumA / count
}

// interpolateAdaptive blends two Lipgloss AdaptiveColors.
func interpolateAdaptive(c1, c2 lipgloss.AdaptiveColor, t float64) lipgloss.Color {
	// For simplicity, we just interpolate the Dark variant as it's most common in TUI
	// A more robust version would interpolate both.

	// Parse hex (assuming #RRGGBB)
	parse := func(s string) (uint8, uint8, uint8) {
		if len(s) < 7 {
			return 0, 0, 0
		}
		var r, g, b uint8
		fmt.Sscanf(s, "#%02x%02x%02x", &r, &g, &b)
		return r, g, b
	}

	r1, g1, b1 := parse(c1.Dark)
	r2, g2, b2 := parse(c2.Dark)

	r := uint8(float64(r1) + float64(int(r2)-int(r1))*t)
	g := uint8(float64(g1) + float64(int(g2)-int(g1))*t)
	b := uint8(float64(b1) + float64(int(b2)-int(b1))*t)

	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", r, g, b))
}
