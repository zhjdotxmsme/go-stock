import test from 'node:test'
import assert from 'node:assert/strict'
import { observationLabel, huntingStatusType, backtestSampleWarning } from '../src/components/khunter/presentation.ts'

test('观察与确认状态不会混为买入推荐', () => {
  assert.equal(observationLabel({ status: '观察中', dataStatus: 'intraday' }), '盘中观察')
  assert.equal(observationLabel({ status: '观察中', dataStatus: 'invalid' }), '数据无效')
  assert.equal(observationLabel({ status: '已过期', dataStatus: 'valid' }), '观察过期')
})

test('历史待核不显示为持仓中', () => {
  assert.equal(huntingStatusType('历史待核'), 'default')
  assert.equal(huntingStatusType('持仓中'), 'success')
  assert.equal(huntingStatusType('已退出'), 'info')
})

test('样本外不足100笔必须显示警告', () => {
  assert.match(backtestSampleWarning({ confirmedCount: 99 }), /样本不足/)
  assert.equal(backtestSampleWarning({ confirmedCount: 100 }), '')
})
