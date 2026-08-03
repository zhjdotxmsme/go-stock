/**
 * Stock code normalization utilities
 *
 * Internal canonical format:
 * - A-share (Shanghai): sh600519
 * - A-share (Shenzhen): sz000001
 * - A-share (Beijing): bj430047
 * - Hong Kong: hk00700
 * - US: usAAPL
 *
 * Supported input formats:
 * - Pure digits: 600519
 * - Tushare/EM: 600519.SH, 000001.SZ, 00700.HK, AAPL.US
 * - Sina/TC prefix: sh600519, sz000001, hk00700
 * - US variants: usAAPL, gb_AAPL
 */

/**
 * Normalize any stock code to internal canonical format
 * @param {string} code - Stock code in any format
 * @returns {string} Normalized stock code
 */
export function normalizeStockCode(code) {
  if (!code || typeof code !== 'string') {
    return ''
  }

  const trimmed = code.trim()
  if (!trimmed) {
    return ''
  }

  // Already internal format (lowercase prefix)
  if (/^[a-z]{2}[A-Z0-9]/.test(trimmed) && !trimmed.includes('_')) {
    return trimmed
  }

  // Sina US variant: gb_AAPL
  if (/^gb_/i.test(trimmed)) {
    return 'us' + trimmed.substring(3)
  }

  // Tushare format: 600519.SH, 000001.SZ, 00700.HK, AAPL.US
  const dotIndex = trimmed.indexOf('.')
  if (dotIndex > 0) {
    const suffix = trimmed.substring(dotIndex + 1).toUpperCase()
    const prefix = suffixToInternal(suffix)
    if (prefix) {
      return prefix + trimmed.substring(0, dotIndex)
    }
  }

  // Prefix format (case insensitive: SH600519, hk00700, usAAPL)
  if (/^[A-Za-z]{2}[0-9A-Z]/.test(trimmed)) {
    return trimmed.substring(0, 2).toLowerCase() + trimmed.substring(2)
  }

  // Pure digits - guess by first digit pattern
  if (/^\d+$/.test(trimmed)) {
    const prefix = guessPrefixFromDigits(trimmed)
    if (prefix) {
      return prefix + trimmed
    }
  }

  // Pure letters - assume US ticker
  if (/^[A-Za-z]+$/.test(trimmed)) {
    return 'us' + trimmed.toUpperCase()
  }

  return trimmed
}

/**
 * Convert normalized stock code to Tushare format
 * @param {string} code - Normalized stock code
 * @returns {string} Tushare format (600519.SH, AAPL.US, etc.)
 */
export function toTushareCode(code) {
  const normalized = normalizeStockCode(code)
  if (!normalized) {
    return code
  }

  const prefix = normalized.substring(0, 2)
  const rest = normalized.substring(2)

  switch (prefix) {
    case 'sh':
      return rest + '.SH'
    case 'sz':
      return rest + '.SZ'
    case 'bj':
      return rest + '.BJ'
    case 'hk':
      return rest + '.HK'
    case 'us':
      return rest // US tickers don't have suffix in many tushare APIs
    default:
      return code
  }
}

/**
 * Convert normalized stock code to EastMoney format
 * @param {string} code - Normalized stock code
 * @returns {string} EastMoney format (1.600519, 0.000001, 128.00700)
 */
export function toEastMoneyCode(code) {
  const normalized = normalizeStockCode(code)
  if (!normalized) {
    return code
  }

  const prefix = normalized.substring(0, 2)
  const rest = normalized.substring(2)

  switch (prefix) {
    case 'sh':
      return '1.' + rest
    case 'sz':
    case 'bj':
      return '0.' + rest
    case 'hk':
      return '128.' + rest
    default:
      return code // US not supported on EastMoney K-line
  }
}

/**
 * Convert normalized stock code to Sina/Tencent format
 * @param {string} code - Normalized stock code
 * @returns {string} Sina/Tencent format (sh600519, hk00700, usAAPL)
 */
export function toSinaCode(code) {
  const normalized = normalizeStockCode(code)
  if (!normalized) {
    return code
  }
  return normalized // Already matches for most codes
}

/**
 * Get market identifier from stock code
 * @param {string} code - Stock code
 * @returns {string} Market: SH, SZ, BJ, HK, US, or empty
 */
export function getMarket(code) {
  const normalized = normalizeStockCode(code)
  if (!normalized) {
    return ''
  }

  const prefix = normalized.substring(0, 2)
  switch (prefix) {
    case 'sh':
      return 'SH'
    case 'sz':
      return 'SZ'
    case 'bj':
      return 'BJ'
    case 'hk':
      return 'HK'
    case 'us':
      return 'US'
    default:
      return ''
  }
}

/**
 * Check if code is A-share (Shanghai/Shenzhen/Beijing)
 * @param {string} code - Stock code
 * @returns {boolean} True if A-share
 */
export function isAStock(code) {
  const m = getMarket(code)
  return m === 'SH' || m === 'SZ' || m === 'BJ'
}

/**
 * Check if code is Hong Kong stock
 * @param {string} code - Stock code
 * @returns {boolean} True if HK stock
 */
export function isHKStock(code) {
  return getMarket(code) === 'HK'
}

/**
 * Check if code is US stock
 * @param {string} code - Stock code
 * @returns {boolean} True if US stock
 */
export function isUSStock(code) {
  return getMarket(code) === 'US'
}

/**
 * Extract just the pure code part (no prefix/suffix)
 * @param {string} code - Stock code in any format
 * @returns {string} Pure code
 */
export function getPureCode(code) {
  const normalized = normalizeStockCode(code)
  if (!normalized) {
    return code
  }
  return normalized.substring(2)
}

// --- Internal helpers ---

/**
 * Map tushare suffix to internal prefix
 */
function suffixToInternal(suffix) {
  switch (suffix.toUpperCase()) {
    case 'SH':
      return 'sh'
    case 'SZ':
      return 'sz'
    case 'BJ':
      return 'bj'
    case 'HK':
      return 'hk'
    case 'US':
      return 'us'
    default:
      return ''
  }
}

/**
 * Guess market prefix from digit-only code
 */
function guessPrefixFromDigits(code) {
  if (code.length < 6) {
    return null // Too short for A-share
  }

  const firstDigit = code.charAt(0)
  switch (firstDigit) {
    case '6':
      return 'sh'
    case '0':
    case '3':
      return 'sz'
    case '4':
    case '8':
    case '9':
      return 'bj'
    default:
      return null
  }
}
