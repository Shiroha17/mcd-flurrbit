# MCP 集成说明

## MCP Server

| 项目 | 值 |
|---|---|
| Server | [麦当劳中国官方 MCP Server](https://github.com/M-China/mcd-mcp-server) |
| 地址 | `https://mcp.mcd.cn` |
| 传输 | Streamable HTTP，JSON-RPC 2.0，协议版本 `2025-06-18` |
| 鉴权 | `Authorization: Bearer ${MCD_MCP_TOKEN}` |

## Tool

只调用 6 个查询类 Tool，白名单之外的 Tool 在请求发出前就会被拦下。

| Tool | 读取的字段 | 用途 |
|---|---|---|
| `now-time-info` | `formatted` | 当前时间：犯困时段、各种倒计时 |
| `query-my-account` | `availablePoint`、`accumulativePoint`、`currentMouthExpirePoint` | 成长阶段，月底积分过期提醒 |
| `query-my-coupons` | `coupons[].title`、`tradeEndDate` | 快过期的券 |
| `available-coupons` | `couponStatus` | 麦麦省没领的券 |
| `campaign-calendar` | `dailyList[].today`、`events[].activityTitle` | 今天的活动 |
| `order-list` | `list[].createTime`、`orderProductList[].productName` | 饱腹度、最爱餐品 |

## 调用流程

```
每 10 分钟一次
  ├─ initialize → notifications/initialized
  ├─ 并发 tools/call 上面 6 个 Tool，单个失败只影响自己那部分
  ├─ 解析 structuredContent，组装成账户快照
  └─ 算出心情、饱腹、淡定、成长阶段和台词
```

## 业务价值

麦当劳 App 里的积分、卡包和活动分散在不同页面，过期提醒很容易被忽略。旋宝把它们变成桌面上一个不会被划走的提醒：

- **券不浪费**：券快过期时，它会冒汗并说出券名和剩余时间。
- **积分止损**：月底有积分要过期，提前一周开始着急。
- **活动触达**：有券没领、今天有活动，它会兴奋地告诉你。
- **长期陪伴**：饱腹度和成长阶段让人愿意常来看看它。

全程只读，不会下单、领券或消耗积分。
