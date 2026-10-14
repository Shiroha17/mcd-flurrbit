package sprite

import (
	"image"
	"image/color"
	"image/draw"
	"math"

	"flurrbit/internal/pet"
)

const (
	Width          = 32
	Height         = 40
	DisplayScale   = 4
	AnimationCycle = 48

	centerX      = 16
	rimRow       = 19
	cupBottomRow = 37
	cupTopWidth  = 16
	cupBaseWidth = 12
	waveRow      = 34
	eyeTopRow    = 26
	mouthRow     = 30

	blinkPeriodTicks = 16
)

type Pose struct {
	Mood       pet.Mood
	Stage      pet.Stage
	Tick       int
	Walking    bool
	FacingLeft bool
}

type shades struct {
	light, body, dark color.RGBA
}

var (
	paper      = shades{light: rgb(0xff, 0xff, 0xff), body: rgb(0xf6, 0xf7, 0xfb), dark: rgb(0xd8, 0xdc, 0xe6)}
	cream      = shades{light: rgb(0xff, 0xff, 0xff), body: rgb(0xfb, 0xf6, 0xec), dark: rgb(0xe2, 0xd9, 0xc8)}
	berryCream = shades{light: rgb(0xff, 0xf4, 0xf6), body: rgb(0xfc, 0xe4, 0xea), dark: rgb(0xec, 0xc4, 0xce)}
	blueWave   = shades{light: rgb(0x8c, 0xc2, 0xf5), body: rgb(0x3d, 0x8f, 0xe3), dark: rgb(0x2a, 0x6c, 0xb8)}
	goldTrim   = shades{light: rgb(0xff, 0xe7, 0x8c), body: rgb(0xff, 0xc8, 0x3a), dark: rgb(0xd9, 0x94, 0x16)}
	cupInside  = rgb(0xb4, 0xba, 0xc8)
	crumb      = rgb(0xb3, 0x9e, 0x92)
	strawberry = rgb(0xee, 0x5f, 0x84)
	outline    = rgb(0x3b, 0x3d, 0x4c)
	ink        = rgb(0x2a, 0x1e, 0x1a)
	white      = rgb(0xff, 0xff, 0xff)
	cheek      = rgb(0xff, 0x9e, 0xaa)
	tongue     = rgb(0xff, 0x6e, 0x7c)
	sweat      = rgb(0x84, 0xd4, 0xff)
	sparkle    = rgb(0xff, 0xe9, 0x8a)
	crownGold  = rgb(0xff, 0xd4, 0x3c)
	crownJewel = rgb(0xe8, 0x3a, 0x2c)
	sleepZ     = rgb(0xa8, 0xb8, 0xf0)
)

func rgb(red, green, blue uint8) color.RGBA {
	return color.RGBA{red, green, blue, 0xff}
}

type canvas struct {
	image   *image.RGBA
	offsetX int
	offsetY int
}

func (c *canvas) set(x, y int, pixel color.RGBA) {
	c.image.SetRGBA(x+c.offsetX, y+c.offsetY, pixel)
}

func (c *canvas) rect(left, top, width, height int, pixel color.RGBA) {
	for y := top; y < top+height; y++ {
		for x := left; x < left+width; x++ {
			c.set(x, y, pixel)
		}
	}
}

func (c *canvas) points(pixel color.RGBA, coordinates ...[2]int) {
	for _, point := range coordinates {
		c.set(point[0], point[1], pixel)
	}
}

func Draw(pose Pose) *image.RGBA {
	offsetX, offsetY := bodyOffset(pose)
	frame := image.NewRGBA(image.Rect(0, 0, Width, Height))
	body := &canvas{image: frame, offsetX: offsetX, offsetY: offsetY}
	background := &canvas{image: frame}

	iceCreamTop := drawIceCream(body, pose)
	drawCup(body, pose)
	drawArms(body, pose)
	drawMelting(body, pose)
	addOutline(frame)

	drawEyes(body, pose)
	drawCheeks(body, pose)
	drawMouth(body, pose)
	if pose.Stage == pet.DeluxeFlurry {
		drawCrown(body, iceCreamTop)
	}
	drawEffects(body, background, iceCreamTop, pose)

	if pose.FacingLeft {
		return mirror(frame)
	}

	return frame
}

