/**
 * 系统相关 API
 * 封装设置、配置、版本等系统相关调用
 */

import { callApi } from './client'
import * as App from '../../wailsjs/go/main/App'
import * as SystemHandler from '../../wailsjs/go/handler/SystemHandler'
import * as AnalysisHandler from '../../wailsjs/go/handler/AnalysisHandler'
import * as AgentHandler from '../../wailsjs/go/handler/AgentHandler'
import * as NotificationHandler from '../../wailsjs/go/handler/NotificationHandler'

// ========== 版本信息 ==========

/**
 * 获取版本信息
 * @returns {Promise<ApiResult>}
 */
export async function getVersionInfo(): Promise<any> {
  return callApi(SystemHandler.GetVersionInfo)
}

/**
 * 检查更新
 * @returns {Promise<ApiResult>}
 */
export async function checkUpdate(): Promise<any> {
  return callApi(SystemHandler.CheckUpdate)
}

// ========== 设置相关 ==========

/**
 * 获取运行时配置 (GetConfig - 功能开关如 enableFund/enableAgent/darkTheme)
 * 注意：与 getSettings 不同，GetConfig 返回的是功能开关配置
 * @returns {Promise<ApiResult>}
 */
export async function getConfig(): Promise<any> {
  return callApi(SystemHandler.GetConfig)
}

// ========== AI 分析结果保存 ==========

/**
 * 保存 AI 响应结果
 * @param {string} arg1
 * @param {string} arg2
 * @param {string} arg3
 * @param {string} arg4
 * @param {string} arg5
 * @param {number} arg6
 * @returns {Promise<ApiResult>}
 */
export async function saveAiResponseResult(arg1: string, arg2: string, arg3: string, arg4: string, arg5: string, arg6: number): Promise<any> {
  return callApi(AnalysisHandler.SaveAIResponseResult, arg1, arg2, arg3, arg4, arg5, arg6)
}

/**
 * 保存为 Markdown 文件
 * @param {string} title
 * @param {string} content
 * @returns {Promise<ApiResult>}
 */
export async function saveAsMarkdown(title: string, content: string): Promise<any> {
  return callApi(AnalysisHandler.SaveAsMarkdown, title, content)
}

/**
 * 分享分析结果
 * @param {string} arg1
 * @param {string} arg2
 * @returns {Promise<ApiResult>}
 */
export async function shareAnalysis(arg1: string, arg2: string): Promise<any> {
  return callApi(AnalysisHandler.ShareAnalysis, arg1, arg2)
}

// ========== AI 配置 ==========

/**
 * 获取所有策略
 * Go: GetAllStrategies() []*strategy.Strategy
 * @returns {Promise<ApiResult>}
 */
export async function getAllStrategies(): Promise<any> {
  return callApi(AgentHandler.GetAllStrategies)
}

/**
 * 获取 AI 配置列表
 * @returns {Promise<ApiResult>}
 */
export async function getAiConfigs(): Promise<any> {
  return callApi(SystemHandler.GetAiConfigs)
}

// ========== Cron 定时任务 ==========

/**
 * 创建定时任务
 * @param {Object} task - 任务配置
 * @returns {Promise<ApiResult>}
 */
export async function createCronTask(task: any): Promise<any> {
  return callApi(SystemHandler.CreateCronTask, task)
}

/**
 * 更新定时任务
 * @param {Object} task - 任务配置
 * @returns {Promise<ApiResult>}
 */
export async function updateCronTask(task: any): Promise<any> {
  return callApi(SystemHandler.UpdateCronTask, task)
}

/**
 * 删除定时任务
 * @param {number} id - 任务ID
 * @returns {Promise<ApiResult>}
 */
export async function deleteCronTask(id: number): Promise<any> {
  return callApi(SystemHandler.DeleteCronTask, id)
}

// ========== MCP 服务 ==========

/**
 * 删除 MCP 服务器
 * @param {number} id - 服务器ID
 * @returns {Promise<ApiResult>}
 */
export async function deleteMcpServer(id: number): Promise<any> {
  return callApi(SystemHandler.DeleteMCPServer, id)
}

/**
 * 测试 MCP 服务器连接
 * @param {number} id - 服务器ID
 * @returns {Promise<ApiResult>}
 */
