package account

import (
	"context"
	"strings"
	"sync"
	"time"

	"flurrbit/internal/mcp"
)

type toolRequest struct {
	tool      string
	arguments map[string]any
}

var toolRequests = []toolRequest{
	{tool: mcp.ToolNowTimeInfo},
	{tool: mcp.ToolQueryMyAccount},
	{tool: mcp.ToolQueryMyCoupons, arguments: map[string]any{"page": "1", "pageSize": "200"}},
	{tool: mcp.ToolAvailableCoupons},
	{tool: mcp.ToolCampaignCalendar},
	{tool: mcp.ToolOrderList},
}

func Fetch(ctx context.Context, caller mcp.ToolCaller, fallbackNow time.Time) Snapshot {
	responsesByTool := make(map[string]string, len(toolRequests))
	var problems []Problem
	var resultsMutex sync.Mutex
	var waitGroup sync.WaitGroup

	for _, request := range toolRequests {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()

			text, err := caller.CallTool(ctx, request.tool, request.arguments)

			resultsMutex.Lock()
			defer resultsMutex.Unlock()

			switch {
			case err != nil:
				problems = append(problems, Problem{Tool: request.tool, Err: err})
			case strings.TrimSpace(text) != "":
				responsesByTool[request.tool] = text
			}
		}()
	}
	waitGroup.Wait()

	return assembleSnapshot(responsesByTool, problems, fallbackNow)
}

func assembleSnapshot(responsesByTool map[string]string, problems []Problem, fallbackNow time.Time) Snapshot {
	snapshot := Snapshot{Now: fallbackNow.In(Beijing), Problems: problems}
	if text, ok := responsesByTool[mcp.ToolNowTimeInfo]; ok {
		if serverNow, ok := parseServerTime(text); ok {
			snapshot.Now = serverNow
		}
	}

	if text, ok := responsesByTool[mcp.ToolQueryMyAccount]; ok {
		snapshot.Points = parsePoints(text)
	}
	if text, ok := responsesByTool[mcp.ToolQueryMyCoupons]; ok {
		snapshot.Wallet = parseWallet(text)
	}
	if text, ok := responsesByTool[mcp.ToolAvailableCoupons]; ok {
		snapshot.Claimable = parseClaimable(text)
	}
	if text, ok := responsesByTool[mcp.ToolCampaignCalendar]; ok {
		snapshot.CampaignsToday = parseCampaignsToday(text)
	}
	if text, ok := responsesByTool[mcp.ToolOrderList]; ok {
		snapshot.History = parseHistory(text)
	}

	return snapshot
}
