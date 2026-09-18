// 快速语法校验：抽出 .vue 的 <script setup> 块，交给 esbuild 解析（不解析依赖，只查语法）。
import { readFileSync } from 'node:fs'
import { transform } from 'esbuild'

const file = process.argv[2]
const src = readFileSync(file, 'utf8')
const m = src.match(/<script(?:\s[^>]*)?>([\s\S]*?)<\/script>/)
if (!m) {
  console.error(`✗ ${file}: 未找到 <script> 块`)
  process.exitCode = 1
} else {
  try {
    await transform(m[1], { loader: 'ts' })
    console.log(`✓ ${file}: script 语法通过 (${m[1].length} 字符)`)
  } catch (e) {
    console.error(`✗ ${file}: ${e.errors?.[0]?.text ?? e.message}`)
    console.error(e.errors?.[0]?.location ? `  ${e.errors[0].location.line}:${e.errors[0].location.column}` : '')
    process.exitCode = 1
  }
}