func bodyOffset(pose Pose) (int, int) {
	phase := pose.Tick % 4

	switch pose.Mood {
	case pet.Excited:
		return 0, []int{0, -1, -2, -1}[phase]
	case pet.Anxious:
		return []int{-1, 0, 1, 0}[phase], 0
	case pet.Sleepy:
		return 0, []int{0, 0, 1, 1}[pose.Tick/2%4]
	case pet.Eating:
		return 0, []int{0, -1, 0, -1}[phase]
	case pet.Starving:
		return 0, 0
	}

	if pose.Walking {
		return 0, []int{0, -2, -3, -1}[phase]
	}

	return 0, []int{0, 0, -1, 0}[phase]
}

func cupEdges(row int) (int, int) {
	depth := float64(row-rimRow) / float64(cupBottomRow-rimRow)
	halfWidth := float64(cupTopWidth)/2 - depth*float64(cupTopWidth-cupBaseWidth)/2
	roundedHalfWidth := int(math.Round(halfWidth))

	return centerX - roundedHalfWidth, centerX + roundedHalfWidth - 1
}

func iceCreamShades(stage pet.Stage) shades {
	if stage >= pet.StrawberryFlurry {
		return berryCream
	}

	return cream
}

var middleScoopLift = map[pet.Stage]float64{
	pet.MiniFlurry:       0,
	pet.ClassicFlurry:    1,
	pet.StrawberryFlurry: 1,
	pet.DeluxeFlurry:     2,
}

func drawIceCream(c *canvas, pose Pose) int {
	iceCream := iceCreamShades(pose.Stage)
	floorRow := rimRow - 1

	switch pose.Mood {
	case pet.Starving:
		return rimRow
	case pet.Hungry:
		return drawScoop(c, iceCream, centerX, float64(floorRow)+0.5, 8.6, 2.2, floorRow)
	}

	lift := middleScoopLift[pose.Stage]
	topRow := drawScoop(c, iceCream, centerX-4.5, float64(floorRow)-1, 4.6, 3.2, floorRow)
	topRow = min(topRow, drawScoop(c, iceCream, centerX+4.5, float64(floorRow)-1, 4.6, 3.2, floorRow))
	topRow = min(topRow, drawScoop(c, iceCream, centerX, float64(floorRow)-3-lift, 4.8, 3.6+lift*0.6, floorRow))

	if pose.Stage >= pet.StrawberryFlurry {
		c.points(strawberry,
			[2]int{centerX - 2, topRow + 1}, [2]int{centerX + 1, topRow + 2},
			[2]int{centerX - 6, floorRow - 3}, [2]int{centerX + 5, floorRow - 3})
	}

	return topRow
}

func drawScoop(c *canvas, iceCream shades, middleX, middleY, radiusX, radiusY float64, floorRow int) int {
	topRow := floorRow
	for row := int(middleY - radiusY - 1); row <= floorRow; row++ {
		for column := int(middleX - radiusX - 1); column <= int(middleX+radiusX+1); column++ {
			normalizedX := (float64(column) + 0.5 - middleX) / radiusX
			normalizedY := (float64(row) + 0.5 - middleY) / radiusY
			if normalizedX*normalizedX+normalizedY*normalizedY > 1 {
				continue
			}

			pixel := iceCream.body
			switch {
			case normalizedX+normalizedY < -0.9:
				pixel = iceCream.light
			case normalizedX+normalizedY > 0.75:
				pixel = iceCream.dark
			}
			c.set(column, row, pixel)
			topRow = min(topRow, row)
		}
	}

	return topRow
}

