// 测试入口前注册 loader：node --import ./tests/register-ts-ext.mjs tests/xxx.mjs
import { register } from 'node:module'

register('./loader-ts-ext.mjs', import.meta.url)
