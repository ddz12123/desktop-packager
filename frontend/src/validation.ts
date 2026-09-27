// appNameError 校验应用名，合法时返回 null，否则返回可直接展示的错误信息
export function appNameError(name: string): string | null {
  const trimmed = (name || '').trim()
  if (!trimmed) return '请输入应用名称'
  if ([...trimmed].length > 50) return '名称长度不能超过 50 个字符'
  if (trimmed.includes('..')) return '应用名称不能包含 ..'
  if (/[<>:"/\\|?*\x00-\x1f]/.test(trimmed)) return '应用名称包含非法字符'
  if (/[. ]$/.test(trimmed)) return '应用名称不能以空格或点结尾'
  const upper = trimmed.toUpperCase()
  const reserved = new Set([
    'CON', 'PRN', 'AUX', 'NUL',
    'COM1', 'COM2', 'COM3', 'COM4', 'COM5', 'COM6', 'COM7', 'COM8', 'COM9',
    'LPT1', 'LPT2', 'LPT3', 'LPT4', 'LPT5', 'LPT6', 'LPT7', 'LPT8', 'LPT9',
  ])
  const base = upper.split('.')[0]
  if (reserved.has(upper) || reserved.has(base)) return '应用名称不能使用 Windows 保留名'
  return null
}

export function validateProxyRule(rule: {
  path: string
  target: string
  rewrite: string
  enabled: boolean
}, index: number): string | null {
  if (!rule.enabled) return null
  const path = (rule.path || '').trim()
  if (!path) return `规则 #${index + 1}: 路径前缀不能为空`
  if (!path.startsWith('/')) return `规则 #${index + 1}: 路径前缀应以 / 开头`
  if (path.includes('..')) return `规则 #${index + 1}: 路径前缀非法`

  const target = (rule.target || '').trim()
  if (!target) return `规则 #${index + 1}: 目标地址不能为空`
  let url: URL
  try {
    url = new URL(target)
  } catch {
    return `规则 #${index + 1}: 目标地址无效（示例: http://localhost:8080/）`
  }
  if (url.protocol !== 'http:' && url.protocol !== 'https:') {
    return `规则 #${index + 1}: 目标地址仅支持 http/https`
  }
  const rewrite = (rule.rewrite || '').trim()
  if (rewrite && !rewrite.startsWith('/')) {
    return `规则 #${index + 1}: 重写路径应以 / 开头`
  }
  if (rewrite.includes('..')) return `规则 #${index + 1}: 重写路径非法`
  return null
}

export function canEnterStep(step: number, state: {
  distPath: string
  appName: string
}): boolean {
  if (step <= 0) return true
  if (!state.distPath) return false
  if (step >= 2 && appNameError(state.appName)) return false
  return true
}