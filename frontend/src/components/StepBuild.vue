<script lang="ts" setup>
import {computed, ref} from 'vue'
import {NCard, NButton, NSpace, NProgress, NAlert, NResult, NSpin, useMessage} from 'naive-ui'
import {useStore} from '../store'
import {appNameError} from '../validation'
import {BuildApp, CancelBuild, OpenOutputFolder, RunOutput, SelectOutputPath} from '../../wailsjs/go/main/App'
import {appconf} from '../../wailsjs/go/models'

const store = useStore()
const message = useMessage()
const openError = ref('')
const canceling = ref(false)

const canBuild = computed(() => {
  return !!store.state.distPath
    && !!store.state.savePath
    && !appNameError(store.state.appName)
})

async function selectSavePath() {
  openError.value = ''
  try {
    const path = await SelectOutputPath(store.state.appName.trim() || 'app')
    if (!path) return
    store.setSavePath(path)
  } catch (e: any) {
    openError.value = e?.message || String(e)
  }
}

async function startBuild() {
  openError.value = ''
  if (!store.state.distPath) {
    store.setBuildError('请先导入前端构建产物')
    return
  }
  const nameErr = appNameError(store.state.appName)
  if (nameErr) {
    store.setBuildError(nameErr)
    return
  }
  if (!store.state.savePath) {
    store.setBuildError('请先选择保存位置')
    return
  }

  store.setBuilding(true)
  try {
    await BuildApp(appconf.BuildConfig.createFrom({
      appName: store.state.appName.trim(),
      iconPath: store.state.iconPath,
      distPath: store.state.distPath,
      outputPath: store.state.savePath,
      tempPath: store.state.tempPath,
      proxyRules: store.state.proxyRules.map((r) => ({...r})),
      windowWidth: store.state.windowWidth,
      windowHeight: store.state.windowHeight,
      windowFullscreen: store.state.windowFullscreen,
      windowMaximized: store.state.windowMaximized,
      confirmClose: store.state.confirmClose,
      version: store.state.version,
      description: store.state.description,
      company: store.state.company,
      windowTitle: store.state.windowTitle,
      singleInstance: store.state.singleInstance,
      rememberWindow: store.state.rememberWindow,
      targetPlatform: store.state.targetPlatform,
      signPfxPath: store.state.signPfxPath,
      signTimestamp: store.state.signTimestamp,
    }))
  } catch (e: any) {
    store.setBuildError(e?.message || String(e))
  }
}

async function cancelBuild() {
  openError.value = ''
  try {
    canceling.value = true
    await CancelBuild()
  } catch (e: any) {
    openError.value = e?.message || String(e)
  } finally {
    canceling.value = false
  }
}

async function openOutput() {
  openError.value = ''
  if (!store.state.outputPath) return
  try {
    await OpenOutputFolder(store.state.outputPath)
  } catch (e: any) {
    openError.value = e?.message || String(e)
    message.error(openError.value)
  }
}

async function runOutput() {
  openError.value = ''
  if (!store.state.outputPath) return
  try {
    await RunOutput(store.state.outputPath)
  } catch (e: any) {
    openError.value = e?.message || String(e)
    message.error(openError.value)
  }
}

function resetBuild() {
  store.setBuilding(false)
  store.setBuildProgress('', 0)
  store.setBuildError('')
  store.setOutputPath('')
  openError.value = ''
}

function prevStep() {
  store.setCurrentStep(2)
}
</script>

