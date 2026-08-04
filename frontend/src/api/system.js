/**
 * 系统相关 API
 * 封装设置、配置、版本等系统相关调用
 */

import { callApi } from './client'
import * as App from '../../wailsjs/go/main/App'

// ========== 版本信息 ==========

/**
 * 获取版本信息
 * @returns {Promise<ApiResult>}
 */
export async function getVersionInfo() {
  return callApi(App.GetVersionInfo)
}

/**
 * 检查更新
 * @returns {Promise<ApiResult>}
 */
export async function checkUpdate() {
  return callApi(App.CheckUpdate)
}

// ========== 设置相关 ==========

/**
 * 获取设置配置 (GetSettingConfig - 应用设置页面的配置)
 * @returns {Promise<ApiResult>}
 */
export async function getSettings() {
  return callApi(App.GetSettingConfig)
}

/**
 * 获取运行时配置 (GetConfig - 功能开关如 enableFund/enableAgent/darkTheme)
 * 注意：与 getSettings 不同，GetConfig 返回的是功能开关配置
 * @returns {Promise<ApiResult>}
 */
export async function getConfig() {
  return callApi(App.GetConfig)
}

/**
 * 保存设置
 * @param {Object} settings - 设置对象
 * @returns {Promise<ApiResult>}
 */
export async function saveSettings(settings) {
  return callApi(App.SaveSettingConfig, settings)
}

/**
 * 重置设置
 * @returns {Promise<ApiResult>}
 */
