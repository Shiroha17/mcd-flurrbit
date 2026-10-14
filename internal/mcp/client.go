package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	DefaultURL      = "https://mcp.mcd.cn"
	protocolVersion = "2025-06-18"
	jsonRPCVersion  = "2.0"
	clientName      = "flurrbit"
)

const (
	ToolNowTimeInfo      = "now-time-info"
	ToolQueryMyAccount   = "query-my-account"
	ToolQueryMyCoupons   = "query-my-coupons"
	ToolAvailableCoupons = "available-coupons"
	ToolCampaignCalendar = "campaign-calendar"
	ToolOrderList        = "order-list"
)

const (
	errorPreviewLength  = 200
	errorBodyReadLimit  = 512
	eventLineBufferSize = 64 * 1024
	maxEventLineSize    = 16 * 1024 * 1024
)

var ReadOnlyTools = []string{
	ToolNowTimeInfo,
	ToolQueryMyAccount,
	ToolQueryMyCoupons,
	ToolAvailableCoupons,
	ToolCampaignCalendar,
	ToolOrderList,
}

var (
	ErrUnauthorized = errors.New("MCP Token 无效或者过期啦（401），到 https://open.mcd.cn/mcp 重新激活一下")
	ErrRateLimited  = errors.New("请求太频繁啦（429），每个 Token 每分钟最多 600 次")
)

type ForbiddenToolError struct {
	Tool string
}

func (e ForbiddenToolError) Error() string {
	return fmt.Sprintf("旋宝只读取数据，不会调用 %s", e.Tool)
}

type ToolCaller interface {
	CallTool(ctx context.Context, tool string, arguments map[string]any) (string, error)
}

type Client struct {
	url        string
	token      string
	httpClient *http.Client

	sessionMutex  sync.Mutex
	sessionID     string
	lastRequestID atomic.Int64
}