export async function testMcpServer(id: number): Promise<any> {
  return callApi(SystemHandler.TestMCPServer, id)
}

// ========== 技能管理 ==========

/**
 * 删除技能
 * @param {number} id - 技能ID
 * @returns {Promise<ApiResult>}
 */
export async function deleteSkill(id: number): Promise<any> {
  return callApi(SystemHandler.DeleteSkill, id)
}

/**
 * 创建技能
 * Go: CreateSkill(skill *models.Skill) string
 */
export async function createSkill(skill: any): Promise<any> {
  return callApi(SystemHandler.CreateSkill, skill)
}

/**
 * 更新技能
 * Go: UpdateSkill(skill *models.Skill) string
 */
export async function updateSkill(skill: any): Promise<any> {
  return callApi(SystemHandler.UpdateSkill, skill)
}

// ========== 提示词管理 ==========

/**
 * 获取提示词模板列表
 * Go: GetPromptTemplates(name, promptType string) *[]models.PromptTemplate
 * @param {string} name - 名称关键词
 * @param {string} promptType - 模板类型
 * @returns {Promise<ApiResult>}
 */
export async function getPromptTemplates(name: string = '', promptType: string = ''): Promise<any> {
  return callApi(AnalysisHandler.GetPromptTemplates, name, promptType)
}

/**
 * 添加提示词
 * Go: AddPrompt(prompt models.Prompt) string
 */
export async function addPrompt(prompt: any): Promise<any> {
  return callApi(AnalysisHandler.AddPrompt, prompt)
}

/**
 * 删除提示词
 * Go: DelPrompt(id uint) string
 */
export async function delPrompt(id: number): Promise<any> {
  return callApi(AnalysisHandler.DelPrompt, id)
}

/**
 * 删除提示词模板
 * @param {number} id - 模板ID
 * @returns {Promise<ApiResult>}
 */
export async function deletePromptTemplate(id: number): Promise<any> {
  return callApi(AnalysisHandler.DeletePromptTemplate, id)
}

// ========== 用户手册 ==========

/**
 * 获取用户手册
 * Go: GetUserManual() string
 */
export async function getUserManual(): Promise<any> {
  return callApi(SystemHandler.GetUserManual)
}

// ========== 系统操作 ==========

/**
 * 打开外部链接
 * Go: OpenURL(url string)
 */
export async function openURL(url: string): Promise<any> {
  return callApi(SystemHandler.OpenURL, url)
}

/**
 * 以管理员身份重启应用
 * Go: RestartAsAdmin()
 */
export async function restartAsAdmin(): Promise<any> {
  return callApi(SystemHandler.RestartAsAdmin)
}

// ========== 配置管理 ==========

/**
 * 导出配置
 * Go: ExportConfig() string
 */
export async function exportConfig(): Promise<any> {
  return callApi(SystemHandler.ExportConfig)
}

/**
 * 更新配置
 * Go: UpdateConfig(config any) error
 */
export async function updateConfig(config: any): Promise<any> {
  return callApi(App.UpdateConfig, config)
}

// ========== 多 Agent ==========

/**
 * 获取多 Agent 提示词
 * Go: GetMultiAgentPrompts() []models.MultiAgentPrompt
 */
export async function getMultiAgentPrompts(): Promise<any> {
  return callApi(AnalysisHandler.GetMultiAgentPrompts)
}

/**
 * 更新多 Agent 提示词
 * Go: UpdateMultiAgentPrompt(type, name, content string)
 */
export async function updateMultiAgentPrompt(type: string, name: string, content: string): Promise<any> {
  return callApi(AnalysisHandler.UpdateMultiAgentPrompt, type, name, content)
}

// ========== AI 模型 ==========

/**
 * 获取 AI 模型列表
 * Go: FetchAiModels(baseUrl, apiKey string) []map[string]any
 */
export async function fetchAiModels(baseUrl: string, apiKey: string): Promise<any> {
  return callApi(SystemHandler.FetchAiModels, baseUrl, apiKey)
}

/**
 * 获取 AI 模型详情
 * Go: FetchAiModelInfo(baseUrl, apiKey, model string) map[string]any
 */
