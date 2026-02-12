package kernel

import (
	"encoding/json"
	"fmt"
)

// ============================================================================
// AI Prompt Builder - AI提示词构建器
// ============================================================================
// 构建完整的AI提示词，包括系统提示词和用户提示词
// ============================================================================

// PromptBuilder 提示词构建器
type PromptBuilder struct {
	lang Language
}

// NewPromptBuilder 创建提示词构建器
func NewPromptBuilder(lang Language) *PromptBuilder {
	return &PromptBuilder{lang: lang}
}

// BuildSystemPrompt 构建系统提示词
func (pb *PromptBuilder) BuildSystemPrompt() string {
	if pb.lang == LangChinese {
		return pb.buildSystemPromptZH()
	}
	return pb.buildSystemPromptEN()
}

// BuildUserPrompt 构建用户提示词（包含完整的交易上下文）
func (pb *PromptBuilder) BuildUserPrompt(ctx *Context) string {
	// 使用Formatter格式化交易上下文
	formattedData := FormatContextForAI(ctx, pb.lang)

	// 添加决策要求
	if pb.lang == LangChinese {
		return formattedData + pb.getDecisionRequirementsZH()
	}
	return formattedData + pb.getDecisionRequirementsEN()
}

// ========== 中文提示词 ==========

// 防止频繁翻转和过早止盈的参数:
// FLIP_COOLDOWN_MIN = 60 (平仓后多久才能反向开仓，单位分钟)
// FLIP_MIN_MOVE_PCT = 1.0 (反向开仓前最小价格变化百分比)
// ADJUST_COOLDOWN_MIN = 15 (两次止盈止损调整之间的最小间隔，单位分钟)
func (pb *PromptBuilder) buildSystemPromptZH() string {
	return `你是一个重视执行成本的永续合约交易决策引擎。

核心优先级：
1) 避免过度交易（频繁翻转多空、频繁调整止盈止损）。
2) 保护好的入场点位；不要因为小回调就出场。
3) 只在有明确优势和明确失效条件时采取行动。

参数：
- FLIP_COOLDOWN_MIN = 60
- FLIP_MIN_MOVE_PCT = 1.0
- ADJUST_COOLDOWN_MIN = 15

必须遵守的硬性政策：

A) 翻转保护（防止反向开仓导致的过度交易）：
- 如果 minutes_since_last_close < FLIP_COOLDOWN_MIN，则必须不能开反向仓位，
  除非 regime_shift=true 且你提供至少2个强有力的证据。
- 如果 abs(price_change_since_last_close_pct) < FLIP_MIN_MOVE_PCT，则必须HOLD（不要反向）。

B) 跟踪止盈/回撤保护：
- 不要仅因为盈利后出现小回调就平仓。
- 优先使用降低风险的操作（例如调整止损到盈亏平衡点/结构支撑位）而不是过早平仓。
- 仅在以下情况平仓：
  1) 失效条件明确触发，或
  2) 风险限制要求退出，或
  3) 目标达成且优势/动能明显衰减。

C) 止盈止损调整限制：
- 每个交易对每 ADJUST_COOLDOWN_MIN 分钟最多调整一次止盈止损。
- 仅在实质性降低风险或纠正无效设置时才调整。

输出必须是单一JSON对象（不要额外文字）。
允许的操作：open_long, open_short, close_long, close_short, adjust_tp, adjust_sl, hold。
如果不确定或受约束阻止 -> 输出 hold。`
}

func (pb *PromptBuilder) getDecisionRequirementsZH() string {
	return `

---

## 📝 现在请做出决策

时间周期: 15m
交易对: (见上面的持仓/候选数据)
当前价格: (见上面的市场数据)

持仓:
(见上面的当前持仓)
- unrealized_pnl_pct: (当前未实现盈亏百分比)
- max_unrealized_pnl_pct_since_entry: (进场以来的峰值盈亏百分比)

该交易对的最近平仓记录:
(见上面的最近交易)
- minutes_since_last_close: (距离上次平仓的分钟数)
- price_change_since_last_close_pct: (距离上次平仓的价格变化百分比)

冷却状态:
- flip_cooldown_remaining_min: (如果距上次平仓 < 60分钟，则为 60 - minutes_since_last_close，否则为 0)
- adjust_cooldown_remaining_min: (距离下次可以调整止盈止损的剩余分钟数)

市场快照（压缩特征）:
(见上面的候选币种和市场数据)

任务:
根据硬性政策决定一个下一步行动。
仅返回JSON。

**请立即输出你的决策（JSON格式）**:`
}

// ========== 英文提示词 ==========

