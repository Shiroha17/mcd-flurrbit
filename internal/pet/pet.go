package pet

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"flurrbit/internal/account"
)

type Mood int

const (
	Happy Mood = iota
	Excited
	Eating
	Hungry
	Starving
	Anxious
	Sleepy
)

func (m Mood) String() string {
	return [...]string{"happy", "excited", "eating", "hungry", "starving", "anxious", "sleepy"}[m]
}

func (m Mood) Label() string {
	return [...]string{"开心", "兴奋", "开饭中", "有点饿", "饿坏了", "着急", "犯困"}[m]
}

type Stage int

const (
	MiniFlurry Stage = iota
	ClassicFlurry
	StrawberryFlurry
	DeluxeFlurry
)

var stagePointThresholds = []float64{0, 2000, 8000, 24000}

func (s Stage) Label() string {
	return [...]string{"迷你麦旋风", "经典麦旋风", "草莓麦旋风", "至尊麦旋风"}[s]
}

func stageForPoints(points float64) Stage {
	stage := MiniFlurry
	for index, threshold := range stagePointThresholds {
		if points >= threshold {
			stage = Stage(index)
		}
	}

	return stage
}

const (
	eatingDuration       = 90 * time.Minute
	satietyDuration      = 14 * 24 * time.Hour
	urgentCouponWindow   = 24 * time.Hour
	expiringCouponWindow = 48 * time.Hour
	monthEndWorryDays    = 7
	nightEndsAtHour      = 7
	unknownSatiety       = 70
	starvingSatiety      = 20
	hungrySatiety        = 50
)

const (
	calmnessLostPerUrgentCoupon   = 35
	calmnessLostPerExpiringCoupon = 15
	calmnessLostForExpiringPoints = 25
)

const (
	maxCouponLines              = 2
	maxCampaignLines            = 2
	programmersDayCountdownDays = 10
)

type State struct {
	Mood     Mood
	Stage    Stage
	Satiety  int
	Calmness int
	Lines    []string
}

type accountSignals struct {
	now              time.Time
	lastOrder        account.Order
	hasLastOrder     bool
	timeSinceMeal    time.Duration
	urgentCoupons    []account.Coupon
	expiringCoupons  []account.Coupon
	expiringPoints   float64
	unclaimedCoupons int
	campaignsToday   []string
}

func Evaluate(snapshot account.Snapshot) State {
	signals := readSignals(snapshot)

	state := State{
		Stage:    stageForPoints(growthPoints(snapshot.Points)),
		Satiety:  signals.satiety(),
		Calmness: signals.calmness(),
	}
	state.Mood = signals.mood(state.Satiety)
	state.Lines = composeLines(signals, state, snapshot)

	return state
}

func readSignals(snapshot account.Snapshot) accountSignals {
	signals := accountSignals{now: snapshot.Now, campaignsToday: snapshot.CampaignsToday}

	if snapshot.History != nil {
		signals.lastOrder, signals.hasLastOrder = snapshot.History.Latest()
	}
	if signals.hasLastOrder {
		signals.timeSinceMeal = signals.now.Sub(signals.lastOrder.PlacedAt)
	}
	if snapshot.Wallet != nil {
		signals.urgentCoupons = snapshot.Wallet.ExpiringWithin(signals.now, urgentCouponWindow)
		signals.expiringCoupons = snapshot.Wallet.ExpiringWithin(signals.now, expiringCouponWindow)
	}
	if snapshot.Points != nil && daysLeftInMonth(signals.now) <= monthEndWorryDays {
		signals.expiringPoints = snapshot.Points.ExpiringThisMonth
	}
	if snapshot.Claimable != nil {
		signals.unclaimedCoupons = snapshot.Claimable.Unclaimed
	}

	return signals
}

func growthPoints(points *account.Points) float64 {
	switch {
	case points == nil:
		return 0
	case points.Accumulated != 0:
		return points.Accumulated
	}

	return points.Available
}

func (s accountSignals) justAte() bool {
	return s.hasLastOrder && s.timeSinceMeal >= 0 && s.timeSinceMeal < eatingDuration
}

func (s accountSignals) satiety() int {
	if !s.hasLastOrder {
		return unknownSatiety
	}

	return satietyAfter(s.timeSinceMeal)
}

func (s accountSignals) calmness() int {
	expiringButNotUrgent := len(s.expiringCoupons) - len(s.urgentCoupons)

	calmness := 100
	calmness -= calmnessLostPerUrgentCoupon * len(s.urgentCoupons)
	calmness -= calmnessLostPerExpiringCoupon * expiringButNotUrgent
	if s.expiringPoints > 0 {
		calmness -= calmnessLostForExpiringPoints
	}

	return clamp(calmness, 0, 100)
}

func (s accountSignals) mood(satiety int) Mood {
	switch {
	case len(s.urgentCoupons) > 0:
		return Anxious
	case s.justAte():
		return Eating
	case s.now.Hour() < nightEndsAtHour:
		return Sleepy
	case satiety < starvingSatiety:
		return Starving
	case len(s.expiringCoupons) > 0 || s.expiringPoints > 0:
		return Anxious
	case satiety < hungrySatiety:
		return Hungry
	case s.unclaimedCoupons > 0 || len(s.campaignsToday) > 0:
		return Excited
	}

	return Happy
}