export async function fetchAiModelInfo(baseUrl: string, apiKey: string, model: string): Promise<any> {
  return callApi(SystemHandler.FetchAiModelInfo, baseUrl, apiKey, model)
}

// ========== 通知/机器 ==========

/**
 * 发送测试通知
 * Go: SendTestNotification(message string) string
 */
export async function sendTestNotification(message: string): Promise<any> {
  return callApi(NotificationHandler.SendTestNotification, message)
}

/**
 * 获取机器ID
 * Go: GetMachineId() string
 */
export async function getMachineId(): Promise<any> {
  return callApi(SystemHandler.GetMachineId)
}

/**
 * 获取时区
 * Go: GetTimezone() string
 */
export async function getTimezone(): Promise<any> {
  return callApi(SystemHandler.GetTimezone)
}

// ========== AI 推荐 ==========

/**
 * 获取 AI 推荐统计
 * Go: GetAiRecommendStats() map[string]any
 */
export async function getAiRecommendStats(): Promise<any> {
  return callApi(AnalysisHandler.GetAiRecommendStats)
}

/**
 * 获取 AI 推荐股票列表
 * Go: GetAiRecommendStocksList(query data.AiRecommendStockListQuery) data.AiRecommendStockPageData
 */
export async function getAiRecommendStocksList(query: any): Promise<any> {
  return callApi(AnalysisHandler.GetAiRecommendStocksList, query)
}

/**
 * 删除 AI 推荐股票
 * Go: DeleteAiRecommendStocks(id int) string
 */
export async function deleteAiRecommendStocks(id: number): Promise<any> {
  return callApi(AnalysisHandler.DeleteAiRecommendStocks, id)
}

/**
 * 更新 AI 推荐股票预警
 * Go: UpdateAiRecommendStocksAlert(id int, enable bool) string
 */
export async function updateAiRecommendStocksAlert(id: number, enable: boolean): Promise<any> {
  return callApi(AnalysisHandler.UpdateAiRecommendStocksAlert, id, enable)
}

// ========== AI 响应结果管理 ==========

/**
 * 获取 AI 响应结果列表
 * Go: GetAIResponseResultList(query any) any
 */
export async function getAIResponseResultList(query: any): Promise<any> {
  return callApi(AnalysisHandler.GetAIResponseResultList, query)
}

/**
 * 删除 AI 响应结果
 * Go: DeleteAIResponseResult(id int) string
 */
export async function deleteAIResponseResult(id: number): Promise<any> {
  return callApi(AnalysisHandler.DeleteAIResponseResult, id)
}

/**
 * 批量删除 AI 响应结果
 * Go: BatchDeleteAIResponseResult(ids []int) string
 */
export async function batchDeleteAIResponseResult(ids: any): Promise<any> {
  return callApi(AnalysisHandler.BatchDeleteAIResponseResult, ids)
}

// ========== Agent 聊天 ==========

/**
 * 与 Agent 聊天
 * Go: ChatWithAgent(question string, aiConfigId int, sysPromptId *int, memoryMode bool, memoryCount int, thinkingMode bool, agentMode string, sessionID string)
 */
export async function chatWithAgent(question: string, aiConfigId: number, sysPromptId: any, memoryMode: boolean, memoryCount: number, thinkingMode: boolean, agentMode: string, sessionId: string): Promise<any> {
  return callApi(AgentHandler.ChatWithAgent, question, aiConfigId, sysPromptId, memoryMode, memoryCount, thinkingMode, agentMode, sessionId)
}

/**
 * 中止 Agent 聊天
 * Go: AbortChatWithAgent() string
 */
export async function abortChatWithAgent(): Promise<any> {
  return callApi(AgentHandler.AbortChatWithAgent)
}

// ========== AI 助手会话 ==========

/**
 * 保存 AI 助手会话
 * Go: SaveAiAssistantSession(key string, messages []map[string]any)
 */
export async function saveAiAssistantSession(key: string, messages: any): Promise<any> {
  return callApi(SystemHandler.SaveAiAssistantSession, key, messages)
}

/**
 * 获取 AI 助手会话
 * Go: GetAiAssistantSession(key string) []map[string]any
 */
export async function getAiAssistantSession(key: string): Promise<any> {
  return callApi(SystemHandler.GetAiAssistantSession, key)
}

// ========== 分享 ==========

