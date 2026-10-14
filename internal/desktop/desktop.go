package desktop

import (
	"context"
	"errors"
	"fmt"
	"image"
	"math/rand/v2"
	"strings"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"golang.org/x/image/font"

	"flurrbit/internal/account"
	"flurrbit/internal/pet"
	"flurrbit/internal/render"
	"flurrbit/internal/sprite"
)

const (
	updatesPerSecond         = 60
	updatesPerAnimationFrame = 10
	sleepyAnimationSlowdown  = 3
	DefaultRefreshInterval   = 10 * time.Minute
)

const (
	windowWidth      = 300
	bubbleAreaHeight = 120
	bubbleMaxWidth   = 180
	screenEdgeMargin = 24
	taskbarHeight    = 48
)

const (
	bubbleDuration  = 6 * time.Second
	chatterInterval = 45 * time.Second
	dragThreshold   = 4
)

const (
	walkStepPixels         = 1
	excitedWalkSpeedFactor = 2
	updatesPerWalkStep     = 2
	minimumRestUpdates     = 3 * updatesPerSecond
	extraRestRangeFactor   = 3
)

var windowIconSizes = []int{16, 32, 48}

type Options struct {
	Load         account.Loader
	RefreshEvery time.Duration
	Face         font.Face
	Walk         bool
}

type moodAndStage struct {
	mood  pet.Mood
	stage pet.Stage
}

type petWindow struct {
	options Options

	stateMutex sync.Mutex
	state      pet.State
	stateReady bool

	updateCount              int
	spriteImages             map[sprite.Pose]*ebiten.Image
	bubbleImages             map[string]*ebiten.Image
	restingTopByMoodAndStage map[moodAndStage]int

	hasGreeted       bool
	lastMood         pet.Mood
	currentLineIndex int
	bubbleText       string
	bubbleHideAt     time.Time
	nextChatterAt    time.Time
	showingStatus    bool
	placedOnScreen   bool

	pressedOnPet        bool
	dragging            bool
	pressCursorOnScreen image.Point
	pressWindowOrigin   image.Point

	walkEnabled     bool
	walkTargetX     int
	facingLeft      bool
	restUpdatesLeft int
}

func Run(options Options) error {
	if options.RefreshEvery <= 0 {
		options.RefreshEvery = DefaultRefreshInterval
	}

	window := &petWindow{
		options:                  options,
		spriteImages:             map[sprite.Pose]*ebiten.Image{},
		bubbleImages:             map[string]*ebiten.Image{},
		restingTopByMoodAndStage: map[moodAndStage]int{},
		nextChatterAt:            time.Now().Add(chatterInterval),
		walkEnabled:              options.Walk,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go window.refreshLoop(ctx)

	var windowIcons []image.Image
	for _, size := range windowIconSizes {
		windowIcons = append(windowIcons, sprite.Icon(size))
	}

	ebiten.SetWindowTitle("旋宝 Flurrbit")
	ebiten.SetWindowIcon(windowIcons)
	ebiten.SetWindowDecorated(false)
	ebiten.SetWindowFloating(true)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeDisabled)
	ebiten.SetWindowSize(windowWidth, windowHeight())
	ebiten.SetTPS(updatesPerSecond)

	err := ebiten.RunGameWithOptions(window, &ebiten.RunGameOptions{ScreenTransparent: true})
	if errors.Is(err, ebiten.Termination) {
		return nil
	}

	return err
}

func windowHeight() int {
	return bubbleAreaHeight + sprite.Height*sprite.DisplayScale
}