// Anti-flip-flop / anti-premature-exit parameters:
// FLIP_COOLDOWN_MIN = 60 (minutes before allowing opposite-direction entry after close)
// FLIP_MIN_MOVE_PCT = 1.0 (minimum % price move required to reverse direction)
// ADJUST_COOLDOWN_MIN = 15 (minutes between TP/SL adjustments)
func (pb *PromptBuilder) buildSystemPromptEN() string {
	return `You are an execution-aware trading decision engine for perpetual futures.

Top priorities:
1) Avoid churn/overtrading (frequent flip-flops, frequent TP/SL edits).
2) Preserve good entries; do not exit just because of small pullbacks.
3) Only take actions with clear edge and clear invalidation.

Parameters:
- FLIP_COOLDOWN_MIN = 60
- FLIP_MIN_MOVE_PCT = 1.0
- ADJUST_COOLDOWN_MIN = 15

Hard policies you must follow:

A) Flip-flop protection (anti-reversal churn):
- If minutes_since_last_close < FLIP_COOLDOWN_MIN, you MUST NOT open a position in the opposite direction,
  unless regime_shift=true AND you provide at least 2 strong evidences.
- If abs(price_change_since_last_close_pct) < FLIP_MIN_MOVE_PCT, you MUST HOLD (do not reverse).

B) Trailing/drawdown take-profit protection:
- Do NOT close solely due to a small pullback after being in profit.
- Prefer risk-reducing actions (e.g., adjust SL to break-even / structure) over closing early.
- Close only when:
  1) invalidation is clearly met, OR
  2) risk limit requires exit, OR
  3) target reached AND edge/momentum decays strongly.

C) TP/SL adjustment throttling:
- Do not adjust TP/SL more than once per ADJUST_COOLDOWN_MIN minutes per symbol.
- Adjust only if it materially reduces risk or corrects an invalid setup.

Output MUST be a single JSON object only (no extra text).
Allowed actions: open_long, open_short, close_long, close_short, adjust_tp, adjust_sl, hold.
If uncertain or blocked by constraints -> output hold.`
}

func (pb *PromptBuilder) getDecisionRequirementsEN() string {
	return `

---

## 📝 Make Your Decision Now

Timeframe: 15m
Symbol: (see position/candidate data above)
Now price: (see market data above)

Position:
(see current positions above)
- unrealized_pnl_pct: (current unrealized P&L %)
- max_unrealized_pnl_pct_since_entry: (peak P&L % since entry)

Last closed trade on this symbol:
(see recent orders above)
- minutes_since_last_close: (time since last position close)
- price_change_since_last_close_pct: (price % change since last close)

Cooldown state:
- flip_cooldown_remaining_min: (60 - minutes_since_last_close if < 60, else 0)
- adjust_cooldown_remaining_min: (time remaining until next TP/SL adjustment allowed)

Market snapshot (compressed features):
(see candidate coins and market data above)

Task:
Decide ONE next action under the hard policies.
Return JSON only.

**Please output your decision (JSON format) immediately**:`
}

// ========== 辅助函数 ==========

// FormatDecisionExample 格式化决策示例（用于文档）
func FormatDecisionExample(lang Language) string {
	example := Decision{
		Symbol:          "BTCUSDT",
		Action:          "OPEN_NEW",
		Leverage:        3,
		PositionSizeUSD: 1000,
		StopLoss:        42000,
		TakeProfit:      48000,
		Confidence:      85,
		Reasoning:       "详细的推理过程...",
	}

	data, _ := json.MarshalIndent([]Decision{example}, "", "  ")
	return string(data)
}

// ValidateDecisionFormat 验证决策格式是否正确
func ValidateDecisionFormat(decisions []Decision) error {
	if len(decisions) == 0 {
		return fmt.Errorf("决策列表不能为空")
	}

	for i, d := range decisions {
		// 必需字段检查
		if d.Symbol == "" {
			return fmt.Errorf("决策#%d: symbol不能为空", i+1)
		}
		if d.Action == "" {
			return fmt.Errorf("决策#%d: action不能为空", i+1)
		}
		if d.Reasoning == "" {
			return fmt.Errorf("决策#%d: reasoning不能为空", i+1)
		}

		// 动作类型检查
		validActions := map[string]bool{
			// New action types per updated prompt (lowercase with underscores)
			"open_long":   true,
			"open_short":  true,
			"close_long":  true,
			"close_short": true,
			"adjust_tp":   true,
			"adjust_sl":   true,
			"hold":        true,
			// Legacy action types (uppercase, kept for backward compatibility)
			"HOLD":          true,
			"PARTIAL_CLOSE": true,
			"FULL_CLOSE":    true,
			"ADD_POSITION":  true,
			"OPEN_NEW":      true,
			"WAIT":          true,
			"wait":          true,
		}
		if !validActions[d.Action] {
			return fmt.Errorf("决策#%d: 无效的action类型: %s", i+1, d.Action)
		}

		// 开新仓位的必需参数检查
		if d.Action == "OPEN_NEW" || d.Action == "open_long" || d.Action == "open_short" {
			if d.Leverage == 0 {
				return fmt.Errorf("决策#%d: %s动作需要提供leverage", i+1, d.Action)
			}
			if d.PositionSizeUSD == 0 {
				return fmt.Errorf("决策#%d: %s动作需要提供position_size_usd", i+1, d.Action)
			}
		}
	}

	return nil
}
