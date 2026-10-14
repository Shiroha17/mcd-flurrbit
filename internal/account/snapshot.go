package account

import (
	"context"
	"errors"
	"sort"
	"time"

	"flurrbit/internal/mcp"
)

var Beijing = time.FixedZone("CST", 8*60*60)

type Loader func(ctx context.Context) Snapshot

type Snapshot struct {
	Now            time.Time
	Points         *Points
	Wallet         *Wallet
	Claimable      *Claimable
	CampaignsToday []string
	History        *History
	Problems       []Problem
}

const InitializeStep = "initialize"

type Problem struct {
	Tool string
	Err  error
}

func (s Snapshot) Unauthorized() bool {
	for _, problem := range s.Problems {
		if errors.Is(problem.Err, mcp.ErrUnauthorized) {
			return true
		}
	}

	return false
}

func (s Snapshot) Offline() bool {
	for _, problem := range s.Problems {
		if problem.Tool == InitializeStep {
			return true
		}
	}

	mostToolsFailed := len(s.Problems) >= len(toolRequests)-1

	return mostToolsFailed
}

type Points struct {
	Available         float64
	Accumulated       float64
	ExpiringThisMonth float64
}

type Coupon struct {
	Title     string
	ExpiresAt time.Time
}

type Wallet struct {
	Coupons []Coupon
}

func (w Wallet) ExpiringWithin(now time.Time, window time.Duration) []Coupon {
	var expiringCoupons []Coupon
	for _, coupon := range w.Coupons {
		if coupon.ExpiresAt.IsZero() {
			continue
		}

		remaining := coupon.ExpiresAt.Sub(now)
		if remaining >= 0 && remaining <= window {
			expiringCoupons = append(expiringCoupons, coupon)
		}
	}

	sort.Slice(expiringCoupons, func(i, j int) bool {
		return expiringCoupons[i].ExpiresAt.Before(expiringCoupons[j].ExpiresAt)
	})

	return expiringCoupons
}

type Claimable struct {
	Unclaimed int
}

type Order struct {
	Store    string
	PlacedAt time.Time
	Products []string
}

type History struct {
	Orders []Order
}

func (h History) Latest() (Order, bool) {
	var latestOrder Order
	found := false
	for _, order := range h.Orders {
		if order.PlacedAt.IsZero() {
			continue
		}

		if !found || order.PlacedAt.After(latestOrder.PlacedAt) {
			latestOrder = order
			found = true
		}
	}

	return latestOrder, found
}

func (h History) Favorite() (string, int) {
	productCounts := map[string]int{}
	for _, order := range h.Orders {
		for _, product := range order.Products {
			productCounts[product]++
		}
	}

	favoriteProduct, highestCount := "", 0
	for product, count := range productCounts {
		isMoreOrdered := count > highestCount
		winsTie := count == highestCount && product < favoriteProduct
		if isMoreOrdered || winsTie {
			favoriteProduct, highestCount = product, count
		}
	}

	return favoriteProduct, highestCount
}
