package render

import (
	_ "embed"
	"fmt"
	"os"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

const pixelFontSize = 16

//go:embed fonts/fusion-pixel-12px-proportional-zh_hans.ttf
var embeddedPixelFont []byte

func LoadFace(customFontPath string) (font.Face, error) {
	if customFontPath == "" {
		customFontPath = os.Getenv("FLURRBIT_FONT")
	}

	fontData := embeddedPixelFont
	if customFontPath != "" {
		customFontData, err := os.ReadFile(customFontPath)
		if err != nil {
			return nil, fmt.Errorf("字体文件不可用: %w", err)
		}
		fontData = customFontData
	}

	collection, err := opentype.ParseCollection(fontData)
	if err != nil {
		return nil, fmt.Errorf("无法解析字体: %w", err)
	}

	parsedFont, err := collection.Font(0)
	if err != nil {
		return nil, err
	}

	return opentype.NewFace(parsedFont, &opentype.FaceOptions{Size: pixelFontSize, DPI: 72, Hinting: font.HintingFull})
}
