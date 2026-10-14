package render

import (
	"fmt"
	"image"
	"image/color"
	"strings"

	"flurrbit/internal/sprite"
)

const (
	upperHalfBlock = "▀"
	lowerHalfBlock = "▄"
	resetStyle     = "\x1b[0m"
)

func TerminalArt(frame *image.RGBA) []string {
	visible := sprite.VisibleBounds(frame)

	var lines []string
	for y := visible.Min.Y; y < visible.Max.Y; y += 2 {
		var line strings.Builder
		for x := visible.Min.X; x < visible.Max.X; x++ {
			topPixel := frame.RGBAAt(x, y)
			bottomPixel := color.RGBA{}
			if y+1 < visible.Max.Y {
				bottomPixel = frame.RGBAAt(x, y+1)
			}
			line.WriteString(halfBlockCell(topPixel, bottomPixel))
		}
		lines = append(lines, line.String())
	}

	return lines
}

func halfBlockCell(top, bottom color.RGBA) string {
	topVisible, bottomVisible := top.A != 0, bottom.A != 0

	switch {
	case topVisible && bottomVisible:
		return foregroundColor(top) + backgroundColor(bottom) + upperHalfBlock + resetStyle
	case topVisible:
		return foregroundColor(top) + upperHalfBlock + resetStyle
	case bottomVisible:
		return foregroundColor(bottom) + lowerHalfBlock + resetStyle
	}

	return " "
}

func foregroundColor(pixel color.RGBA) string {
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", pixel.R, pixel.G, pixel.B)
}

func backgroundColor(pixel color.RGBA) string {
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", pixel.R, pixel.G, pixel.B)
}

func visibleWidth(text string) int {
	width := 0
	insideEscapeSequence := false
	for _, character := range text {
		switch {
		case character == '\x1b':
			insideEscapeSequence = true
		case insideEscapeSequence:
			if character == 'm' {
				insideEscapeSequence = false
			}
		case isWideCharacter(character):
			width += 2
		default:
			width++
		}
	}

	return width
}

func isWideCharacter(character rune) bool {
	return (character >= 0x1100 && character <= 0x115F) ||
		(character >= 0x2E80 && character <= 0xA4CF) ||
		(character >= 0xAC00 && character <= 0xD7A3) ||
		(character >= 0xF900 && character <= 0xFAFF) ||
		(character >= 0xFE30 && character <= 0xFE4F) ||
		(character >= 0xFF00 && character <= 0xFF60) ||
		(character >= 0xFFE0 && character <= 0xFFE6)
}

func SideBySide(left, right []string, gap int) string {
	leftColumnWidth := 0
	for _, line := range left {
		leftColumnWidth = max(leftColumnWidth, visibleWidth(line))
	}

	var output strings.Builder
	for row := 0; row < max(len(left), len(right)); row++ {
		leftLine := ""
		if row < len(left) {
			leftLine = left[row]
		}
		output.WriteString(leftLine)

		if row < len(right) {
			output.WriteString(strings.Repeat(" ", leftColumnWidth-visibleWidth(leftLine)+gap))
			output.WriteString(right[row])
		}
		output.WriteString("\n")
	}

	return output.String()
}