func (w *petWindow) refreshLoop(ctx context.Context) {
	ticker := time.NewTicker(w.options.RefreshEvery)
	defer ticker.Stop()

	for {
		w.refresh(ctx)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *petWindow) refresh(ctx context.Context) {
	snapshot := w.options.Load(ctx)
	state := pet.Evaluate(snapshot)
	if line, ok := connectionLine(snapshot); ok {
		state.Lines = append([]string{line}, state.Lines...)
	}

	w.stateMutex.Lock()
	defer w.stateMutex.Unlock()

	w.state = state
	w.stateReady = true
}

func connectionLine(snapshot account.Snapshot) (string, bool) {
	switch {
	case snapshot.Unauthorized():
		return "Token 过期了，去 open.mcd.cn 重新激活一下吧", true
	case snapshot.Offline():
		return "暂时连不上麦当劳，我先自己待会儿", true
	}

	return "", false
}

func (w *petWindow) currentState() (pet.State, bool) {
	w.stateMutex.Lock()
	defer w.stateMutex.Unlock()

	return w.state, w.stateReady
}

func (w *petWindow) Update() error {
	w.updateCount++

	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) || inpututil.IsKeyJustPressed(ebiten.KeyQ) {
		return ebiten.Termination
	}

	state, ready := w.currentState()
	if !ready {
		return nil
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyW) {
		w.toggleWalk(state.Mood)
	}
	if !w.placedOnScreen {
		w.placeAtScreenCorner()
	}

	now := time.Now()
	if !w.hasGreeted || state.Mood != w.lastMood {
		w.hasGreeted = true
		w.lastMood = state.Mood
		w.currentLineIndex = 0
		w.say(state.Lines[0], now)
	}
	if now.After(w.nextChatterAt) {
		w.sayNextLine(state, now)
	}

	w.handleMouse(state, now)
	if !w.dragging {
		w.wander(state)
	}

	return nil
}

func (w *petWindow) say(text string, now time.Time) {
	w.showingStatus = false
	w.bubbleText = text
	w.bubbleHideAt = now.Add(bubbleDuration)
	w.nextChatterAt = now.Add(chatterInterval)

	fmt.Println("旋宝：" + text)
}

func (w *petWindow) sayNextLine(state pet.State, now time.Time) {
	w.currentLineIndex = (w.currentLineIndex + 1) % len(state.Lines)
	w.say(state.Lines[w.currentLineIndex], now)
}

func (w *petWindow) placeAtScreenCorner() {
	screenWidth, screenHeight := ebiten.Monitor().Size()
	if screenWidth == 0 {
		return
	}

	left := screenWidth - windowWidth - screenEdgeMargin
	top := screenHeight - windowHeight() - taskbarHeight
	ebiten.SetWindowPosition(left, top)

	w.walkTargetX = left
	w.placedOnScreen = true
}

func (w *petWindow) handleMouse(state pet.State, now time.Time) {
	windowOrigin := image.Pt(ebiten.WindowPosition())
	cursorInWindow := image.Pt(ebiten.CursorPosition())
	cursorOnScreen := windowOrigin.Add(cursorInWindow)

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && cursorInWindow.In(petArea()) {
		w.pressedOnPet = true
		w.pressCursorOnScreen = cursorOnScreen
		w.pressWindowOrigin = windowOrigin
	}

	if w.pressedOnPet && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		dragOffset := cursorOnScreen.Sub(w.pressCursorOnScreen)
		if abs(dragOffset.X) > dragThreshold || abs(dragOffset.Y) > dragThreshold {
			w.dragging = true
		}
		if w.dragging {
			destination := w.pressWindowOrigin.Add(dragOffset)
			ebiten.SetWindowPosition(destination.X, destination.Y)
		}
	}

	if w.pressedOnPet && inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
		if w.dragging {
			w.walkTargetX, _ = ebiten.WindowPosition()
		} else {
			w.sayNextLine(state, now)
		}
		w.pressedOnPet, w.dragging = false, false
	}

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		w.showingStatus = !w.showingStatus
		w.bubbleHideAt = now.Add(bubbleDuration)
	}
}

func petArea() image.Rectangle {
	width, height := sprite.Width*sprite.DisplayScale, sprite.Height*sprite.DisplayScale
	left := (windowWidth - width) / 2
	top := windowHeight() - height

	return image.Rect(left, top, left+width, top+height)
}

func (w *petWindow) canWalk(mood pet.Mood) bool {
	return w.walkEnabled && mood != pet.Sleepy && mood != pet.Eating
}

func (w *petWindow) toggleWalk(mood pet.Mood) {
	w.walkEnabled = !w.walkEnabled
	w.walkTargetX, _ = ebiten.WindowPosition()

	switch {
	case !w.walkEnabled:
		w.say("好，我就待在这儿不动", time.Now())
	case mood == pet.Sleepy:
		w.say("我在睡觉呢，醒了再去溜达", time.Now())
	case mood == pet.Eating:
		w.say("等我吃完这口再去溜达", time.Now())
	default:
		w.say("好耶，我可以出去溜达了", time.Now())
	}
}

