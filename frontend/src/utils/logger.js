/**
 * 统一日志工具
 * 提供分级日志功能，便于调试和问题追踪
 */

// 日志级别
const LOG_LEVELS = {
  DEBUG: 0,
  INFO: 1,
  WARN: 2,
  ERROR: 3,
  NONE: 4,
}

// 当前日志级别
let currentLevel = LOG_LEVELS.DEBUG

// 日志前缀
const PREFIX = '[go-stock]'

/**
 * 设置日志级别
 * @param {number} level - 日志级别
 */
export function setLogLevel(level) {
  currentLevel = level
}

/**
 * 获取当前日志级别
 * @returns {number}
 */
export function getLogLevel() {
  return currentLevel
}

/**
 * 调试日志
 * @param {string} message - 日志消息
 * @param {...*} args - 附加参数
 */
export function debug(message, ...args) {
  if (currentLevel <= LOG_LEVELS.DEBUG) {
    console.debug(`${PREFIX} [DEBUG] ${message}`, ...args)
  }
}

/**
 * 信息日志
 * @param {string} message - 日志消息
 * @param {...*} args - 附加参数
 */
export function info(message, ...args) {
  if (currentLevel <= LOG_LEVELS.INFO) {
    console.info(`${PREFIX} [INFO] ${message}`, ...args)
  }
}

/**
 * 警告日志
 * @param {string} message - 日志消息
 * @param {...*} args - 附加参数
 */
export function warn(message, ...args) {
  if (currentLevel <= LOG_LEVELS.WARN) {
    console.warn(`${PREFIX} [WARN] ${message}`, ...args)
  }
}

/**
 * 错误日志
 * @param {string} message - 日志消息
 * @param {...*} args - 附加参数
 */
export function error(message, ...args) {
  if (currentLevel <= LOG_LEVELS.ERROR) {
    console.error(`${PREFIX} [ERROR] ${message}`, ...args)
  }
}

/**
 * 带时间戳的性能日志
 * @param {string} label - 性能标签
 */
export function time(label) {
  if (currentLevel <= LOG_LEVELS.DEBUG) {
    console.time(`${PREFIX} [PERF] ${label}`)
  }
}

/**
 * 结束性能计时
 * @param {string} label - 性能标签
 */
export function timeEnd(label) {
  if (currentLevel <= LOG_LEVELS.DEBUG) {
    console.timeEnd(`${PREFIX} [PERF] ${label}`)
  }
}

export const logger = {
  setLogLevel,
  getLogLevel,
  debug,
  info,
  warn,
  error,
  time,
  timeEnd,
  levels: LOG_LEVELS,
}

export default logger
