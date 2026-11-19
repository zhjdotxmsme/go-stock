// vue-tsc 兼容入口：vue-tsc@3 需要 TypeScript 5 的 JS 版 tsc（typescript@7 是原生二进制，
// lib/tsc.js 只是启动器，vue-tsc 的源码改写机制无法工作）。
// 用法与 pnpm exec vue-tsc 相同：node tests/run-vue-tsc.mjs --noEmit
import { createRequire } from 'node:module'
import { dirname, join } from 'node:path'

const require = createRequire(import.meta.url)
const tsPkg = require.resolve('typescript5/package.json')
const tscPath = join(dirname(tsPkg), 'lib', 'tsc.js')
require('vue-tsc').run(tscPath)
