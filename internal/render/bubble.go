package render

import (
	"image"
	"image/color"
	"image/draw"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
	"golang.org/x/text/unicode/norm"

	"flurrbit/internal/sprite"
)

var (
	bubbleFill = color.RGBA{0xff, 0xff, 0xff, 0xff}
	bubbleInk  = color.RGBA{0x2b, 0x1d, 0x18, 0xff}
)

const (
	bubblePixelScale   = 1
	bubblePadding      = 5
	bubbleBorderWidth  = 1
	bubbleCornerRadius = 3
	bubbleTailHeight   = 5
	bubbleLineSpacing  = 2
)

const (
	forbiddenLineStarts       = "，。！？、；：」』）》…!?,.;:)]"
	asciiPunctuationEndsWords = "!?;:)]"
)

func Bubble(face font.Face, text string, maxWidth int) *image.RGBA {
	maxTextWidth := maxWidth/bubblePixelScale - 2*bubblePadding
	bubble := drawBubble(face, keepDrawableCharacters(face, text), maxTextWidth)

	return sprite.Scale(bubble, bubblePixelScale)
}

func keepDrawableCharacters(face font.Face, text string) string {
	hasGlyph := func(character rune) bool {
		_, ok := face.GlyphAdvance(character)

		return ok
	}

	var drawable strings.Builder
	for _, character := range text {
		if character == '\n' || hasGlyph(character) {
			drawable.WriteRune(character)
			continue
		}

		for _, plainCharacter := range norm.NFKC.String(string(character)) {
			if hasGlyph(plainCharacter) {
				drawable.WriteRune(plainCharacter)
			}
		}
	}

	return drawable.String()
}

func wrapText(face font.Face, text string, maxWidth int) []string {
	maxLineWidth := fixed.I(maxWidth)

	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		currentLine := ""
		for _, token := range splitTokens(paragraph) {
			extendedLine := currentLine + token
			overflows := font.MeasureString(face, extendedLine) > maxLineWidth
			startsWithClosingPunctuation := strings.ContainsRune(forbiddenLineStarts, []rune(token)[0])
			if currentLine != "" && overflows && !startsWithClosingPunctuation {
				lines = append(lines, strings.TrimRight(currentLine, " "))
				extendedLine = strings.TrimLeft(token, " ")
			}
			currentLine = extendedLine
		}
		lines = append(lines, currentLine)
	}

	return lines
}

func splitTokens(text string) []string {
	var tokens []string
	word := ""
	for _, character := range text {
		if isWordCharacter(character) {
			word += string(character)
			continue
		}

		if word != "" {
			tokens = append(tokens, word)
			word = ""
		}
		tokens = append(tokens, string(character))
	}

	if word != "" {
		tokens = append(tokens, word)
	}

	return tokens
}

func isWordCharacter(character rune) bool {
	isASCII := character < 0x80

	return isASCII && character != ' ' && !strings.ContainsRune(asciiPunctuationEndsWords, character)
}

func drawBubble(face font.Face, text string, maxTextWidth int) *image.RGBA {
	lines := wrapText(face, text, maxTextWidth)
	metrics := face.Metrics()
	lineHeight := (metrics.Ascent + metrics.Descent).Ceil()

	textWidth := 0
	for _, line := range lines {
		textWidth = max(textWidth, font.MeasureString(face, line).Ceil())
	}
	bodyWidth := textWidth + 2*bubblePadding
	bodyHeight := len(lines)*lineHeight + (len(lines)-1)*bubbleLineSpacing + 2*bubblePadding

	bubble := image.NewRGBA(image.Rect(0, 0, bodyWidth, bodyHeight+bubbleTailHeight))
	outerBody := image.Rect(0, 0, bodyWidth, bodyHeight)
	innerBody := outerBody.Inset(bubbleBorderWidth)
	fillRoundedRect(bubble, outerBody, bubbleCornerRadius, bubbleInk)
	fillRoundedRect(bubble, innerBody, bubbleCornerRadius-bubbleBorderWidth, bubbleFill)
	drawTail(bubble, bodyWidth/2, bodyHeight)

	drawer := font.Drawer{Dst: bubble, Src: image.NewUniform(bubbleInk), Face: face}
	for index, line := range lines {
		baseline := bubblePadding + metrics.Ascent.Ceil() + index*(lineHeight+bubbleLineSpacing)
		drawer.Dot = fixed.P(bubblePadding, baseline)
		drawer.DrawString(line)
	}

	return bubble
}

func fillRoundedRect(target *image.RGBA, area image.Rectangle, radius int, fill color.RGBA) {
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			if isInsideRoundedRect(x, y, area, radius) {
				target.SetRGBA(x, y, fill)
			}
		}
	}
}

func isInsideRoundedRect(x, y int, area image.Rectangle, radius int) bool {
	nearestCornerCenterX := min(max(x, area.Min.X+radius), area.Max.X-1-radius)
	nearestCornerCenterY := min(max(y, area.Min.Y+radius), area.Max.Y-1-radius)
	deltaX, deltaY := x-nearestCornerCenterX, y-nearestCornerCenterY

	return deltaX*deltaX+deltaY*deltaY <= radius*radius
}

func drawTail(target *image.RGBA, centerX, top int) {
	for row := 0; row < bubbleTailHeight; row++ {
		halfWidth := bubbleTailHeight - row
		left, right := centerX-halfWidth, centerX+halfWidth
		isBottomRow := row == bubbleTailHeight-1

		for x := left; x <= right; x++ {
			pixel := bubbleFill
			if x == left || x == right || isBottomRow {
				pixel = bubbleInk
			}
			target.SetRGBA(x, top-bubbleBorderWidth+row, pixel)
		}
	}
}

func overlay(target draw.Image, source image.Image, at image.Point) {
	draw.Draw(target, source.Bounds().Add(at), source, source.Bounds().Min, draw.Over)
}