func drawCup(c *canvas, pose Pose) {
	wave, trim := blueWave, paper
	if pose.Stage == pet.DeluxeFlurry {
		wave, trim = goldTrim, goldTrim
	}

	for row := rimRow; row <= cupBottomRow; row++ {
		left, right := cupEdges(row)
		for x := left; x <= right; x++ {
			pixel := paper.body
			switch {
			case row == cupBottomRow || x >= right-1:
				pixel = paper.dark
			case x == left:
				pixel = paper.light
			}
			c.set(x, row, pixel)
		}
	}

	waveLeft, waveRight := cupEdges(waveRow)
	for x := waveLeft + 1; x < waveRight; x++ {
		waveOffset := int(math.Round(math.Sin(float64(x) * 0.8)))
		c.set(x, waveRow+waveOffset, wave.body)
		c.set(x, waveRow+waveOffset+1, wave.dark)
	}

	rimLeft, rimRight := cupEdges(rimRow)
	for x := rimLeft - 2; x <= rimRight+2; x++ {
		topPixel, underPixel := trim.light, trim.body
		if x >= rimRight+1 {
			topPixel, underPixel = trim.body, trim.dark
		}
		c.set(x, rimRow, topPixel)
		c.set(x, rimRow+1, underPixel)
	}

	if pose.Mood == pet.Starving {
		c.rect(rimLeft+1, rimRow, rimRight-rimLeft-1, 1, cupInside)
	}
}

func drawArms(c *canvas, pose Pose) {
	leftLift, rightLift := 0, 0
	switch pose.Mood {
	case pet.Excited:
		if pose.Tick%2 == 0 {
			leftLift = 2
		} else {
			rightLift = 2
		}
	case pet.Anxious:
		leftLift, rightLift = 1, 1
	case pet.Sleepy, pet.Starving:
		leftLift, rightLift = -1, -1
	}

	armRow := eyeTopRow + 1
	left, right := cupEdges(armRow)
	c.rect(left-2, armRow-leftLift, 2, 2, paper.body)
	c.rect(right+1, armRow-rightLift, 2, 2, paper.dark)
}

func drawMelting(c *canvas, pose Pose) {
	if pose.Mood != pet.Anxious {
		return
	}

	melted := iceCreamShades(pose.Stage)
	left, right := cupEdges(rimRow)
	c.rect(left-3, rimRow-1, right-left+7, 1, melted.body)

	for index, dripX := range []int{left - 3, right + 3} {
		dripLength := 1 + (pose.Tick+index)%4
		c.rect(dripX, rimRow, 1, dripLength, melted.body)
		c.set(dripX, rimRow+dripLength, melted.dark)
	}
}

func addOutline(layer *image.RGBA) {
	bounds := layer.Bounds()

	var edgePixels []image.Point
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if layer.RGBAAt(x, y).A != 0 {
				continue
			}

			for _, neighbor := range []image.Point{{x - 1, y}, {x + 1, y}, {x, y - 1}, {x, y + 1}} {
				if neighbor.In(bounds) && layer.RGBAAt(neighbor.X, neighbor.Y).A != 0 {
					edgePixels = append(edgePixels, image.Pt(x, y))
					break
				}
			}
		}
	}

	for _, edge := range edgePixels {
		layer.SetRGBA(edge.X, edge.Y, outline)
	}
}

func isBlinking(pose Pose) bool {
	return pose.Tick%blinkPeriodTicks == blinkPeriodTicks-1
}

