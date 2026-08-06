package ranking

import (
	"encoding/json"
	"regexp"
	"strings"
)

// trailingCommaRe 匹配 }, ] 前的尾逗号（LLM 输出常见格式错误）。
var trailingCommaRe = regexp.MustCompile(`,(\s*[}\]])`)

// RepairJSON 修复 LLM 输出中的常见 JSON 问题（方案 §8.1 D2）：
//  1. 剥离 markdown 代码块围栏与首尾杂散文本（截取第一个 "{" 起）；
//  2. 删除尾逗号（{"a":1,} / [1,2,]）；
//  3. 截断输出：闭合未闭合的括号；
//  4. 部分恢复：输出在 ranked 数组中途截断时，回退到最后一个完整元素；
//  5. 多 JSON 对象：只保留第一个完整对象。
//
// 返回修复后的字符串；无法修复时原样返回（调用方解析仍会失败，走降级逻辑）。
func RepairJSON(raw string) string {
	s := strings.TrimSpace(raw)
	// 剥离 markdown 代码块围栏
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)

	// 截取第一个 "{" 起（丢弃前导解释文字）
	idx := strings.Index(s, "{")
	if idx < 0 {
		return s
	}
	s = s[idx:]

	// 尾逗号删除
	s = trailingCommaRe.ReplaceAllString(s, "$1")

	// 已是合法 JSON：截取第一个完整对象（丢弃尾部杂散文本/多余对象）
	if end := completeObjectEnd(s); end > 0 && json.Valid([]byte(s[:end])) {
		return s[:end]
	}

	// 截断输出：先尝试整体闭合括号
	if closed := closeBrackets(s); json.Valid([]byte(closed)) {
		return closed
	}

	// 部分恢复：从末尾逐个回退到最近的 "}"（丢弃不完整的末尾元素），再闭合括号
	for i := strings.LastIndex(s, "}"); i > 0; i = strings.LastIndex(s[:i], "}") {
		if closed := closeBrackets(s[:i+1]); json.Valid([]byte(closed)) {
			return closed
		}
	}
	return s
}

// completeObjectEnd 扫描第一个完整 JSON 对象的结束位置（不含后续的杂散文本）。
// 返回结束下标（exclusive）；对象未闭合时返回 -1。
func completeObjectEnd(s string) int {
	depth := 0
	inString := false
	escaped := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inString {
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{', '[':
			depth++
		case '}', ']':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return -1
}

// closeBrackets 闭合未闭合的括号：扫描括号栈（忽略字符串内容），
// 去掉末尾残留的逗号后，按栈逆序补齐 "]"/"}"。
func closeBrackets(s string) string {
	var stack []byte
	inString := false
	escaped := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inString {
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{', '[':
			stack = append(stack, c)
		case '}', ']':
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	// 截断在未闭合字符串中时无法可靠修复，交给上层部分恢复逻辑
	if inString {
		return s
	}
	// 去掉末尾残留逗号与空白，避免补出 "1,}" 这样的非法结果
	s = strings.TrimRight(s, " \t\r\n,")
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i] == '{' {
			s += "}"
		} else {
			s += "]"
		}
	}
	return s
}
