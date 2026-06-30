package skill_analysis

const generateSkillPrompt = `根据以下内容生成一个 go-stock Skill 配置，返回 JSON：
{
  "name": "技能名称",
  "category": "分类",
  "description": "一句话描述",
  "systemPrompt": "系统提示词",
  "examples": "示例对话",
  "triggerKeywords": "触发关键词,逗号分隔",
  "confidence": 0-1
}

内容：
%s
`