func (w *petWindow) wander(state pet.State) {
	if !w.canWalk(state.Mood) {
		return
	}

	windowX, windowY := ebiten.WindowPosition()
	if windowX == w.walkTargetX {
		w.chooseNextWalkTarget()
		return
	}

	if w.updateCount%updatesPerWalkStep != 0 {
		return
	}

	stepPixels := walkStepPixels
	if state.Mood == pet.Excited {
		stepPixels *= excitedWalkSpeedFactor
	}

	w.facingLeft = w.walkTargetX < windowX
	if w.facingLeft {
		ebiten.SetWindowPosition(max(w.walkTargetX, windowX-stepPixels), windowY)
	} else {
		ebiten.SetWindowPosition(min(w.walkTargetX, windowX+stepPixels), windowY)
	}
}

func (w *petWindow) chooseNextWalkTarget() {
	if w.restUpdatesLeft > 0 {
		w.restUpdatesLeft--
		return
	}

	screenWidth, _ := ebiten.Monitor().Size()
	if screenWidth <= windowWidth {
		return
	}

	w.walkTargetX = rand.IntN(screenWidth - windowWidth)
	w.restUpdatesLeft = minimumRestUpdates + rand.IntN(extraRestRangeFactor*minimumRestUpdates)
}

func (w *petWindow) isWalking(state pet.State) bool {
	windowX, _ := ebiten.WindowPosition()

	return w.canWalk(state.Mood) && !w.dragging && windowX != w.walkTargetX
}

func (w *petWindow) Draw(screen *ebiten.Image) {
	state, ready := w.currentState()
	if !ready {
		return
	}

	animationFrame := w.updateCount / updatesPerAnimationFrame
	if state.Mood == pet.Sleepy {
		animationFrame /= sleepyAnimationSlowdown
	}
	animationFrame %= sprite.AnimationCycle

	pose := sprite.Pose{
		Mood:       state.Mood,
		Stage:      state.Stage,
		Tick:       animationFrame,
		Walking:    w.isWalking(state),
		FacingLeft: w.facingLeft,
	}
	area := petArea()
	drawOptions := &ebiten.DrawImageOptions{}
	drawOptions.GeoM.Scale(sprite.DisplayScale, sprite.DisplayScale)
	drawOptions.GeoM.Translate(float64(area.Min.X), float64(area.Min.Y))
	screen.DrawImage(w.spriteImage(pose), drawOptions)

	if time.Now().Before(w.bubbleHideAt) {
		bubbleText := w.bubbleText
		if w.showingStatus {
			bubbleText = statusText(state)
		}
		bubbleBottom := area.Min.Y + w.restingTop(state)*sprite.DisplayScale
		w.drawBubble(screen, bubbleText, bubbleBottom)
	}
}

func (w *petWindow) spriteImage(pose sprite.Pose) *ebiten.Image {
	if cachedImage, ok := w.spriteImages[pose]; ok {
		return cachedImage
	}

	spriteImage := ebiten.NewImageFromImage(sprite.Draw(pose))
	w.spriteImages[pose] = spriteImage

	return spriteImage
}

func (w *petWindow) restingTop(state pet.State) int {
	key := moodAndStage{mood: state.Mood, stage: state.Stage}
	if cachedTop, ok := w.restingTopByMoodAndStage[key]; ok {
		return cachedTop
	}

	restingTop := sprite.VisibleTop(sprite.Draw(sprite.Pose{Mood: state.Mood, Stage: state.Stage}))
	w.restingTopByMoodAndStage[key] = restingTop

	return restingTop
}

func (w *petWindow) drawBubble(screen *ebiten.Image, text string, bottom int) {
	if w.options.Face == nil || text == "" {
		return
	}

	bubble, ok := w.bubbleImages[text]
	if !ok {
		bubble = ebiten.NewImageFromImage(render.Bubble(w.options.Face, text, bubbleMaxWidth))
		w.bubbleImages[text] = bubble
	}

	width, height := bubble.Bounds().Dx(), bubble.Bounds().Dy()
	drawOptions := &ebiten.DrawImageOptions{}
	drawOptions.GeoM.Translate(float64((windowWidth-width)/2), float64(max(0, bottom-height)))
	screen.DrawImage(bubble, drawOptions)
}

func statusText(state pet.State) string {
	return strings.Join([]string{
		"心情　" + state.Mood.Label(),
		fmt.Sprintf("饱腹　%d", state.Satiety),
		fmt.Sprintf("淡定　%d", state.Calmness),
		"阶段　" + state.Stage.Label(),
	}, "\n")
}

func (w *petWindow) Layout(int, int) (int, int) {
	return windowWidth, windowHeight()
}

func abs(value int) int {
	if value < 0 {
		return -value
	}

	return value
}