func composeLines(signals accountSignals, state State, snapshot account.Snapshot) []string {
	var lines []string

	for _, coupon := range firstN(signals.expiringCoupons, maxCouponLines) {
		lines = append(lines, couponLine(signals.now, coupon))
	}
	if signals.expiringPoints > 0 {
		lines = append(lines, fmt.Sprintf("月底有 %s 积分要过期，挑点东西换掉吧", FormatNumber(signals.expiringPoints)))
	}
	if line, ok := mealLine(signals, state.Satiety); ok {
		lines = append(lines, line)
	}
	if signals.unclaimedCoupons > 0 {
		lines = append(lines, fmt.Sprintf("麦麦省有 %d 张券还没领，去领一下吧", signals.unclaimedCoupons))
	}
	for _, title := range firstN(signals.campaignsToday, maxCampaignLines) {
		lines = append(lines, fmt.Sprintf("今天的活动是「%s」", title))
	}
	if line, ok := growthLine(state.Stage, snapshot.Points != nil); ok {
		lines = append(lines, line)
	}
	if snapshot.History != nil {
		if favoriteProduct, orderCount := snapshot.History.Favorite(); orderCount > 1 {
			lines = append(lines, fmt.Sprintf("你点得最多的是%s，一共 %d 次", favoriteProduct, orderCount))
		}
	}
	if line, ok := programmersDayLine(signals.now); ok {
		lines = append(lines, line)
	}

	if state.Mood == Sleepy {
		lines = append([]string{"这么晚了，我先睡啦，你也早点休息"}, lines...)
	}
	if len(lines) == 0 {
		lines = append(lines, "今天也要好好吃饭")
	}

	return lines
}

func mealLine(signals accountSignals, satiety int) (string, bool) {
	daysSinceMeal := int(signals.timeSinceMeal.Hours() / 24)

	switch {
	case signals.justAte() && len(signals.lastOrder.Products) > 0:
		return fmt.Sprintf("开饭啦，%s闻着就香", signals.lastOrder.Products[0]), true
	case signals.justAte():
		return "开饭啦", true
	case signals.hasLastOrder && satiety < starvingSatiety:
		return fmt.Sprintf("%d 天没见你来，我的杯子都空了", daysSinceMeal), true
	case signals.hasLastOrder && satiety < hungrySatiety:
		return fmt.Sprintf("%d 天没吃麦当劳了，我只剩小半杯", daysSinceMeal), true
	}

	return "", false
}

func growthLine(stage Stage, pointsKnown bool) (string, bool) {
	switch {
	case stage == DeluxeFlurry:
		return "我是至尊麦旋风，皇冠可得戴稳了", true
	case !pointsKnown:
		return "", false
	}

	nextStage := stage + 1

	return fmt.Sprintf("攒到 %s 积分，我就能加料变成%s", FormatNumber(stagePointThresholds[nextStage]), nextStage.Label()), true
}

func satietyAfter(timeSinceMeal time.Duration) int {
	if timeSinceMeal <= 0 {
		return 100
	}

	remainingShare := 1 - timeSinceMeal.Hours()/satietyDuration.Hours()

	return clamp(int(math.Round(remainingShare*100)), 0, 100)
}

func daysLeftInMonth(now time.Time) int {
	firstOfNextMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())

	return int(firstOfNextMonth.Sub(now).Hours() / 24)
}

func couponLine(now time.Time, coupon account.Coupon) string {
	remaining := coupon.ExpiresAt.Sub(now)

	switch {
	case remaining < time.Hour:
		return fmt.Sprintf("「%s」马上就过期了，快用掉它", coupon.Title)
	case remaining < 24*time.Hour:
		return fmt.Sprintf("「%s」只剩 %d 小时了，我急得都快化了", coupon.Title, int(remaining.Hours()))
	default:
		return fmt.Sprintf("「%s」还有 %d 天到期，别忘了用", coupon.Title, int(remaining.Hours()/24))
	}
}

func programmersDayLine(now time.Time) (string, bool) {
	programmersDay := time.Date(now.Year(), time.October, 24, 0, 0, 0, 0, now.Location())
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	daysLeft := int(programmersDay.Sub(today).Hours() / 24)

	switch {
	case daysLeft == 0:
		return "今天是 1024，祝你一次编译通过", true
	case daysLeft > 0 && daysLeft <= programmersDayCountdownDays:
		return fmt.Sprintf("再过 %d 天就是 1024 程序员节了", daysLeft), true
	}

	return "", false
}

func FormatNumber(value float64) string {
	text := strconv.FormatFloat(value, 'f', 1, 64)
	text = strings.TrimSuffix(text, ".0")
	integerPart, fractionPart, hasFraction := strings.Cut(text, ".")

	var grouped strings.Builder
	for index, digit := range integerPart {
		digitsRemaining := len(integerPart) - index
		if index > 0 && digitsRemaining%3 == 0 {
			grouped.WriteByte(',')
		}
		grouped.WriteRune(digit)
	}

	if hasFraction {
		return grouped.String() + "." + fractionPart
	}

	return grouped.String()
}

func firstN[T any](items []T, limit int) []T {
	if len(items) > limit {
		return items[:limit]
	}

	return items
}

func clamp(value, low, high int) int {
	return max(low, min(high, value))
}