func drawEyes(c *canvas, pose Pose) {
	leftEyeX, rightEyeX := centerX-4, centerX+2

	switch {
	case pose.Mood == pet.Excited || pose.Mood == pet.Eating:
		c.points(ink, [2]int{leftEyeX, eyeTopRow}, [2]int{leftEyeX + 1, eyeTopRow + 1}, [2]int{leftEyeX, eyeTopRow + 2})
		c.points(ink, [2]int{rightEyeX + 1, eyeTopRow}, [2]int{rightEyeX, eyeTopRow + 1}, [2]int{rightEyeX + 1, eyeTopRow + 2})
	case pose.Mood == pet.Sleepy || isBlinking(pose):
		c.rect(leftEyeX, eyeTopRow+2, 2, 1, ink)
		c.rect(rightEyeX, eyeTopRow+2, 2, 1, ink)
	case pose.Mood == pet.Starving:
		for _, eyeMiddleX := range []int{leftEyeX, rightEyeX + 1} {
			c.points(ink,
				[2]int{eyeMiddleX - 1, eyeTopRow}, [2]int{eyeMiddleX + 1, eyeTopRow},
				[2]int{eyeMiddleX, eyeTopRow + 1},
				[2]int{eyeMiddleX - 1, eyeTopRow + 2}, [2]int{eyeMiddleX + 1, eyeTopRow + 2})
		}
	case pose.Mood == pet.Hungry:
		c.rect(leftEyeX, eyeTopRow+1, 2, 2, ink)
		c.rect(rightEyeX, eyeTopRow+1, 2, 2, ink)
		c.set(leftEyeX, eyeTopRow+1, white)
		c.set(rightEyeX, eyeTopRow+1, white)
	default:
		c.rect(leftEyeX, eyeTopRow, 2, 3, ink)
		c.rect(rightEyeX, eyeTopRow, 2, 3, ink)
		c.set(leftEyeX, eyeTopRow, white)
		c.set(rightEyeX, eyeTopRow, white)
	}
}

func drawCheeks(c *canvas, pose Pose) {
	if pose.Mood == pet.Starving || pose.Mood == pet.Anxious {
		return
	}

	c.rect(centerX-6, eyeTopRow+3, 2, 1, cheek)
	c.rect(centerX+4, eyeTopRow+3, 2, 1, cheek)
}

func drawMouth(c *canvas, pose Pose) {
	switch pose.Mood {
	case pet.Happy:
		c.points(ink, [2]int{centerX - 2, mouthRow - 1}, [2]int{centerX - 1, mouthRow}, [2]int{centerX, mouthRow}, [2]int{centerX + 1, mouthRow - 1})
	case pet.Excited:
		c.rect(centerX-1, mouthRow-1, 2, 1, ink)
		c.rect(centerX-1, mouthRow, 2, 1, tongue)
	case pet.Eating:
		if pose.Tick%2 == 0 {
			c.rect(centerX-1, mouthRow-1, 2, 2, ink)
		} else {
			c.rect(centerX-2, mouthRow, 4, 1, ink)
		}
	case pet.Hungry:
		c.rect(centerX-1, mouthRow-1, 2, 2, ink)
	case pet.Starving:
		c.points(ink, [2]int{centerX - 2, mouthRow}, [2]int{centerX - 1, mouthRow - 1}, [2]int{centerX, mouthRow - 1}, [2]int{centerX + 1, mouthRow})
	case pet.Anxious:
		c.points(ink,
			[2]int{centerX - 3, mouthRow}, [2]int{centerX - 2, mouthRow - 1}, [2]int{centerX - 1, mouthRow},
			[2]int{centerX, mouthRow - 1}, [2]int{centerX + 1, mouthRow}, [2]int{centerX + 2, mouthRow - 1})
	}
}

func drawCrown(c *canvas, iceCreamTop int) {
	left, top := centerX-3, iceCreamTop-3

	c.points(crownGold, [2]int{left, top}, [2]int{left + 2, top}, [2]int{left + 4, top})
	c.rect(left, top+1, 5, 2, crownGold)
	c.set(left+2, top+1, crownJewel)
}

func drawEffects(body, background *canvas, iceCreamTop int, pose Pose) {
	_, rimRight := cupEdges(rimRow)

	switch pose.Mood {
	case pet.Anxious:
		dropX, dropY := rimRight+5, rimRow-5+pose.Tick%3
		body.points(sweat, [2]int{dropX, dropY}, [2]int{dropX, dropY + 2})
		body.rect(dropX-1, dropY+1, 3, 1, sweat)
	case pet.Sleepy:
		if pose.Tick/3%2 == 0 {
			drawZ(background, rimRight, iceCreamTop-4)
		} else {
			drawZ(background, rimRight+3, max(iceCreamTop-9, 0))
		}
	case pet.Hungry, pet.Starving:
		if pose.Tick%4 < 2 {
			sideLeft, sideRight := cupEdges(mouthRow)
			body.points(paper.dark, [2]int{sideLeft - 6, mouthRow}, [2]int{sideLeft - 5, mouthRow - 1}, [2]int{sideLeft - 4, mouthRow})
			body.points(paper.dark, [2]int{sideRight + 4, mouthRow}, [2]int{sideRight + 5, mouthRow - 1}, [2]int{sideRight + 6, mouthRow})
		}
	case pet.Eating:
		crumbFall := pose.Tick % 3
		sideLeft, sideRight := cupEdges(mouthRow)
		body.points(crumb,
			[2]int{sideLeft - 3, mouthRow - 1 + crumbFall},
			[2]int{sideRight + 3, mouthRow - 2 + crumbFall},
			[2]int{sideRight + 5, mouthRow + crumbFall})
	case pet.Excited:
		drawSparkles(background, pose.Tick)
	}

	if pose.Stage == pet.DeluxeFlurry && pose.Mood != pet.Excited {
		drawSparkles(background, pose.Tick+1)
	}
}