func NewClient(url, token string, timeout time.Duration) *Client {
	return &Client{
		url:        url,
		token:      token,
		httpClient: &http.Client{Timeout: timeout},
	}
}

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  any             `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func (c *Client) Initialize(ctx context.Context) (ServerInfo, error) {
	params := map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": clientName, "version": Version},
	}

	response, err := c.request(ctx, "initialize", params)
	if err != nil {
		return ServerInfo{}, err
	}

	var result struct {
		ServerInfo ServerInfo `json:"serverInfo"`
	}
	_ = json.Unmarshal(response.Result, &result)

	initializedNotification := rpcMessage{JSONRPC: jsonRPCVersion, Method: "notifications/initialized"}
	if _, err := c.post(ctx, initializedNotification); err != nil {
		return ServerInfo{}, err
	}

	return result.ServerInfo, nil
}

func (c *Client) ListTools(ctx context.Context) ([]string, error) {
	response, err := c.request(ctx, "tools/list", nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(response.Result, &result); err != nil {
		return nil, fmt.Errorf("tools/list 返回无法解析: %w", err)
	}

	names := make([]string, 0, len(result.Tools))
	for _, tool := range result.Tools {
		names = append(names, tool.Name)
	}

	return names, nil
}

func (c *Client) CallTool(ctx context.Context, tool string, arguments map[string]any) (string, error) {
	if !IsReadOnly(tool) {
		return "", ForbiddenToolError{Tool: tool}
	}

	if arguments == nil {
		arguments = map[string]any{}
	}

	response, err := c.request(ctx, "tools/call", map[string]any{"name": tool, "arguments": arguments})
	if err != nil {
		return "", err
	}

	var result toolResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		return "", fmt.Errorf("%s 返回无法解析: %w", tool, err)
	}

	text := result.Text()
	if result.IsError {
		return "", fmt.Errorf("%s 返回错误: %s", tool, truncate(text, errorPreviewLength))
	}

	return text, nil
}

func IsReadOnly(tool string) bool {
	return slices.Contains(ReadOnlyTools, tool)
}

type toolResult struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StructuredContent json.RawMessage `json:"structuredContent"`
	IsError           bool            `json:"isError"`
}

func (r toolResult) Text() string {
	if len(r.StructuredContent) > 0 && string(r.StructuredContent) != "null" {
		return string(r.StructuredContent)
	}

	var textBlocks []string
	for _, block := range r.Content {
		if block.Type == "text" {
			textBlocks = append(textBlocks, block.Text)
		}
	}

	return strings.Join(textBlocks, "\n")
}

func (c *Client) request(ctx context.Context, method string, params any) (rpcMessage, error) {
	requestID := c.lastRequestID.Add(1)

	response, err := c.post(ctx, rpcMessage{JSONRPC: jsonRPCVersion, ID: &requestID, Method: method, Params: params})
	if err != nil {
		return rpcMessage{}, err
	}
	if response == nil {
		return rpcMessage{}, fmt.Errorf("%s 没有返回内容", method)
	}
	if response.Error != nil {
		return rpcMessage{}, fmt.Errorf("JSON-RPC 错误 %d: %s", response.Error.Code, response.Error.Message)
	}

	return *response, nil
}

func (c *Client) post(ctx context.Context, message rpcMessage) (*rpcMessage, error) {
	payload, err := json.Marshal(message)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("User-Agent", clientName+"/"+Version)
	if sessionID := c.GetSession(); sessionID != "" {
		request.Header.Set("Mcp-Session-Id", sessionID)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("连不上麦当劳 MCP：%w", err)
	}
	defer response.Body.Close()

	switch {
	case response.StatusCode == http.StatusUnauthorized:
		return nil, ErrUnauthorized
	case response.StatusCode == http.StatusTooManyRequests:
		return nil, ErrRateLimited
	case response.StatusCode >= http.StatusMultipleChoices:
		errorBody, _ := io.ReadAll(io.LimitReader(response.Body, errorBodyReadLimit))
		return nil, fmt.Errorf("HTTP %d: %s", response.StatusCode, truncate(string(errorBody), errorPreviewLength))
	}

	if sessionID := response.Header.Get("Mcp-Session-Id"); sessionID != "" {
		c.SetSession(sessionID)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	trimmedBody := bytes.TrimSpace(body)
	switch {
	case len(trimmedBody) == 0:
		return nil, nil
	case trimmedBody[0] == '{':
		return decodeJSONBody(trimmedBody)
	default:
		return decodeEventStream(body)
	}
}

func (c *Client) GetSession() string {
	c.sessionMutex.Lock()
	defer c.sessionMutex.Unlock()

	return c.sessionID
}

func (c *Client) SetSession(sessionID string) {
	c.sessionMutex.Lock()
	defer c.sessionMutex.Unlock()

	c.sessionID = sessionID
}

func decodeJSONBody(body []byte) (*rpcMessage, error) {
	var message rpcMessage
	if err := json.Unmarshal(body, &message); err != nil {
		return nil, fmt.Errorf("响应不是合法 JSON: %w", err)
	}

	return &message, nil
}

func decodeEventStream(body []byte) (*rpcMessage, error) {
	var response *rpcMessage
	var eventData strings.Builder

	finishEvent := func() {
		if eventData.Len() == 0 {
			return
		}

		var message rpcMessage
		isResponse := json.Unmarshal([]byte(eventData.String()), &message) == nil && (message.Result != nil || message.Error != nil)
		if isResponse {
			response = &message
		}

		eventData.Reset()
	}

	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 0, eventLineBufferSize), maxEventLineSize)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "data:"):
			if eventData.Len() > 0 {
				eventData.WriteByte('\n')
			}
			eventData.WriteString(strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		case strings.TrimSpace(line) == "":
			finishEvent()
		}
	}
	finishEvent()

	if response == nil {
		return nil, errors.New("SSE 流中没有找到 JSON-RPC 响应")
	}

	return response, nil
}

func truncate(text string, limit int) string {
	characters := []rune(text)
	if len(characters) <= limit {
		return text
	}

	return string(characters[:limit]) + "…"
}
