package analysis

// 5 档决策标尺（方案 §8.1 D5）：标准化的 分数 -> 操作 映射。
// 纯数据与纯函数，供合成/风控覆盖/前端展示统一引用。

// DecisionSignal 决策信号键。
type DecisionSignal string

const (
	SignalStrongBuy DecisionSignal = "strong_buy"
	SignalBuy       DecisionSignal = "buy"
	SignalWatch     DecisionSignal = "watch"
	SignalReduce    DecisionSignal = "reduce"
	SignalSell      DecisionSignal = "sell"
)

// 操作动作（DecisionBand.Action 的取值）。
const (
	ActionBuy  = "buy"
	ActionHold = "hold"
	ActionSell = "sell"
)

// DecisionBand 决策标尺中的一档。
type DecisionBand struct {
	MinScore float64        // 区间下限（含）
	MaxScore float64        // 区间上限（含，展示用；判定按下限从高到低匹配）
	Signal   DecisionSignal // 信号键
	Action   string         // buy / hold / sell
	LabelZH  string         // 中文标签
}

// DecisionScale 5 档标尺（方案 §8.1 D5，按分数从高到低排列）。
var DecisionScale = []DecisionBand{
	{MinScore: 80, MaxScore: 100, Signal: SignalStrongBuy, Action: ActionBuy, LabelZH: "强烈买入"},
	{MinScore: 60, MaxScore: 79, Signal: SignalBuy, Action: ActionBuy, LabelZH: "买入"},
	{MinScore: 40, MaxScore: 59, Signal: SignalWatch, Action: ActionHold, LabelZH: "观望"},
	{MinScore: 20, MaxScore: 39, Signal: SignalReduce, Action: ActionSell, LabelZH: "减仓"},
	{MinScore: 0, MaxScore: 19, Signal: SignalSell, Action: ActionSell, LabelZH: "卖出"},
}

// SignalForScore 查询分数所属档位。
// 分数先钳位到 [0,100]，再按下限从高到低匹配（如 79.5 落入 60-79 的 buy 档）。
func SignalForScore(score float64) DecisionBand {
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}
	for _, band := range DecisionScale {
		if score >= band.MinScore {
			return band
		}
	}
	return DecisionScale[len(DecisionScale)-1]
}

// NeedsGuardrail 护栏校验（方案 §8.1 D5）：action 与分数所属档位的标准动作不一致时，
// 必须填写 guardrail_reason 说明原因（如分数 >= 60 对应买入档但 action 是 hold，或反过来）。
func NeedsGuardrail(score float64, action string) bool {
	return SignalForScore(score).Action != action
}