<template>
  <div class="step-container">
    <div class="step-title">
      <h3>构建生成</h3>
      <p>确认配置无误后，点击构建按钮生成桌面应用</p>
    </div>

    <NCard style="margin-bottom: 16px">
      <div class="config-summary">
        <div class="summary-item">
          <span class="label">应用名称:</span>
          <span class="value">{{ store.state.appName || '未设置' }}</span>
        </div>
        <div class="summary-item">
          <span class="label">版本:</span>
          <span class="value">{{ store.state.version || '1.0.0' }}</span>
        </div>
        <div class="summary-item">
          <span class="label">构建产物:</span>
          <span class="value">{{ store.state.distPath || '未导入' }}</span>
        </div>
        <div class="summary-item">
          <span class="label">保存位置:</span>
          <span class="value">{{ store.state.savePath || '未选择' }}</span>
        </div>
        <div class="summary-item">
          <span class="label">临时目录:</span>
          <span class="value">{{ store.state.tempPath || '未设置，使用当前导入目录' }}</span>
        </div>
        <div class="summary-item">
          <span class="label">应用图标:</span>
          <span class="value">{{ store.state.iconPath ? '已选择' : '使用默认' }}</span>
        </div>
        <div class="summary-item">
          <span class="label">代理规则:</span>
          <span class="value">{{ store.state.proxyRules.length }} 条</span>
        </div>
        <div class="summary-item">
          <span class="label">关闭确认:</span>
          <span class="value">{{ store.state.confirmClose ? '开启' : '关闭' }}</span>
        </div>
        <div class="summary-item">
          <span class="label">窗口大小:</span>
          <span class="value">
            {{
              store.state.windowFullscreen
                ? '全屏'
                : store.state.windowMaximized
                  ? '最大化'
                  : store.state.windowWidth + ' x ' + store.state.windowHeight
            }}
          </span>
        </div>
        <div class="summary-item">
          <span class="label">发布平台:</span>
          <span class="value">Windows</span>
        </div>
        <div class="summary-item">
          <span class="label">签名脚本:</span>
          <span class="value">{{ store.state.signPfxPath ? '生成（构建后双击运行）' : '不生成' }}</span>
        </div>
      </div>
    </NCard>

    <NAlert v-if="store.state.buildError" type="error" closable style="margin-bottom: 16px">
      {{ store.state.buildError }}
    </NAlert>
    <NAlert v-if="openError" type="warning" closable style="margin-bottom: 16px" @close="openError = ''">
      {{ openError }}
    </NAlert>

    <NCard v-if="store.state.building">
      <div class="build-progress">
        <NSpin size="large" style="margin-bottom: 16px" />
        <NProgress
          type="line"
          :percentage="store.state.buildProgress"
          :processing="store.state.buildProgress < 100"
          style="margin-bottom: 12px"
        />
        <p class="progress-text">{{ store.state.buildStep }}</p>
        <NButton size="small" style="margin-top: 16px" :disabled="canceling" @click="cancelBuild">
          取消构建
        </NButton>
      </div>
    </NCard>

    <NCard v-else-if="store.state.outputPath">
      <NResult
        status="success"
        title="构建成功!"
        :description="'输出路径: ' + store.state.outputPath"
      >
        <template #footer>
          <NAlert
            v-if="store.state.signScriptPath"
            type="info"
            style="margin-bottom: 16px; text-align: left"
          >
            签名脚本已生成: {{ store.state.signScriptPath }}，双击运行并输入证书密码即可完成签名。
          </NAlert>
          <NSpace justify="center">
            <NButton type="primary" @click="runOutput">
              试运行
            </NButton>
            <NButton @click="openOutput">
              打开输出目录
            </NButton>
            <NButton @click="resetBuild">
              重新构建
            </NButton>
          </NSpace>
        </template>
      </NResult>
    </NCard>

    <NCard v-else>
      <div class="build-ready">
        <div class="ready-icon">🚀</div>
        <p>一切准备就绪，点击下方按钮开始构建</p>
        <div class="save-path-box" :class="{ empty: !store.state.savePath }">
          <div class="save-path-meta">
            <span class="save-path-label">保存位置</span>
            <NButton text type="primary" size="tiny" @click="selectSavePath">
              {{ store.state.savePath ? '更改' : '选择位置' }}
            </NButton>
          </div>
          <div class="save-path-value" :title="store.state.savePath">
            {{ store.state.savePath || '尚未选择保存位置，点击右上角「选择位置」' }}
          </div>
        </div>
        <NSpace justify="center" style="margin-top: 20px">
          <NButton @click="prevStep">上一步</NButton>
          <NButton
            type="primary"
            size="large"
            @click="startBuild"
            :disabled="!canBuild"
          >            开始构建
          </NButton>
        </NSpace>
      </div>
    </NCard>
  </div>
</template>

<style scoped>
.step-container {
  padding: 32px;
  max-width: 600px;
  margin: 0 auto;
}
.step-title {
  margin-bottom: 24px;
}
.step-title h3 {
  margin: 0 0 8px;
  font-size: 20px;
  color: #333;
}
.step-title p {
  margin: 0;
  color: #666;
  font-size: 14px;
}
.config-summary {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.summary-item {
  display: flex;
  gap: 8px;
  font-size: 14px;
}
.summary-item .label {
  color: #999;
  min-width: 80px;
}
.summary-item .value {
  color: #333;
  word-break: break-all;
}
.build-progress {
  text-align: center;
  padding: 32px 0;
}
.progress-text {
  color: #666;
  font-size: 14px;
  margin: 0;
}
.build-ready {
  text-align: center;
  padding: 48px 0;
}
.ready-icon {
  font-size: 64px;
  margin-bottom: 16px;
}
.build-ready p {
  color: #666;
  font-size: 14px;
  margin: 0;
}
.save-path-box {
  margin: 20px auto 0;
  max-width: 560px;
  text-align: left;
  border: 1px solid #e5e6eb;
  border-radius: 10px;
  padding: 12px 16px;
  background: #fafbfc;
}

.save-path-box.empty .save-path-value {
  color: #b0b6bf;
}

.save-path-meta {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 6px;
}

.save-path-label {
  font-size: 12px;
  color: #8a919f;
}

.save-path-value {
  font-size: 13px;
  color: #333;
  word-break: break-all;
  line-height: 1.6;
}
</style>