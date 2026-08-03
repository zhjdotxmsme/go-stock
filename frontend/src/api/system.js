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
 * 获取设置配置
 * @returns {Promise<ApiResult>}
 */
export async function getSettings() {
  return callApi(App.GetSettingConfig)
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

// ========== AI 配置 ==========

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

// ========== 提示词管理 ==========

/**
 * 获取提示词模板列表
 * @returns {Promise<ApiResult>}
 */
export async function getPromptTemplates() {
  return callApi(App.GetPromptTemplates)
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

export default {
  // 版本信息
  getVersionInfo,
  checkUpdate,

  // 设置相关
  getSettings,
  saveSettings,
  resetSettings,

  // AI 配置
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

  // MCP 服务
  getMcpServers,
  saveMcpServer,
  deleteMcpServer,
  testMcpServer,

  // 技能管理
  getSkills,
  saveSkill,
  deleteSkill,

  // 提示词管理
  getPromptTemplates,
  savePromptTemplate,
  deletePromptTemplate,

  // 日志
  getAppLogs,
  clearLogs,

  // 数据管理
  exportData,
  importData,
  clearCache,
}
