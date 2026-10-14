package account

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

type timeData struct {
	Formatted string `json:"formatted"`
}

type accountData struct {
	AvailablePoint          string `json:"availablePoint"`
	AccumulativePoint       string `json:"accumulativePoint"`
	CurrentMouthExpirePoint string `json:"currentMouthExpirePoint"`
}

type couponsData struct {
	Coupons []struct {
		Title        string `json:"title"`
		TradeEndDate string `json:"tradeEndDate"`
	} `json:"coupons"`
}

type claimableCoupon struct {
	CouponStatus string `json:"couponStatus"`
}

type calendarData struct {
	DailyList []struct {
		Today  bool `json:"today"`
		Events []struct {
			ActivityTitle string `json:"activityTitle"`
			ArticleDto    struct {
				Title string `json:"title"`
			} `json:"articleDto"`
		} `json:"events"`
	} `json:"dailyList"`
}

type ordersData struct {
	List []struct {
		StoreName        string `json:"storeName"`
		CreateTime       string `json:"createTime"`
		OrderProductList []struct {
			ProductName string `json:"productName"`
		} `json:"orderProductList"`
	} `json:"list"`
}

const claimableCouponStatus = "CAN_GET"

func decodeData[T any](text string) (T, bool) {
	var response struct {
		Data *T `json:"data"`
	}
	if json.Unmarshal([]byte(text), &response) != nil || response.Data == nil {
		var zero T

		return zero, false
	}

	return *response.Data, true
}

func parseNumber(text string) (float64, bool) {
	value, err := strconv.ParseFloat(strings.TrimSpace(text), 64)

	return value, err == nil
}

func parseServerTime(text string) (time.Time, bool) {
	data, ok := decodeData[timeData](text)
	if !ok {
		return time.Time{}, false
	}

	serverTime, err := time.ParseInLocation(time.DateTime, data.Formatted, Beijing)

	return serverTime, err == nil
}

func parsePoints(text string) *Points {
	data, ok := decodeData[accountData](text)
	if !ok {
		return nil
	}

	available, ok := parseNumber(data.AvailablePoint)
	if !ok {
		return nil
	}

	points := &Points{Available: available}
	points.Accumulated, _ = parseNumber(data.AccumulativePoint)
	points.ExpiringThisMonth, _ = parseNumber(data.CurrentMouthExpirePoint)

	return points
}

func parseWallet(text string) *Wallet {
	data, ok := decodeData[couponsData](text)
	if !ok {
		return nil
	}

	wallet := &Wallet{}
	for _, coupon := range data.Coupons {
		if coupon.Title == "" {
			continue
		}

		wallet.Coupons = append(wallet.Coupons, Coupon{
			Title:     coupon.Title,
			ExpiresAt: endOfDay(coupon.TradeEndDate),
		})
	}

	return wallet
}

func endOfDay(date string) time.Time {
	day, err := time.ParseInLocation(time.DateOnly, date, Beijing)
	if err != nil {
		return time.Time{}
	}

	return day.Add(24*time.Hour - time.Minute)
}

func parseClaimable(text string) *Claimable {
	coupons, ok := decodeData[[]claimableCoupon](text)
	if !ok {
		return nil
	}

	claimable := &Claimable{}
	for _, coupon := range coupons {
		if coupon.CouponStatus == claimableCouponStatus {
			claimable.Unclaimed++
		}
	}

	return claimable
}

func parseCampaignsToday(text string) []string {
	data, ok := decodeData[calendarData](text)
	if !ok {
		return nil
	}

	seenTitles := map[string]bool{}
	var titles []string
	for _, day := range data.DailyList {
		if !day.Today {
			continue
		}

		for _, event := range day.Events {
			title := strings.TrimSpace(event.ActivityTitle)
			if title == "" {
				title = strings.TrimSpace(event.ArticleDto.Title)
			}

			if title != "" && !seenTitles[title] {
				seenTitles[title] = true
				titles = append(titles, title)
			}
		}
	}

	return titles
}

func parseHistory(text string) *History {
	data, ok := decodeData[ordersData](text)
	if !ok {
		return nil
	}

	history := &History{}
	for _, order := range data.List {
		placedAt, _ := time.ParseInLocation(time.DateTime, order.CreateTime, Beijing)

		var productNames []string
		for _, product := range order.OrderProductList {
			productNames = append(productNames, product.ProductName)
		}

		history.Orders = append(history.Orders, Order{
			Store:    order.StoreName,
			PlacedAt: placedAt,
			Products: productNames,
		})
	}

	return history
}