func drawZ(c *canvas, left, top int) {
	c.rect(left, top, 4, 1, sleepZ)
	c.set(left+2, top+1, sleepZ)
	c.set(left+1, top+2, sleepZ)
	c.rect(left, top+3, 4, 1, sleepZ)
}

func drawSparkles(c *canvas, tick int) {
	sparkleSpots := [][2]int{{4, 14}, {28, 18}, {5, 30}, {27, 8}}
	for index, spot := range sparkleSpots {
		if (tick+index)%2 == 0 {
			continue
		}

		x, y := spot[0], spot[1]
		c.points(sparkle, [2]int{x, y - 1}, [2]int{x - 1, y}, [2]int{x, y}, [2]int{x + 1, y}, [2]int{x, y + 1})
	}
}

func mirror(source *image.RGBA) *image.RGBA {
	bounds := source.Bounds()
	mirrored := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			mirroredX := bounds.Max.X - 1 - (x - bounds.Min.X)
			mirrored.SetRGBA(mirroredX, y, source.RGBAAt(x, y))
		}
	}

	return mirrored
}

func Scale(source *image.RGBA, factor int) *image.RGBA {
	bounds := source.Bounds()
	scaled := image.NewRGBA(image.Rect(0, 0, bounds.Dx()*factor, bounds.Dy()*factor))
	for y := 0; y < scaled.Bounds().Dy(); y++ {
		for x := 0; x < scaled.Bounds().Dx(); x++ {
			scaled.SetRGBA(x, y, source.RGBAAt(bounds.Min.X+x/factor, bounds.Min.Y+y/factor))
		}
	}

	return scaled
}

func Swatches() []color.RGBA {
	swatches := []color.RGBA{cupInside, crumb, strawberry, outline, ink, white, cheek, tongue, sweat, sparkle, crownGold, crownJewel, sleepZ}
	for _, group := range []shades{paper, cream, berryCream, blueWave, goldTrim} {
		swatches = append(swatches, group.light, group.body, group.dark)
	}

	return swatches
}

func VisibleBounds(frame *image.RGBA) image.Rectangle {
	bounds := frame.Bounds()

	visible := image.Rectangle{}
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if frame.RGBAAt(x, y).A != 0 {
				visible = visible.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}

	return visible
}

func VisibleTop(frame *image.RGBA) int {
	visible := VisibleBounds(frame)
	if visible.Empty() {
		return 0
	}

	return visible.Min.Y - frame.Bounds().Min.Y
}

func Icon(size int) *image.RGBA {
	frame := Draw(Pose{Mood: pet.Happy, Stage: pet.ClassicFlurry})
	visible := VisibleBounds(frame)
	scaleFactor := max(1, size/max(visible.Dx(), visible.Dy()))
	scaled := Scale(frame.SubImage(visible).(*image.RGBA), scaleFactor)

	icon := image.NewRGBA(image.Rect(0, 0, size, size))
	centeredOffset := image.Pt((size-scaled.Bounds().Dx())/2, (size-scaled.Bounds().Dy())/2)
	draw.Draw(icon, scaled.Bounds().Add(centeredOffset), scaled, image.Point{}, draw.Src)

	return icon
}
