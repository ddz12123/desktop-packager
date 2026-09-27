<script lang="ts" setup>
/**
 * StepImport 组件 - 导入构建产物步骤
 * 功能：
 * 1. 选择前端构建产物文件夹
 * 2. 上传 ZIP 压缩包并自动解压
 * 3. 显示导入的文件统计信息
 * 4. 验证构建产物是否有效（包含 index.html）
 */
import {ref} from 'vue'
import {NCard, NButton, NSpace, NResult, NSpin, NStatistic, NAlert} from 'naive-ui'
import {useStore} from '../store'
import {OpenDistFolder, UploadDistZip, GetDistInfo} from '../../wailsjs/go/main/App'

// 获取全局状态管理
const store = useStore()
const loading = ref(false)  // 加载状态
const error = ref('')       // 错误信息

/**
 * 选择文件夹方式导入构建产物
 * 打开文件夹选择对话框，验证并导入
 */
async function selectFolder() {
  loading.value = true
  error.value = ''
  try {
    // 打开文件夹选择对话框
    const path = await OpenDistFolder()
    if (!path) return // 用户取消选择

    // 获取目录信息（文件数量、大小等）
    const info = await GetDistInfo(path)

    // 保存到全局状态；应用名为空时用 package.json 推断的名称填充
    store.setDist(info.path, info.fileCount, info.totalSize)
    if (!store.state.appName && info.suggestedName) {
      store.setAppName(info.suggestedName)
    }
  } catch (e: any) {
    // 显示错误信息
    error.value = e?.message || String(e)
  } finally {
    loading.value = false
  }
}

/**
 * 上传 ZIP 压缩包方式导入构建产物
 * 选择 ZIP 文件，自动解压并导入
 */
async function uploadZip() {
  loading.value = true
  error.value = ''
  try {
    // 打开文件选择对话框并解压
    const path = await UploadDistZip(store.state.tempPath)
    if (!path) return // 用户取消选择

    // 获取解压后的目录信息；应用名为空时同样尝试用 package.json 预填
    const info = await GetDistInfo(path)
    store.setDist(info.path, info.fileCount, info.totalSize)
    if (!store.state.appName && info.suggestedName) {
      store.setAppName(info.suggestedName)
    }
  } catch (e: any) {
    // 显示错误信息
    error.value = e?.message || String(e)
  } finally {
    loading.value = false
  }
}

/**
 * 格式化文件大小显示
 * @param bytes - 字节数
 * @returns 格式化后的字符串（B, KB, MB）
 */
function formatSize(bytes: number): string {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
}

/**
 * 进入下一步（应用设置步骤）
 */
function nextStep() {
  store.setCurrentStep(1)
}
</script>

<template>
  <div class="step-container">
    <div class="step-title">
      <h3>导入前端构建产物</h3>
      <p>选择 Vite、Webpack 等工具打包后的 dist 文件夹，或包含 dist 内容的 ZIP 压缩包</p>
    </div>

    <NAlert v-if="error" type="error" closable @close="error = ''" style="margin-bottom: 16px">
      {{ error }}
    </NAlert>

    <!-- Not imported yet -->
    <NCard v-if="!store.state.distPath" class="import-card">
      <div class="import-zone">
        <div class="import-icon">📁</div>
        <p class="import-hint">选择前端构建产物</p>
        <NSpin :show="loading">
          <NSpace justify="center">
            <NButton type="primary" size="large" @click="selectFolder" :disabled="loading">
              选择文件夹
            </NButton>
            <NButton size="large" @click="uploadZip" :disabled="loading">
              上传 ZIP 压缩包
            </NButton>
          </NSpace>
        </NSpin>
        <div class="import-note">
          <p>支持 Vite、Webpack、Rollup 等构建工具输出的 dist 目录</p>
          <p>
            ZIP 解压目录：
            <code>{{ store.state.tempPath || '未设置，使用 ZIP 所在目录' }}</code>
          </p>
          <p class="zip-format"><strong>ZIP 格式说明：</strong></p>
          <ul class="zip-list">
            <li>格式1：ZIP 内直接包含 <code>index.html</code>、<code>assets/</code> 等文件</li>
            <li>格式2：ZIP 内包含一个文件夹（如 <code>dist/</code>），文件夹内是上述文件</li>
          </ul>
        </div>
      </div>
    </NCard>

    <!-- Imported successfully -->
    <NCard v-else class="import-card">
      <NResult status="success" title="导入成功" :description="'路径: ' + store.state.distPath">
        <template #footer>
          <div class="import-stats">
            <NSpace justify="center" size="large">
              <NStatistic label="文件数量" :value="store.state.distFileCount + ' 个'" />
              <NStatistic label="总大小" :value="formatSize(store.state.distTotalSize)" />
            </NSpace>
          </div>
          <NSpace justify="center" style="margin-top: 16px">
            <NButton @click="store.clearDist(); error = ''">
              重新选择
            </NButton>
            <NButton type="primary" @click="nextStep">
              下一步
            </NButton>
          </NSpace>
        </template>
      </NResult>
    </NCard>
  </div>
</template>

<style scoped>
.step-container {
  padding: 32px;
  max-width: 700px;
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

.import-card {
  min-height: 300px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.import-zone {
  text-align: center;
  padding: 40px 20px;
}

.import-icon {
  font-size: 64px;
  margin-bottom: 16px;
}

.import-hint {
  font-size: 16px;
  color: #333;
  margin-bottom: 24px;
}

.import-note {
  font-size: 12px;
  color: #999;
  margin-top: 16px;
}

.import-note p {
  margin: 4px 0;
}

.zip-format {
  margin-top: 8px !important;
}

.zip-list {
  margin: 4px 0;
  padding-left: 20px;
  text-align: left;
  display: inline-block;
}

.zip-list li {
  margin: 2px 0;
}

.zip-list code {
  background: #f0f0f0;
  padding: 1px 4px;
  border-radius: 3px;
  font-size: 11px;
}

.import-stats {
  margin: 16px 0;
}
</style>
