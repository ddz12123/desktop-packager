<script lang="ts" setup>
/**
 * GlobalPanels 全局面板：构建方案、全局设置、拖拽导入、配置持久化。
 * 从 App.vue 拆出，使「方案管理」与「设置」成为两个独立入口，
 * 并统一在此处理全局性的副作用（启动恢复、自动保存、文件拖放）。
 */
import {onMounted, onUnmounted, ref, watch} from 'vue'
import {
  NAlert,
  NButton,
  NInputGroup,
  NInput,
  NModal,
  NPopconfirm,
  NSpace,
  useMessage,
} from 'naive-ui'
import {useStore} from '../store'
import {
  ApplyUpdate,
  AppVersion,
  CheckUpdate,
  DeleteProfile,
  ExportProfile,
  GetDistInfo,
  ImportFromPaths,
  ImportProfile,
  ListProfiles,
  LoadProfile,
  LoadSettings,
  OpenTempFolder,
  SaveProfile,
  SaveSettings,
  ValidateIcon,
} from '../../wailsjs/go/main/App'
import {EventsOn} from '../../wailsjs/runtime/runtime'

const store = useStore()
const message = useMessage()

const showSettings = ref(false)
const showProfiles = ref(false)
const panelError = ref('')

// 方案管理
const profileNames = ref<string[]>([])
const profileName = ref('')
const profileBusy = ref(false)

// ---------- 对外打开入口（App.vue 侧边栏按钮调用） ----------
function openSettings() {
  showSettings.value = true
}

function openProfiles() {
  showProfiles.value = true
  refreshProfiles()
}

defineExpose({openSettings, openProfiles})

// ---------- 启动恢复 + 自动保存 ----------

async function refreshDistInfo() {
  if (!store.state.distPath) return
  try {
    const info = await GetDistInfo(store.state.distPath)
    store.setDist(info.path, info.fileCount, info.totalSize)
  } catch {
    // 上次的目录已不存在，清空导入状态
    store.clearDist()
  }
}

async function applyLoadedSettings(settings: Record<string, any>) {
  store.applySettings(settings || {})
  // 图标预览不持久化，按路径重新生成；路径失效则清空
  if (store.state.iconPath) {
    try {
      const preview = await ValidateIcon(store.state.iconPath)
      store.setIcon(store.state.iconPath, preview)
    } catch {
      store.clearIcon()
    }
  }
  await refreshDistInfo()
}

let saveTimer: ReturnType<typeof setTimeout> | undefined
let unlisteners: Array<() => void> = []

onMounted(async () => {
  try {
    const settings = await LoadSettings()
    await applyLoadedSettings(settings as any)
  } catch {
    // 首次使用或读取失败时使用默认配置
  }
  await refreshProfiles()
  try {
    appVersion.value = await AppVersion()
  } catch {
    appVersion.value = ''
  }

  // 配置变化时自动保存（防抖）
  watch(store.state, () => {
    if (saveTimer) clearTimeout(saveTimer)
    saveTimer = setTimeout(() => {
      SaveSettings(store.collectSettings() as any).catch(() => {})
    }, 500)
  })

  // 构建事件挂在应用生命周期上：构建中切换步骤也不会丢失完成事件
  unlisteners.push(EventsOn('build:progress', (data: any) => {
    store.setBuildProgress(data.step, data.progress)
  }))
  unlisteners.push(EventsOn('build:complete', (data: any) => {
    store.setBuildComplete(data.outputPath, data.signScriptPath)
  }))
  unlisteners.push(EventsOn('app:file-drop', onFileDrop as any))
})

onUnmounted(() => {
  unlisteners.forEach((u) => u())
  unlisteners = []
})

// ---------- 拖拽导入 ----------

const dropBusy = ref(false)

async function onFileDrop(paths: string[]) {
  if (!paths || paths.length === 0) return
  if (store.state.building) {
    message.info('构建进行中，暂不能导入')
    return
  }
  if (dropBusy.value) return
  dropBusy.value = true
  try {
    const distPath = await ImportFromPaths(store.state.tempPath, paths)
    if (!distPath) return
    const info = await GetDistInfo(distPath)
    store.setDist(info.path, info.fileCount, info.totalSize)
    if (!store.state.appName && info.suggestedName) {
      store.setAppName(info.suggestedName)
    }
    store.setCurrentStep(0)
    message.success(`导入成功：${info.path}（${info.fileCount} 个文件）`)
  } catch (e: any) {
    message.error(e?.message || String(e))
  } finally {
    dropBusy.value = false
  }
}

// ---------- 设置 ----------

const appVersion = ref('')
const updateChecking = ref(false)
const updateApplying = ref(false)
const updateChecked = ref(false)
const updateInfo = ref<any>(null)

