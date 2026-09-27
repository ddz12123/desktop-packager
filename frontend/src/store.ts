import {reactive, readonly} from 'vue'

export interface ProxyRule {
  path: string
  target: string
  rewrite: string
  enabled: boolean
}

export interface AppState {
  distPath: string
  distFileCount: number
  distTotalSize: number

  appName: string
  iconPath: string
  iconPreview: string
  tempPath: string
  savePath: string
  version: string
  description: string
  company: string
  confirmClose: boolean

  windowTitle: string
  singleInstance: boolean
  rememberWindow: boolean
  targetPlatform: string

  signPfxPath: string
  signTimestamp: string

  proxyRules: ProxyRule[]

  building: boolean
  buildProgress: number
  buildStep: string
  outputPath: string
  signScriptPath: string
  buildError: string

  currentStep: number

  windowWidth: number
  windowHeight: number
  windowFullscreen: boolean
  windowMaximized: boolean
}

const state = reactive<AppState>({
  distPath: '',
  distFileCount: 0,
  distTotalSize: 0,

  appName: '',
  iconPath: '',
  iconPreview: '',
  tempPath: '',
  savePath: '',
  version: '1.0.0',
  description: '',
  company: '',
  confirmClose: true,

  windowTitle: '',
  singleInstance: true,
  rememberWindow: true,
  targetPlatform: 'windows',

  signPfxPath: '',
  signTimestamp: '',

  proxyRules: [],

  building: false,
  buildProgress: 0,
  buildStep: '',
  outputPath: '',
  signScriptPath: '',
  buildError: '',

  currentStep: 0,

  windowWidth: 1024,
  windowHeight: 768,
  windowFullscreen: false,
  windowMaximized: false,
})

// 需要持久化的字段清单：collectSettings / applySettings 共用，
// 新增持久化字段时只需在此追加，并同步 Go 侧 BuildConfig。
const PERSIST_STRING_FIELDS = [
  'distPath', 'appName', 'iconPath', 'tempPath', 'savePath',
  'version', 'description', 'company', 'windowTitle', 'targetPlatform',
  'signPfxPath', 'signTimestamp',
] as const
const PERSIST_BOOL_FIELDS = [
  'confirmClose', 'singleInstance', 'rememberWindow',
  'windowFullscreen', 'windowMaximized',
] as const
const PERSIST_NUMBER_FIELDS = ['windowWidth', 'windowHeight'] as const

export function useStore() {
  return {
    state: readonly(state),

    // collectSettings 提取需要持久化的配置（不含密码、图标预览、构建过程状态）
    collectSettings() {
      const settings: Record<string, any> = {
        proxyRules: state.proxyRules.map((r) => ({...r})),
      }
      for (const f of PERSIST_STRING_FIELDS) settings[f] = state[f]
      for (const f of PERSIST_BOOL_FIELDS) settings[f] = state[f]
      for (const f of PERSIST_NUMBER_FIELDS) settings[f] = state[f]
      return settings
    },

    // applySettings 恢复持久化配置，缺省字段保持当前默认值
    applySettings(s: Record<string, any>) {
      if (!s) return
      for (const f of PERSIST_STRING_FIELDS) {
        if (typeof s[f] === 'string') (state as any)[f] = s[f]
      }
      for (const f of PERSIST_BOOL_FIELDS) {
        if (typeof s[f] === 'boolean') (state as any)[f] = s[f]
      }
      for (const f of PERSIST_NUMBER_FIELDS) {
        if (typeof s[f] === 'number' && s[f] > 0) (state as any)[f] = s[f]
      }
      if (Array.isArray(s.proxyRules)) {
        state.proxyRules = s.proxyRules.map((r: any) => ({
          path: String(r?.path ?? ''),
          target: String(r?.target ?? ''),
          rewrite: String(r?.rewrite ?? ''),
          enabled: Boolean(r?.enabled),
        }))
      }
    },

    setDist(path: string, fileCount: number, totalSize: number) {
      state.distPath = path
      state.distFileCount = fileCount
      state.distTotalSize = totalSize
    },

    clearDist() {
      state.distPath = ''
      state.distFileCount = 0
      state.distTotalSize = 0
    },

    setAppName(name: string) {
      state.appName = name
    },

    setIcon(path: string, preview: string) {
      state.iconPath = path
      state.iconPreview = preview
    },

    clearIcon() {
      state.iconPath = ''
      state.iconPreview = ''
    },

    setTempPath(path: string) {
      state.tempPath = path
    },

    setSavePath(path: string) {
      state.savePath = path
    },

    setVersion(version: string) {
      state.version = version
    },

    setDescription(description: string) {
      state.description = description
    },

    setCompany(company: string) {
      state.company = company
    },

    setConfirmClose(confirmClose: boolean) {
      state.confirmClose = confirmClose
    },

    setWindowTitle(title: string) {
      state.windowTitle = title
    },

    setSingleInstance(v: boolean) {
      state.singleInstance = v
    },

    setRememberWindow(v: boolean) {
      state.rememberWindow = v
    },

    setSignPfxPath(path: string) {
      state.signPfxPath = path
    },

    setSignTimestamp(ts: string) {
      state.signTimestamp = ts
    },


    addProxyRule() {
      state.proxyRules.push({
        path: '/api/',
        target: 'http://localhost:8080/',
        rewrite: '',
        enabled: true,
      })
    },

    removeProxyRule(index: number) {
      state.proxyRules.splice(index, 1)
    },

    updateProxyRule(index: number, rule: Partial<ProxyRule>) {
      Object.assign(state.proxyRules[index], rule)
    },

    setBuilding(building: boolean) {
      state.building = building
      if (building) {
        state.buildError = ''
        state.outputPath = ''
        state.signScriptPath = ''
        state.buildProgress = 0
        state.buildStep = ''
      }
    },

    setBuildProgress(step: string, progress: number) {
      state.buildStep = step
      state.buildProgress = progress
    },

    setBuildComplete(outputPath: string, signScriptPath: string) {
      state.building = false
      state.outputPath = outputPath
      state.signScriptPath = signScriptPath || ''
      state.buildProgress = 100
      state.buildStep = '构建完成!'
    },

    setBuildError(error: string) {
      state.building = false
      state.buildError = error
    },

    setOutputPath(path: string) {
      state.outputPath = path
    },

    setCurrentStep(step: number) {
      state.currentStep = step
    },

    setWindowWidth(width: number) {
      state.windowWidth = width
    },

    setWindowHeight(height: number) {
      state.windowHeight = height
    },

    setWindowFullscreen(fullscreen: boolean) {
      state.windowFullscreen = fullscreen
      if (fullscreen) {
        state.windowMaximized = false
      }
    },

    setWindowMaximized(maximized: boolean) {
      state.windowMaximized = maximized
      if (maximized) {
        state.windowFullscreen = false
      }
    },
  }
}