/**
 * 分享文本
 * Go: ShareText(title, content string) string
 */
export async function shareText(title: string, content: string): Promise<any> {
  return callApi(AnalysisHandler.ShareText, title, content)
}

// ========== Cron 定时任务（分页/增强版） ==========

/**
 * 获取定时任务列表（分页）
 * Go: GetCronTaskList(query any) any
 */
export async function getCronTaskList(query: any): Promise<any> {
  return callApi(SystemHandler.GetCronTaskList, query)
}

/**
 * 获取定时任务详情
 * Go: GetCronTaskByID(id int) any
 */
export async function getCronTaskById(id: number): Promise<any> {
  return callApi(SystemHandler.GetCronTaskByID, id)
}

/**
 * 获取定时任务类型
 * Go: GetCronTaskTypes() []map[string]any
 */
export async function getCronTaskTypes(): Promise<any> {
  return callApi(SystemHandler.GetCronTaskTypes)
}

/**
 * 启用/禁用定时任务
 * Go: EnableCronTask(id int, enable bool)
 */
export async function enableCronTask(id: number, enable: boolean): Promise<any> {
  return callApi(SystemHandler.EnableCronTask, id, enable)
}

/**
 * 立即执行定时任务
 * Go: ExecuteCronTaskNow(id int)
 */
export async function executeCronTaskNow(id: number): Promise<any> {
  return callApi(SystemHandler.ExecuteCronTaskNow, id)
}

/**
 * 验证 cron 表达式
 * Go: ValidateCronExpr(expr string) map[string]any
 */
export async function validateCronExpr(expr: string): Promise<any> {
  return callApi(SystemHandler.ValidateCronExpr, expr)
}

/**
 * 搜索定时任务
 * Go: SearchCronTasks(keyword string) any
 */
export async function searchCronTasks(keyword: string): Promise<any> {
  return callApi(SystemHandler.SearchCronTasks, keyword)
}

/**
 * 计算下次运行时间
 * Go: CalculateNextRunTime(expr string) string
 */
export async function calculateNextRunTime(expr: string): Promise<any> {
  return callApi(SystemHandler.CalculateNextRunTime, expr)
}

/**
 * 计算多次运行时间
 * Go: CalculateNextRunTimes(expr string, count int) []string
 */
export async function calculateNextRunTimes(expr: string, count: number): Promise<any> {
  return callApi(SystemHandler.CalculateNextRunTimes, expr, count)
}

// ========== 技能管理（增强版） ==========

/**
 * 获取所有技能
 * Go: GetAllSkills() []models.Skill
 */
export async function getAllSkills(): Promise<any> {
  return callApi(SystemHandler.GetAllSkills)
}

/**
 * 获取技能列表（分页）
 * Go: GetSkillList(query any) any
 */
export async function getSkillList(query: any): Promise<any> {
  return callApi(SystemHandler.GetSkillList, query)
}

/**
 * 获取技能详情
 * Go: GetSkillByID(id int) models.Skill
 */
export async function getSkillById(id: number): Promise<any> {
  return callApi(SystemHandler.GetSkillByID, id)
}

/**
 * 启用/禁用技能
 * Go: EnableSkill(id int, enable bool)
 */
export async function enableSkill(id: number, enable: boolean): Promise<any> {
  return callApi(SystemHandler.EnableSkill, id, enable)
}

/**
 * 从URL生成技能
 * Go: GenerateSkillFromURL(url string) models.Skill
 */
export async function generateSkillFromURL(url: string): Promise<any> {
  return callApi(App.GenerateSkillFromURL, url)
}

// ========== MCP 服务（增强版） ==========

/**
 * 获取 MCP 服务器列表（分页）
 * Go: GetMCPServerList(query any) any
 */
export async function getMCPServerList(query: any): Promise<any> {
  return callApi(SystemHandler.GetMCPServerList, query)
}

/**
 * 获取 MCP 服务器详情
 * Go: GetMCPServerByID(id int) models.MCPServer
 */
export async function getMCPServerById(id: number): Promise<any> {
  return callApi(SystemHandler.GetMCPServerByID, id)
}

/**
 * 启用/禁用 MCP 服务器
 * Go: EnableMCPServer(id int, enable bool)
 */