export async function resetSettings() {
  return callApi(App.ResetSettingConfig)
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
export async function saveAiResponseResult(arg1, arg2, arg3, arg4, arg5, arg6) {
  return callApi(App.SaveAIResponseResult, arg1, arg2, arg3, arg4, arg5, arg6)
}

/**
 * 保存为 Markdown 文件
 * @param {string} title
 * @param {string} content
 * @returns {Promise<ApiResult>}
 */
export async function saveAsMarkdown(title, content) {
  return callApi(App.SaveAsMarkdown, title, content)
}

/**
 * 分享分析结果
 * @param {string} arg1
 * @param {string} arg2
 * @returns {Promise<ApiResult>}
 */
export async function shareAnalysis(arg1, arg2) {
  return callApi(App.ShareAnalysis, arg1, arg2)
}

// ========== AI 配置 ==========

/**
 * 获取所有策略
 * Go: GetAllStrategies() []*strategy.Strategy
 * @returns {Promise<ApiResult>}
 */
export async function getAllStrategies() {
  return callApi(App.GetAllStrategies)
}

/**
 * 获取 AI 配置列表
 * @returns {Promise<ApiResult>}
 */
export async function getAiConfigs() {
  return callApi(App.GetAiConfigs)
}

/**
 * 保存 AI 配置
 * @param {Object} config - AI 配置对象
 * @returns {Promise<ApiResult>}
 */
export async function saveAiConfig(config) {
  return callApi(App.SaveAiConfig, config)
}

/**
 * 删除 AI 配置
 * @param {number} id - 配置ID
 * @returns {Promise<ApiResult>}
 */
export async function deleteAiConfig(id) {
  return callApi(App.DeleteAiConfig, id)
}

/**
 * 设置活跃 AI 配置
 * @param {number} id - 配置ID
 * @returns {Promise<ApiResult>}
 */
export async function setActiveAiConfig(id) {
  return callApi(App.SetActiveAiConfig, id)
}

// ========== Cron 定时任务 ==========

/**
 * 获取定时任务列表
 * @returns {Promise<ApiResult>}
 */
export async function getCronTasks() {
  return callApi(App.GetCronTasks)
}

/**
 * 创建定时任务
 * @param {Object} task - 任务配置
 * @returns {Promise<ApiResult>}
 */
export async function createCronTask(task) {
  return callApi(App.CreateCronTask, task)
}

/**
 * 更新定时任务
 * @param {Object} task - 任务配置
 * @returns {Promise<ApiResult>}
 */
export async function updateCronTask(task) {
  return callApi(App.UpdateCronTask, task)
}

/**
 * 删除定时任务
 * @param {number} id - 任务ID
 * @returns {Promise<ApiResult>}
 */
export async function deleteCronTask(id) {
  return callApi(App.DeleteCronTask, id)
}

/**
 * 切换定时任务启用状态
 * @param {number} id - 任务ID
 * @param {boolean} enable - 是否启用
 * @returns {Promise<ApiResult>}
 */
export async function toggleCronTask(id, enable) {
  return callApi(App.ToggleCronTask, id, enable)
}

/**
 * 手动运行定时任务
 * @param {number} id - 任务ID
 * @returns {Promise<ApiResult>}
 */
export async function runCronTask(id) {
  return callApi(App.RunCronTask, id)
}

// ========== MCP 服务 ==========

/**
 * 获取 MCP 服务器列表
 * @returns {Promise<ApiResult>}
 */
export async function getMcpServers() {
  return callApi(App.GetMcpServers)
}

/**
 * 保存 MCP 服务器配置
 * @param {Object} server - 服务器配置
 * @returns {Promise<ApiResult>}
 */
export async function saveMcpServer(server) {
  return callApi(App.SaveMcpServer, server)
}

/**
 * 删除 MCP 服务器
 * @param {number} id - 服务器ID
 * @returns {Promise<ApiResult>}
 */
export async function deleteMcpServer(id) {
  return callApi(App.DeleteMcpServer, id)
}

/**
 * 测试 MCP 服务器连接
 * @param {number} id - 服务器ID
 * @returns {Promise<ApiResult>}
 */
export async function testMcpServer(id) {
  return callApi(App.TestMcpServer, id)
}

// ========== 技能管理 ==========

/**
 * 获取技能列表
 * @returns {Promise<ApiResult>}
 */
export async function getSkills() {
  return callApi(App.GetSkills)
}

/**
 * 保存技能
 * @param {Object} skill - 技能配置
 * @returns {Promise<ApiResult>}
 */
export async function saveSkill(skill) {
  return callApi(App.SaveSkill, skill)
}

/**
 * 删除技能
 * @param {number} id - 技能ID
 * @returns {Promise<ApiResult>}
 */
export async function deleteSkill(id) {
  return callApi(App.DeleteSkill, id)
}

/**
 * 创建技能
 * Go: CreateSkill(skill *models.Skill) string
 */
export async function createSkill(skill) {
  return callApi(App.CreateSkill, skill)
}

/**
 * 更新技能
 * Go: UpdateSkill(skill *models.Skill) string
 */
export async function updateSkill(skill) {
  return callApi(App.UpdateSkill, skill)
}

// ========== 提示词管理 ==========

/**
 * 获取提示词模板列表
 * Go: GetPromptTemplates(name, promptType string) *[]models.PromptTemplate
 * @param {string} name - 名称关键词
 * @param {string} promptType - 模板类型
 * @returns {Promise<ApiResult>}
 */
export async function getPromptTemplates(name = '', promptType = '') {
  return callApi(App.GetPromptTemplates, name, promptType)
}

/**
 * 添加提示词
 * Go: AddPrompt(prompt models.Prompt) string
 */
export async function addPrompt(prompt) {
  return callApi(App.AddPrompt, prompt)
}

/**
 * 删除提示词
 * Go: DelPrompt(id uint) string
 */
export async function delPrompt(id) {
  return callApi(App.DelPrompt, id)
}

/**
 * 保存提示词模板
 * @param {Object} template - 模板配置
 * @returns {Promise<ApiResult>}
 */
export async function savePromptTemplate(template) {
  return callApi(App.SavePromptTemplate, template)
}

/**
 * 删除提示词模板
 * @param {number} id - 模板ID
 * @returns {Promise<ApiResult>}
 */
export async function deletePromptTemplate(id) {
  return callApi(App.DeletePromptTemplate, id)
}

// ========== 日志 ==========

/**
 * 获取应用日志
 * @param {number} lines - 获取行数
 * @returns {Promise<ApiResult>}
 */
export async function getAppLogs(lines = 500) {
  return callApi(App.GetAppLogs, lines)
}

/**
 * 清理日志
 * @returns {Promise<ApiResult>}
 */
export async function clearLogs() {
  return callApi(App.ClearLogs)
}

// ========== 数据管理 ==========

/**
 * 导出数据
 * @param {string} type - 数据类型
 * @returns {Promise<ApiResult>}
 */
export async function exportData(type = 'all') {
  return callApi(App.ExportData, type)
}

/**
 * 导入数据
 * @param {File} file - 文件
 * @returns {Promise<ApiResult>}
 */
export async function importData(file) {
  return callApi(App.ImportData, file)
}

/**
 * 清理缓存
 * @returns {Promise<ApiResult>}
 */
export async function clearCache() {
  return callApi(App.ClearCache)
}

// ========== 赞助/用户 ==========

/**
 * 获取赞助信息
 * Go: GetSponsorInfo() map[string]any
 */
export async function getSponsorInfo() {
  return callApi(App.GetSponsorInfo)
}

/**
 * 获取用户手册
 * Go: GetUserManual() string
 */
export async function getUserManual() {
  return callApi(App.GetUserManual)
}

/**
 * 检查赞助码
 * Go: CheckSponsorCode(code string) map[string]any
 */
export async function checkSponsorCode(code) {
  return callApi(App.CheckSponsorCode, code)
}

// ========== 配置管理 ==========

/**
 * 导出配置
 * Go: ExportConfig() string
 */
export async function exportConfig() {
  return callApi(App.ExportConfig)
}

/**
 * 更新配置
 * Go: UpdateConfig(config any) error
 */
export async function updateConfig(config) {
  return callApi(App.UpdateConfig, config)
}

// ========== 多 Agent ==========

/**
 * 获取多 Agent 提示词
 * Go: GetMultiAgentPrompts() []models.MultiAgentPrompt
 */
export async function getMultiAgentPrompts() {
  return callApi(App.GetMultiAgentPrompts)
}

/**
 * 更新多 Agent 提示词
 * Go: UpdateMultiAgentPrompt(type, name, content string)
 */
export async function updateMultiAgentPrompt(type, name, content) {
  return callApi(App.UpdateMultiAgentPrompt, type, name, content)
}

// ========== AI 模型 ==========

/**
 * 获取 AI 模型列表
 * Go: FetchAiModels(baseUrl, apiKey string) []map[string]any
 */
export async function fetchAiModels(baseUrl, apiKey) {
  return callApi(App.FetchAiModels, baseUrl, apiKey)
}

/**
 * 获取 AI 模型详情
 * Go: FetchAiModelInfo(baseUrl, apiKey, model string) map[string]any
 */
export async function fetchAiModelInfo(baseUrl, apiKey, model) {
  return callApi(App.FetchAiModelInfo, baseUrl, apiKey, model)
}

// ========== 通知/机器 ==========

/**
 * 发送测试通知
 * Go: SendTestNotification(message string) string
 */
export async function sendTestNotification(message) {
  return callApi(App.SendTestNotification, message)
}

/**
 * 获取机器ID
 * Go: GetMachineId() string
 */
export async function getMachineId() {
  return callApi(App.GetMachineId)
}

/**
 * 获取时区
 * Go: GetTimezone() string
 */
export async function getTimezone() {
  return callApi(App.GetTimezone)
}

// ========== AI 推荐 ==========

/**
 * 获取 AI 推荐统计
 * Go: GetAiRecommendStats() map[string]any
 */
export async function getAiRecommendStats() {
  return callApi(App.GetAiRecommendStats)
}

/**
 * 获取 AI 推荐股票列表
 * Go: GetAiRecommendStocksList(query data.AiRecommendStockListQuery) data.AiRecommendStockPageData
 */
export async function getAiRecommendStocksList(query) {
  return callApi(App.GetAiRecommendStocksList, query)
}

/**
 * 删除 AI 推荐股票
 * Go: DeleteAiRecommendStocks(id int) string
 */
export async function deleteAiRecommendStocks(id) {
  return callApi(App.DeleteAiRecommendStocks, id)
}

/**
 * 更新 AI 推荐股票预警
 * Go: UpdateAiRecommendStocksAlert(id int, enable bool) string
 */
export async function updateAiRecommendStocksAlert(id, enable) {
  return callApi(App.UpdateAiRecommendStocksAlert, id, enable)
}

// ========== AI 响应结果管理 ==========

/**
 * 获取 AI 响应结果列表
 * Go: GetAIResponseResultList(query any) any
 */
export async function getAIResponseResultList(query) {
  return callApi(App.GetAIResponseResultList, query)
}

/**
 * 删除 AI 响应结果
 * Go: DeleteAIResponseResult(id int) string
 */
export async function deleteAIResponseResult(id) {
  return callApi(App.DeleteAIResponseResult, id)
}

/**
 * 批量删除 AI 响应结果
 * Go: BatchDeleteAIResponseResult(ids []int) string
 */
export async function batchDeleteAIResponseResult(ids) {
  return callApi(App.BatchDeleteAIResponseResult, ids)
}

// ========== Agent 聊天 ==========

/**
 * 与 Agent 聊天
 * Go: ChatWithAgent(agentType, question string, context []any, stream bool, aiConfigId int, sysPromptId *int, enableTools bool)
 */
export async function chatWithAgent(agentType, question, context, stream, aiConfigId, sysPromptId, enableTools) {
  return callApi(App.ChatWithAgent, agentType, question, context, stream, aiConfigId, sysPromptId, enableTools)
}

/**
 * 中止 Agent 聊天
 * Go: AbortChatWithAgent() string
 */
export async function abortChatWithAgent() {
  return callApi(App.AbortChatWithAgent)
}

// ========== AI 助手会话 ==========

/**
 * 保存 AI 助手会话
 * Go: SaveAiAssistantSession(key string, messages []map[string]any)
 */
export async function saveAiAssistantSession(key, messages) {
  return callApi(App.SaveAiAssistantSession, key, messages)
}

/**
 * 获取 AI 助手会话
 * Go: GetAiAssistantSession(key string) []map[string]any
 */
export async function getAiAssistantSession(key) {
  return callApi(App.GetAiAssistantSession, key)
}

// ========== 分享 ==========

/**
 * 分享文本
 * Go: ShareText(title, content string) string
 */
export async function shareText(title, content) {
  return callApi(App.ShareText, title, content)
}

// ========== Cron 定时任务（分页/增强版） ==========

/**
 * 获取定时任务列表（分页）
 * Go: GetCronTaskList(query any) any
 */
export async function getCronTaskList(query) {
  return callApi(App.GetCronTaskList, query)
}

/**
 * 获取定时任务详情
 * Go: GetCronTaskById(id int) any
 */
export async function getCronTaskById(id) {
  return callApi(App.GetCronTaskById, id)
}

/**
 * 获取定时任务类型
 * Go: GetCronTaskTypes() []map[string]any
 */
export async function getCronTaskTypes() {
  return callApi(App.GetCronTaskTypes)
}

/**
 * 启用/禁用定时任务
 * Go: EnableCronTask(id int, enable bool)
 */
export async function enableCronTask(id, enable) {
  return callApi(App.EnableCronTask, id, enable)
}

/**
 * 立即执行定时任务
 * Go: ExecuteCronTaskNow(id int)
 */
export async function executeCronTaskNow(id) {
  return callApi(App.ExecuteCronTaskNow, id)
}

/**
 * 验证 cron 表达式
 * Go: ValidateCronExpr(expr string) map[string]any
 */
export async function validateCronExpr(expr) {
  return callApi(App.ValidateCronExpr, expr)
}

/**
 * 搜索定时任务
 * Go: SearchCronTasks(keyword string) any
 */
export async function searchCronTasks(keyword) {
  return callApi(App.SearchCronTasks, keyword)
}

/**
 * 计算下次运行时间
 * Go: CalculateNextRunTime(expr string) string
 */
export async function calculateNextRunTime(expr) {
  return callApi(App.CalculateNextRunTime, expr)
}

/**
 * 计算多次运行时间
 * Go: CalculateNextRunTimes(expr string, count int) []string
 */
export async function calculateNextRunTimes(expr, count) {
  return callApi(App.CalculateNextRunTimes, expr, count)
}

// ========== 技能管理（增强版） ==========

/**
 * 获取所有技能
 * Go: GetAllSkills() []models.Skill
 */
export async function getAllSkills() {
  return callApi(App.GetAllSkills)
}

/**
 * 获取技能列表（分页）
 * Go: GetSkillList(query any) any
 */
export async function getSkillList(query) {
  return callApi(App.GetSkillList, query)
}

/**
 * 获取技能详情
 * Go: GetSkillById(id int) models.Skill
 */
export async function getSkillById(id) {
  return callApi(App.GetSkillById, id)
}

/**
 * 启用/禁用技能
 * Go: EnableSkill(id int, enable bool)
 */
export async function enableSkill(id, enable) {
  return callApi(App.EnableSkill, id, enable)
}

/**
 * 从URL生成技能
 * Go: GenerateSkillFromURL(url string) models.Skill
 */
export async function generateSkillFromURL(url) {
  return callApi(App.GenerateSkillFromURL, url)
}

// ========== MCP 服务（增强版） ==========

/**
 * 获取 MCP 服务器列表（分页）
 * Go: GetMCPServerList(query any) any
 */
export async function getMCPServerList(query) {
  return callApi(App.GetMCPServerList, query)
}

/**
 * 获取 MCP 服务器详情
 * Go: GetMCPServerByID(id int) models.MCPServer
 */
export async function getMCPServerById(id) {
  return callApi(App.GetMCPServerByID, id)
}

/**
 * 启用/禁用 MCP 服务器
 * Go: EnableMCPServer(id int, enable bool)
 */
export async function enableMCPServer(id, enable) {
  return callApi(App.EnableMCPServer, id, enable)
}

/**
 * 创建 MCP 服务器
 * Go: CreateMCPServer(server any)
 */
export async function createMCPServer(server) {
  return callApi(App.CreateMCPServer, server)
}

/**
 * 更新 MCP 服务器
 * Go: UpdateMCPServer(server any)
 */
export async function updateMCPServer(server) {
  return callApi(App.UpdateMCPServer, server)
}

/**
 * 获取服务器下的 MCP 工具
 * Go: GetMCPToolsByServerID(id int) []map[string]any
 */
export async function getMCPToolsByServerID(id) {
  return callApi(App.GetMCPToolsByServerID, id)
}

/**
 * 获取所有 MCP 工具
 * Go: GetAllMCPTools() []map[string]any
 */
export async function getAllMCPTools() {
  return callApi(App.GetAllMCPTools)
}

// ========== 提示词管理（增强版） ==========

/**
 * 获取提示词模板列表（分页）
 * Go: GetPromptTemplateList(query any) any
 */
export async function getPromptTemplateList(query) {
  return callApi(App.GetPromptTemplateList, query)
}

/**
 * 添加提示词模板
 * Go: AddPromptTemplate(template models.PromptTemplate)
 */
export async function addPromptTemplate(template) {
  return callApi(App.AddPromptTemplate, template)
}

/**
 * 更新提示词模板
 * Go: UpdatePromptTemplate(template models.PromptTemplate)
 */
export async function updatePromptTemplate(template) {
  return callApi(App.UpdatePromptTemplate, template)
}

export default {
  // 版本信息
  getVersionInfo,
  checkUpdate,

  // 设置相关
  getSettings,
  getConfig,
  saveSettings,
  resetSettings,

  // AI 分析结果保存
  saveAiResponseResult,
  saveAsMarkdown,
  shareAnalysis,

  // AI 配置
  getAllStrategies,
  getAiConfigs,
  saveAiConfig,
  deleteAiConfig,
  setActiveAiConfig,

  // Cron 定时任务
  getCronTasks,
  createCronTask,
  updateCronTask,
  deleteCronTask,
  toggleCronTask,
  runCronTask,
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
  getMcpServers,
  saveMcpServer,
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
  getSkills,
  saveSkill,
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
  savePromptTemplate,
  deletePromptTemplate,
  // 提示词（增强版）
  getPromptTemplateList,
  addPromptTemplate,
  updatePromptTemplate,

  // 日志
  getAppLogs,
  clearLogs,

  // 数据管理
  exportData,
  importData,
  clearCache,

  // 赞助/用户
  getSponsorInfo,
  getUserManual,
  checkSponsorCode,

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