async function checkUpdate() {
  panelError.value = ''
  updateChecking.value = true
  try {
    updateInfo.value = await CheckUpdate()
    updateChecked.value = true
    if (updateInfo.value?.hasUpdate) {
      message.success(`发现新版本 v${updateInfo.value.latestVersion}`)
    } else {
      message.success('当前已是最新版本')
    }
  } catch (e: any) {
    message.error(e?.message || String(e))
  } finally {
    updateChecking.value = false
  }
}

function openReleaseNotes() {
  const url = updateInfo.value?.notesUrl
  if (url) window.open(url, '_blank')
}

async function applyUpdate() {
  panelError.value = ''
  const url = updateInfo.value?.downloadUrl
  if (!url) return
  try {
    updateApplying.value = true
    await ApplyUpdate(url)
    message.success('更新包已就绪，应用即将自动重启并完成升级…')
  } catch (e: any) {
    updateApplying.value = false
    message.error(e?.message || String(e))
  }
}

async function selectTempFolder() {
  panelError.value = ''
  try {
    const path = await OpenTempFolder()
    if (!path) return
    store.setTempPath(path)
  } catch (e: any) {
    panelError.value = e?.message || String(e)
  }
}

// ---------- 构建方案 ----------

async function refreshProfiles() {
  try {
    profileNames.value = (await ListProfiles()) || []
  } catch {
    profileNames.value = []
  }
}

async function handleSaveProfile() {
  panelError.value = ''
  try {
    profileBusy.value = true
    const name = await SaveProfile(profileName.value, store.collectSettings() as any)
    profileName.value = ''
    message.success(`方案「${name}」已保存`)
    await refreshProfiles()
  } catch (e: any) {
    panelError.value = e?.message || String(e)
  } finally {
    profileBusy.value = false
  }
}

async function handleLoadProfile(name: string) {
  panelError.value = ''
  try {
    const settings = await LoadProfile(name)
    await applyLoadedSettings(settings as any)
    message.success(`方案「${name}」已载入`)
  } catch (e: any) {
    panelError.value = e?.message || String(e)
  }
}

async function handleDeleteProfile(name: string) {
  panelError.value = ''
  try {
    await DeleteProfile(name)
    message.success(`方案「${name}」已删除`)
    await refreshProfiles()
  } catch (e: any) {
    panelError.value = e?.message || String(e)
  }
}

async function handleExportProfile(name: string) {
  panelError.value = ''
  try {
    const path = await ExportProfile(name)
    if (path) {
      message.success(`已导出到 ${path}`)
    }
  } catch (e: any) {
    panelError.value = e?.message || String(e)
  }
}

async function handleImportProfile() {
  panelError.value = ''
  try {
    const name = await ImportProfile()
    if (!name) return
    message.success(`方案「${name}」已导入`)
    await refreshProfiles()
  } catch (e: any) {
    panelError.value = e?.message || String(e)
  }
}
</script>

