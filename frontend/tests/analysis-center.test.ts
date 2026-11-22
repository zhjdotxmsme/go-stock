import test from 'node:test'
import assert from 'node:assert/strict'
import {
  blendedScore, tabFromQuery, rowSubtitle, errText,
  type SubtitleInput,
} from '../src/components/analysis/recommend.ts'

test('有因子按权重混分，无因子保持原分（与后端 round2 同口径）', () => {
  assert.equal(blendedScore(80, 60, 0.05), 79)
  assert.equal(blendedScore(70, 50, 0), 70)
  assert.equal(blendedScore(66.6, 60, 0.05), 66.27)
})

test('未知 tab 回落到推荐，合法 tab 原样返回', () => {
  assert.equal(tabFromQuery('khunter'), 'khunter')
  assert.equal(tabFromQuery('nope'), 'recommend')
  assert.equal(tabFromQuery(undefined), 'recommend')
})

test('降级行必须显示原因而不是当成有因子', () => {
  assert.match(
    rowSubtitle({ hasFactor: false, degraded: true, degradedReason: 'KRONOS_OFFLINE' } satisfies SubtitleInput),
    /KRONOS_OFFLINE/,
  )
  assert.equal(rowSubtitle({ hasFactor: false, degraded: false } satisfies SubtitleInput), '')
})

test('异常归一化为可读文案（不依赖 any）', () => {
  assert.equal(errText(new Error('KRONOS_DISABLED')), 'KRONOS_DISABLED')
  assert.equal(errText('boom'), 'boom')
  assert.equal(errText(undefined), '未知错误')
})
