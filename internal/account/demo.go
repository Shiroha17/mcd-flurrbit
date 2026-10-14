package account

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"flurrbit/internal/mcp"
)

var DemoScenarios = []string{"happy", "excited", "eating", "hungry", "starving", "anxious", "sleepy", "deluxe"}

type Demo struct {
	Scenario string
	Today    time.Time
}

func NewDemo(scenario string, today time.Time) (*Demo, error) {
	if !slices.Contains(DemoScenarios, scenario) {
		return nil, fmt.Errorf("没有 %s 这个演示场景，可以试试 %s", scenario, strings.Join(DemoScenarios, "、"))
	}

	return &Demo{Scenario: scenario, Today: today.In(Beijing)}, nil
}

func (d *Demo) Now() time.Time {
	year, month, day := d.Today.Date()
	hour, minute := 12, 30
	if d.Scenario == "sleepy" {
		hour, minute = 2, 30
	}

	return time.Date(year, month, day, hour, minute, 0, 0, Beijing)
}

func (d *Demo) CallTool(_ context.Context, tool string, _ map[string]any) (string, error) {
	if !mcp.IsReadOnly(tool) {
		return "", mcp.ForbiddenToolError{Tool: tool}
	}

	switch tool {
	case mcp.ToolNowTimeInfo:
		return d.timeResponse(), nil
	case mcp.ToolQueryMyAccount:
		return d.accountResponse(), nil
	case mcp.ToolQueryMyCoupons:
		return d.couponsResponse(), nil
	case mcp.ToolAvailableCoupons:
		return d.claimableResponse(), nil
	case mcp.ToolCampaignCalendar:
		return d.calendarResponse(), nil
	case mcp.ToolOrderList:
		return d.ordersResponse(), nil
	}

	return "", nil
}

func (d *Demo) timeResponse() string {
	return fmt.Sprintf(`{"success":true,"code":200,"data":{"formatted":"%s"}}`, d.Now().Format(time.DateTime))
}

func (d *Demo) accountResponse() string {
	accumulatedPoints, pointsExpiringThisMonth := 6144.0, 0.0
	switch d.Scenario {
	case "deluxe":
		accumulatedPoints = 30720
	case "anxious":
		pointsExpiringThisMonth = 17.1
	}

	return fmt.Sprintf(`{"success":true,"code":200,"data":{"availablePoint":"2048.5","accumulativePoint":"%.0f","currentMouthExpirePoint":"%.1f","nextMouthExpirePoint":"120"}}`,
		accumulatedPoints, pointsExpiringThisMonth)
}

func (d *Demo) couponsResponse() string {
	soonestExpiry := d.Now().AddDate(0, 0, 5)
	if d.Scenario == "anxious" {
		soonestExpiry = d.Now()
	}
	dateOf := func(moment time.Time) string { return moment.Format(time.DateOnly) }

	return fmt.Sprintf(`{"success":true,"code":200,"data":{"coupons":[`+
		`{"title":"9.9元麦辣鸡翅两块","tradeEndDate":"%s"},`+
		`{"title":"中杯美式咖啡 7 折","tradeEndDate":"%s"},`+
		`{"title":"薯条第二份半价","tradeEndDate":"%s"}`+
		`]}}`, dateOf(soonestExpiry), dateOf(d.Now().AddDate(0, 0, 12)), dateOf(d.Now().AddDate(0, 0, 30)))
}

func (d *Demo) claimableResponse() string {
	couponStatus := "HAVING_RECEIVE"
	if d.Scenario == "excited" {
		couponStatus = "CAN_GET"
	}

	return fmt.Sprintf(`{"success":true,"code":200,"data":[`+
		`{"couponName":"板烧鸡腿堡 13.9 元","couponStatus":"%s"},`+
		`{"couponName":"麦乐鸡 5 块 9.9 元","couponStatus":"%s"},`+
		`{"couponName":"可乐中杯 5 元","couponStatus":"HAVING_RECEIVE"}`+
		`]}`, couponStatus, couponStatus)
}

func (d *Demo) calendarResponse() string {
	campaignTitle := "秋日麦旋风季"
	isToday := d.Scenario == "excited"
	if isToday {
		campaignTitle = "薯条加大不加价周"
	}

	return fmt.Sprintf(`{"success":true,"code":200,"data":{"dailyList":[{"date":"%s","today":%t,"events":[{"activityTitle":"%s"}]}]}}`,
		d.Now().Format(time.DateOnly), isToday, campaignTitle)
}

func (d *Demo) ordersResponse() string {
	timeSinceLastOrder := 26 * time.Hour
	switch d.Scenario {
	case "eating":
		timeSinceLastOrder = 25 * time.Minute
	case "hungry":
		timeSinceLastOrder = 9 * 24 * time.Hour
	case "starving":
		timeSinceLastOrder = 20 * 24 * time.Hour
	}
	lastOrderTime := d.Now().Add(-timeSinceLastOrder)
	earlierOrderTime := lastOrderTime.AddDate(0, 0, -4)

	return fmt.Sprintf(`{"success":true,"code":200,"data":{"list":[`+
		`{"storeName":"示例广场餐厅","createTime":"%s","orderProductList":[{"productName":"麦辣鸡腿汉堡套餐"}]},`+
		`{"storeName":"示例大厦餐厅","createTime":"%s","orderProductList":[{"productName":"麦辣鸡腿汉堡套餐"},{"productName":"中薯条"}]}`+
		`]}}`, lastOrderTime.Format(time.DateTime), earlierOrderTime.Format(time.DateTime))
}