<template>
  <!-- 全局设置：仅环境类配置 -->
  <NModal v-model:show="showSettings" preset="card" title="设置" style="width: 560px">
    <NAlert
      v-if="panelError"
      type="error"
      closable
      style="margin-bottom: 14px"
      @close="panelError = ''"
    >
      {{ panelError }}
    </NAlert>

    <section class="settings-group">
      <header class="group-header">
        <span class="group-title">通用</span>
        <span class="group-desc">临时文件存放位置</span>
      </header>
      <div class="group-body">
        <div class="field-label">临时目录</div>
        <NInputGroup>
          <NInput
            :value="store.state.tempPath"
            @update:value="store.setTempPath($event)"
            placeholder="留空则使用当前导入文件所在目录"
          />
          <NButton @click="selectTempFolder">选择目录</NButton>
        </NInputGroup>
        <div class="field-hint">
          ZIP 解压、构建工作目录都会放在这里；退出应用时自动清理本次会话创建的临时目录。
          <a class="text-btn" @click="store.setTempPath('')">清空</a>
        </div>
      </div>
    </section>

    <section class="settings-group">
      <header class="group-header">
        <span class="group-title">关于与更新</span>
        <span class="group-desc">检查并安装新版本</span>
      </header>
      <div class="group-body">
        <div class="version-row">
          <span class="field-label">当前版本</span>
          <span class="version-badge">v{{ appVersion || '…' }}</span>
          <span v-if="updateChecked && !updateInfo?.hasUpdate" class="status-pill ok">已是最新</span>
          <span v-else-if="updateInfo?.hasUpdate" class="status-pill new">发现 v{{ updateInfo.latestVersion }}</span>
        </div>
        <NSpace>
          <NButton size="small" :loading="updateChecking" @click="checkUpdate">
            检查更新
          </NButton>
          <NButton
            v-if="updateInfo?.hasUpdate"
            type="primary"
            size="small"
            :loading="updateApplying"
            @click="applyUpdate"
          >
            一键更新
          </NButton>
          <NButton v-if="updateInfo?.hasUpdate" quaternary size="small" @click="openReleaseNotes">
            更新说明
          </NButton>
        </NSpace>
        <div class="field-hint">
          更新源为 GitHub Releases，需可访问网络；更新包下载完成后自动替换并重启应用。
        </div>
      </div>
    </section>

    <template #footer>
      <NSpace justify="end">
        <NButton type="primary" @click="showSettings = false">完成</NButton>
      </NSpace>
    </template>
  </NModal>

  <!-- 构建方案：独立的方案管理面板 -->
  <NModal v-model:show="showProfiles" preset="card" title="构建方案" style="width: 620px">
    <NAlert
      v-if="panelError"
      type="error"
      closable
      style="margin-bottom: 14px"
      @close="panelError = ''"
    >
      {{ panelError }}
    </NAlert>

    <section class="settings-group">
      <header class="group-header">
        <span class="group-title">保存当前配置</span>
        <span class="group-desc">命名后即可随时载入</span>
      </header>
      <div class="group-body">
        <NInputGroup>
          <NInput
            v-model:value="profileName"
            placeholder="方案名称，如：管理后台 v1"
            @keydown.enter="handleSaveProfile"
          />
          <NButton type="primary" :disabled="!profileName.trim() || profileBusy" @click="handleSaveProfile">
            保存
          </NButton>
        </NInputGroup>
        <div class="field-hint">
          方案包含导入目录、应用配置、代理规则、签名等全部构建配置；证书密码不会保存。
        </div>
      </div>
    </section>

    <section class="settings-group">
      <header class="group-header">
        <span class="group-title">已保存方案</span>
        <span class="group-desc">{{ profileNames.length }} 个</span>
      </header>
      <div class="group-body">
        <div v-if="profileNames.length === 0" class="panel-empty">
          还没有保存过方案。配置好一次构建后，在上方命名保存，之后可一键载入。
        </div>
        <div v-else class="profile-list">
          <div v-for="name in profileNames" :key="name" class="profile-row">
            <span class="profile-name" :title="name">{{ name }}</span>
            <NSpace :size="4" :wrap="false">
              <NButton size="tiny" type="primary" @click="handleLoadProfile(name)">载入</NButton>
              <NButton size="tiny" @click="handleExportProfile(name)">导出</NButton>
              <NPopconfirm @positive-click="handleDeleteProfile(name)">
                <template #trigger>
                  <NButton size="tiny" type="error" secondary>删除</NButton>
                </template>
                确定删除方案「{{ name }}」？
              </NPopconfirm>
            </NSpace>
          </div>
        </div>
      </div>
    </section>

    <template #footer>
      <NSpace justify="space-between">
        <NButton @click="handleImportProfile">从文件导入</NButton>
        <NButton type="primary" @click="showProfiles = false">完成</NButton>
      </NSpace>
    </template>
  </NModal>
</template>

<style scoped>
.settings-group {
  border: 1px solid #eceef2;
  border-radius: 10px;
  padding: 14px 16px 16px;
  background: #fafbfc;
}

.settings-group + .settings-group {
  margin-top: 14px;
}

.group-header {
  display: flex;
  align-items: baseline;
  gap: 8px;
  margin-bottom: 12px;
}

.group-title {
  font-size: 14px;
  font-weight: 600;
  color: #1f2329;
}

.group-desc {
  font-size: 12px;
  color: #8f959e;
}

.group-body {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.field-label {
  font-size: 13px;
  color: #4e5358;
}

.field-hint {
  font-size: 12px;
  color: #9aa0a8;
  line-height: 1.6;
}

.text-btn {
  color: #3b5bdb;
  cursor: pointer;
  margin-left: 4px;
}

.text-btn:hover {
  text-decoration: underline;
}

.version-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.version-badge {
  font-family: Consolas, monospace;
  font-size: 13px;
  background: #eef2ff;
  color: #3b5bdb;
  padding: 2px 10px;
  border-radius: 999px;
}

.status-pill {
  font-size: 12px;
  padding: 2px 10px;
  border-radius: 999px;
}

.status-pill.ok {
  background: #e8f7ee;
  color: #18a058;
}

.status-pill.new {
  background: #e8f1ff;
  color: #2080f0;
}

.panel-empty {
  color: #999;
  font-size: 13px;
  padding: 20px 0;
  text-align: center;
  border: 1px dashed #e0e0e6;
  border-radius: 8px;
  background: #fff;
}

.profile-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  max-height: 280px;
  overflow-y: auto;
}

.profile-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 12px;
  border-radius: 8px;
  border: 1px solid #eceef2;
  background: #fff;
}

.profile-row:hover {
  border-color: #d8dce3;
}

.profile-name {
  font-size: 13px;
  color: #333;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
