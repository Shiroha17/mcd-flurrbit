package render

import (
	"image"
	"image/color"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"io"

	"golang.org/x/image/font"

	"flurrbit/internal/pet"
	"flurrbit/internal/sprite"
)

var sceneBackground = color.RGBA{0xff, 0xf4, 0xdc, 0xff}

const (
	scenePadding   = 16
	bubbleMaxWidth = 170
	bubbleGap      = 4
)

const (
	maxGIFPaletteSize = 256
	inkToFillSteps    = 16
)

type Scene struct {
	State pet.State
	Face  font.Face
	Line  string
}

func (s Scene) Frame(tick int) *image.RGBA {
	pose := sprite.Pose{Mood: s.State.Mood, Stage: s.State.Stage, Tick: tick}
	petImage := sprite.Scale(sprite.Draw(pose), sprite.DisplayScale)

	var bubble *image.RGBA
	if s.Face != nil && s.Line != "" {
		bubble = Bubble(s.Face, s.Line, bubbleMaxWidth)
	}

	width := petImage.Bounds().Dx()
	height := petImage.Bounds().Dy()
	if bubble != nil {
		width = max(width, bubble.Bounds().Dx())
		height += bubble.Bounds().Dy() + bubbleGap
	}
	width += 2 * scenePadding
	height += 2 * scenePadding

	frame := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(frame, frame.Bounds(), image.NewUniform(sceneBackground), image.Point{}, draw.Src)

	contentTop := scenePadding
	if bubble != nil {
		overlay(frame, bubble, image.Pt((width-bubble.Bounds().Dx())/2, contentTop))
		contentTop += bubble.Bounds().Dy() + bubbleGap
	}
	overlay(frame, petImage, image.Pt((width-petImage.Bounds().Dx())/2, contentTop))

	return frame
}

func WriteGIF(output io.Writer, scene Scene, frameCount, frameDelayHundredths int) error {
	animation := &gif.GIF{LoopCount: 0}
	colors := gifPalette()

	for tick := 0; tick < frameCount; tick++ {
		frame := scene.Frame(tick)
		palettedFrame := image.NewPaletted(frame.Bounds(), colors)
		draw.Draw(palettedFrame, frame.Bounds(), frame, image.Point{}, draw.Src)

		animation.Image = append(animation.Image, palettedFrame)
		animation.Delay = append(animation.Delay, frameDelayHundredths)
	}

	return gif.EncodeAll(output, animation)
}

func gifPalette() color.Palette {
	colors := color.Palette{sceneBackground, bubbleFill, bubbleInk}
	for _, swatch := range sprite.Swatches() {
		colors = append(colors, swatch)
	}
	for step := 1; step < inkToFillSteps; step++ {
		colors = append(colors, blendColors(bubbleInk, bubbleFill, step, inkToFillSteps))
	}
	for _, webSafeColor := range palette.WebSafe {
		if len(colors) >= maxGIFPaletteSize {
			break
		}
		colors = append(colors, webSafeColor)
	}

	return colors
}

func blendColors(from, to color.RGBA, step, totalSteps int) color.RGBA {
	blendChannel := func(start, end uint8) uint8 {
		return uint8((int(start)*(totalSteps-step) + int(end)*step) / totalSteps)
	}

	return color.RGBA{blendChannel(from.R, to.R), blendChannel(from.G, to.G), blendChannel(from.B, to.B), 0xff}
}
