// Node ESM 解析补丁：src 里的相对导入不带扩展名（Vite 习惯），
// Node 直接跑 .ts 时需要补 .ts 后缀。仅用于测试，不影响构建产物。
export async function resolve(specifier, context, nextResolve) {
  if (specifier.startsWith('./') || specifier.startsWith('../')) {
    try {
      return await nextResolve(specifier, context)
    } catch (err) {
      return nextResolve(specifier + '.ts', context)
    }
  }
  return nextResolve(specifier, context)
}
