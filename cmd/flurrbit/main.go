package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"flurrbit/internal/account"
	"flurrbit/internal/desktop"
	"flurrbit/internal/mcp"
	"flurrbit/internal/pet"
	"flurrbit/internal/render"
	"flurrbit/internal/sprite"
)

const usage = `旋宝 Flurrbit　桌面上的像素麦旋风，它的喜怒哀乐都来自你的麦当劳账户

用法
  flurrbit [选项]          把旋宝放到桌面上
  flurrbit status [选项]   在终端里看看它现在怎么样
  flurrbit gif [选项]      把它此刻的样子存成动图
  flurrbit doctor          检查 Token 和网络
  flurrbit version         查看版本

通用选项
  --demo <场景>     用演示数据，可选 ` + "happy、excited、eating、hungry、starving、anxious、sleepy、deluxe" + `
  --font <文件>     换一款中文字体，默认用内置的像素字体，也可以设置 FLURRBIT_FONT
  --timeout <时长>  每次请求最多等多久，默认 15s

桌面选项
  --refresh <时长>  多久同步一次账户，默认 10m
  --walk            让它在屏幕底下到处溜达，默认不动，运行时按 W 切换

动图选项
  -o <文件>         保存到哪里，默认 flurrbit.gif
  --frames <数量>   一共几帧，默认 16
  --line <序号>     气泡里放第几句话，从 0 数起

环境变量
  MCD_MCP_TOKEN     麦当劳 MCP Token，到 https://open.mcd.cn/mcp 申请
  MCD_MCP_URL       MCP 地址，默认 https://mcp.mcd.cn

旋宝只读取账户信息，不会下单，不会领券，也不会花你的积分。
`

const (
	defaultTimeout       = 15 * time.Second
	defaultGIFPath       = "flurrbit.gif"
	defaultGIFFrames     = 16
	gifFrameDelay        = 15
	statusColumnGap      = 3
	statusSeparatorWidth = 22
	meterSegments        = 10
)

const missingTokenMessage = `还没设置 MCD_MCP_TOKEN。到 https://open.mcd.cn/mcp 激活 Token，` +
	`再在 PowerShell 里运行 $env:MCD_MCP_TOKEN = "你的 Token"；想先看看效果，可以加 --demo happy`

type commonFlags struct {
	demoScenario string
	fontPath     string
	timeout      time.Duration
}

func (c *commonFlags) register(flags *flag.FlagSet) {
	flags.StringVar(&c.demoScenario, "demo", "", "")
	flags.StringVar(&c.fontPath, "font", "", "")
	flags.DurationVar(&c.timeout, "timeout", defaultTimeout, "")
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "flurrbit:", err)
		os.Exit(1)
	}
}

func run(arguments []string) error {
	command := "desktop"
	if len(arguments) > 0 && !strings.HasPrefix(arguments[0], "-") {
		command, arguments = arguments[0], arguments[1:]
	}

	switch command {
	case "desktop", "run":
		return runDesktop(arguments)
	case "status":
		return runStatus(arguments)
	case "gif":
		return runGIF(arguments)
	case "doctor":
		return runDoctor()
	case "version":
		fmt.Println("flurrbit", mcp.Version)
		return nil
	case "help":
		fmt.Print(usage)
		return nil
	}

	return fmt.Errorf("没有 %q 这个命令，运行 flurrbit help 看看有哪些", command)
}

func newFlagSet(name string) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.Usage = func() { fmt.Fprint(os.Stderr, usage) }

	return flags
}

func runDesktop(arguments []string) error {
	var common commonFlags
	var refreshEvery time.Duration
	var walk bool

	flags := newFlagSet("desktop")
	common.register(flags)
	flags.DurationVar(&refreshEvery, "refresh", desktop.DefaultRefreshInterval, "")
	flags.BoolVar(&walk, "walk", false, "")
	if err := flags.Parse(arguments); err != nil {
		return err
	}

	loadSnapshot, err := newLoader(common)
	if err != nil {
		return err
	}

	face, err := render.LoadFace(common.fontPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "flurrbit: %v，它说的话会打印在终端里\n", err)
	}

	return desktop.Run(desktop.Options{Load: loadSnapshot, RefreshEvery: refreshEvery, Face: face, Walk: walk})
}

func runStatus(arguments []string) error {
	var common commonFlags

	flags := newFlagSet("status")
	common.register(flags)
	if err := flags.Parse(arguments); err != nil {
		return err
	}

	loadSnapshot, err := newLoader(common)
	if err != nil {
		return err
	}

	snapshot := loadSnapshot(context.Background())
	if snapshot.Unauthorized() {
		return mcp.ErrUnauthorized
	}

	state := pet.Evaluate(snapshot)
	if !stdoutSupportsColor() {
		fmt.Print(strings.Join(statusLines(state, snapshot, plainText), "\n") + "\n")
		return nil
	}

	petArt := render.TerminalArt(sprite.Draw(sprite.Pose{Mood: state.Mood, Stage: state.Stage}))
	textBesideArt := append([]string{""}, statusLines(state, snapshot, highlighted)...)
	fmt.Print(render.SideBySide(petArt, textBesideArt, statusColumnGap))

	return nil
}

func stdoutSupportsColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	stdoutInfo, err := os.Stdout.Stat()
	isTerminal := err == nil && stdoutInfo.Mode()&os.ModeCharDevice != 0
	if !isTerminal {
		return false
	}

	return enableVirtualTerminal()
}

func highlighted(text string) string {
	return "\x1b[1;38;2;255;199;44m" + text + "\x1b[0m"
}

func plainText(text string) string {
	return text
}

func statusLines(state pet.State, snapshot account.Snapshot, highlight func(string) string) []string {
	lines := []string{
		highlight(fmt.Sprintf("旋宝 · %s · %s", state.Stage.Label(), state.Mood.Label())),
		strings.Repeat("─", statusSeparatorWidth),
		fmt.Sprintf("%s  %s %3d", highlight("饱腹"), meter(state.Satiety), state.Satiety),
		fmt.Sprintf("%s  %s %3d", highlight("淡定"), meter(state.Calmness), state.Calmness),
	}
	if snapshot.Points != nil {
		lines = append(lines, fmt.Sprintf("%s  %s", highlight("积分"), pet.FormatNumber(snapshot.Points.Available)))
	}

	lines = append(lines, "")
	for _, line := range state.Lines {
		lines = append(lines, "· "+line)
	}

	return lines
}

func meter(percent int) string {
	filledSegments := (percent + meterSegments/2) / meterSegments

	return strings.Repeat("█", filledSegments) + strings.Repeat("░", meterSegments-filledSegments)
}

func runGIF(arguments []string) error {
	var common commonFlags
	var outputPath string
	var frameCount, lineIndex int

	flags := newFlagSet("gif")
	common.register(flags)
	flags.StringVar(&outputPath, "o", defaultGIFPath, "")
	flags.IntVar(&frameCount, "frames", defaultGIFFrames, "")
	flags.IntVar(&lineIndex, "line", 0, "")
	if err := flags.Parse(arguments); err != nil {
		return err
	}

	loadSnapshot, err := newLoader(common)
	if err != nil {
		return err
	}
	state := pet.Evaluate(loadSnapshot(context.Background()))

	face, err := render.LoadFace(common.fontPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "flurrbit: %v，这次的动图先不带气泡\n", err)
	}

	scene := render.Scene{State: state, Face: face}
	if lineIndex >= 0 && lineIndex < len(state.Lines) {
		scene.Line = state.Lines[lineIndex]
	}

	outputFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	if err := render.WriteGIF(outputFile, scene, frameCount, gifFrameDelay); err != nil {
		outputFile.Close()
		return err
	}
	if err := outputFile.Close(); err != nil {
		return err
	}

	fmt.Printf("存好了：%s，%s，%s\n", outputPath, state.Stage.Label(), state.Mood.Label())

	return nil
}

func runDoctor() error {
	url, token := mcpEndpoint()
	fmt.Println("flurrbit", mcp.Version, "doctor")
	fmt.Println("  MCP 地址   ", url)

	if token == "" {
		fmt.Println("  Token       还没设置 MCD_MCP_TOKEN")
		return errors.New("缺少 Token")
	}
	fmt.Printf("  Token       已设置（%d 位）\n", len(token))

	ctx := context.Background()
	client := mcp.NewClient(url, token, defaultTimeout)

	serverInfo, err := client.Initialize(ctx)
	if err != nil {
		fmt.Println("  握手        没成功：", err)
		return err
	}
	fmt.Printf("  握手        成功，服务端 %s %s\n", serverInfo.Name, serverInfo.Version)

	toolNames, err := client.ListTools(ctx)
	if err != nil {
		fmt.Println("  tools/list  没成功：", err)
		return err
	}

	availableTools := map[string]bool{}
	for _, name := range toolNames {
		availableTools[name] = true
	}

	fmt.Printf("  Tool 数量   %d\n", len(toolNames))
	for _, tool := range mcp.ReadOnlyTools {
		toolStatus := "缺失"
		if availableTools[tool] {
			toolStatus = " ok "
		}
		fmt.Printf("    [%s] %s\n", toolStatus, tool)
	}

	return nil
}

func mcpEndpoint() (string, string) {
	url := strings.TrimSpace(os.Getenv("MCD_MCP_URL"))
	if url == "" {
		url = mcp.DefaultURL
	}
	token := strings.TrimSpace(os.Getenv("MCD_MCP_TOKEN"))

	return url, token
}

func newLoader(common commonFlags) (account.Loader, error) {
	if common.demoScenario != "" {
		return newDemoLoader(common.demoScenario)
	}

	url, token := mcpEndpoint()
	if token == "" {
		return nil, errors.New(missingTokenMessage)
	}

	loadFromMCP := func(ctx context.Context) account.Snapshot {
		client := mcp.NewClient(url, token, common.timeout)
		if _, err := client.Initialize(ctx); err != nil {
			return account.Snapshot{
				Now:      time.Now().In(account.Beijing),
				Problems: []account.Problem{{Tool: account.InitializeStep, Err: err}},
			}
		}

		return account.Fetch(ctx, client, time.Now())
	}

	return loadFromMCP, nil
}

func newDemoLoader(scenario string) (account.Loader, error) {
	demo, err := account.NewDemo(scenario, time.Now())
	if err != nil {
		return nil, err
	}

	loadDemo := func(ctx context.Context) account.Snapshot {
		return account.Fetch(ctx, demo, demo.Now())
	}

	return loadDemo, nil
}