export async function enableMCPServer(id: number, enable: boolean): Promise<any> {
  return callApi(SystemHandler.EnableMCPServer, id, enable)
}

/**
 * 创建 MCP 服务器
 * Go: CreateMCPServer(server any)
 */
export async function createMCPServer(server: any): Promise<any> {
  return callApi(SystemHandler.CreateMCPServer, server)
}

/**
 * 更新 MCP 服务器
 * Go: UpdateMCPServer(server any)
 */
export async function updateMCPServer(server: any): Promise<any> {
  return callApi(SystemHandler.UpdateMCPServer, server)
}

/**
 * 获取服务器下的 MCP 工具
 * Go: GetMCPToolsByServerID(id int) []map[string]any
 */
export async function getMCPToolsByServerID(id: number): Promise<any> {
  return callApi(SystemHandler.GetMCPToolsByServerID, id)
}

/**
 * 获取所有 MCP 工具
 * Go: GetAllMCPTools() []map[string]any
 */
export async function getAllMCPTools(): Promise<any> {
  return callApi(SystemHandler.GetAllMCPTools)
}

// ========== 提示词管理（增强版） ==========

/**
 * 获取提示词模板列表（分页）
 * Go: GetPromptTemplateList(query any) any
 */
export async function getPromptTemplateList(query: any): Promise<any> {
  return callApi(AnalysisHandler.GetPromptTemplateList, query)
}

/**
 * 添加提示词模板
 * Go: AddPromptTemplate(template models.PromptTemplate)
 */
export async function addPromptTemplate(template: any): Promise<any> {
  return callApi(AnalysisHandler.AddPromptTemplate, template)
}

/**
 * 更新提示词模板
 * Go: UpdatePromptTemplate(template models.PromptTemplate)
 */
export async function updatePromptTemplate(template: any): Promise<any> {
  return callApi(AnalysisHandler.UpdatePromptTemplate, template)
}

export default {
  // 版本信息
  getVersionInfo,
  checkUpdate,

  // 设置相关
  getConfig,

  // AI 分析结果保存
  saveAiResponseResult,
  saveAsMarkdown,
  shareAnalysis,

  // AI 配置
  getAllStrategies,
  getAiConfigs,

  // Cron 定时任务
  createCronTask,
  updateCronTask,
  deleteCronTask,
  // Cron（增强版）
  getCronTaskList,
  getCronTaskById,
  getCronTaskTypes,
  enableCronTask,
  executeCronTaskNow,
  validateCronExpr,
  searchCronTasks,
  calculateNextRunTime,
  calculateNextRunTimes,

  // MCP 服务
  deleteMcpServer,
  testMcpServer,
  // MCP（增强版）
  getMCPServerList,
  getMCPServerById,
  enableMCPServer,
  createMCPServer,
  updateMCPServer,
  getMCPToolsByServerID,
  getAllMCPTools,

  // 技能管理
  deleteSkill,
  // 技能（增强版）
  createSkill,
  updateSkill,
  getAllSkills,
  getSkillList,
  getSkillById,
  enableSkill,
  generateSkillFromURL,

  // 提示词管理
  addPrompt,
  delPrompt,
  getPromptTemplates,
  deletePromptTemplate,
  // 提示词（增强版）
  getPromptTemplateList,
  addPromptTemplate,
  updatePromptTemplate,



  // 用户手册
  getUserManual,

  // 系统操作
  openURL,
  restartAsAdmin,

  // 配置管理
  exportConfig,
  updateConfig,

  // 多 Agent
  getMultiAgentPrompts,
  updateMultiAgentPrompt,

  // AI 模型
  fetchAiModels,
  fetchAiModelInfo,

  // 通知/机器
  sendTestNotification,
  getMachineId,
  getTimezone,

  // AI 推荐
  getAiRecommendStats,
  getAiRecommendStocksList,
  deleteAiRecommendStocks,
  updateAiRecommendStocksAlert,

  // AI 响应结果管理
  getAIResponseResultList,
  deleteAIResponseResult,
  batchDeleteAIResponseResult,

  // Agent 聊天
  chatWithAgent,
  abortChatWithAgent,

  // AI 助手会话
  saveAiAssistantSession,
  getAiAssistantSession,

  // 分享
  shareText,
}
