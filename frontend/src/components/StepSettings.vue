<script lang="ts" setup>
import {ref} from 'vue'
import {NCard, NButton, NSpace, NInput, NFormItem, NForm, NImage, NAlert, NInputNumber, NSwitch, NDivider} from 'naive-ui'
import {useStore} from '../store'
import {appNameError} from '../validation'
import {OpenIconFile, OpenSignPfxFile, ValidateIcon} from '../../wailsjs/go/main/App'

const store = useStore()
const formRef = ref()
const error = ref('')

const rules = {
  appName: [
    {
      required: true,
      validator(_: any, value: string) {
        const msg = appNameError(value)
        if (msg) return new Error(msg)
        return true
      },
      trigger: ['blur', 'input'],
    },
  ],
}

async function selectIcon() {
  error.value = ''
  try {
    const path = await OpenIconFile()
    if (!path) return
    const preview = await ValidateIcon(path)
    store.setIcon(path, preview)
  } catch (e: any) {
    error.value = e?.message || String(e)
  }
}

function clearIcon() {
  store.clearIcon()
}

async function selectPfx() {
  error.value = ''
  try {
    const path = await OpenSignPfxFile()
    if (!path) return
    store.setSignPfxPath(path)
  } catch (e: any) {
    error.value = e?.message || String(e)
  }
}

function clearPfx() {
  store.setSignPfxPath('')
}

function prevStep() {
  store.setCurrentStep(0)
}

async function nextStep() {
  try {
    await formRef.value?.validate()
    store.setCurrentStep(2)
  } catch {
    // keep on page
  }
}
</script>

<template>
  <div class="step-container">
    <div class="step-title">
      <h3>应用配置</h3>
      <p>设置应用名称、图标、版本信息和窗口参数</p>
    </div>

    <NAlert v-if="error" type="error" closable @close="error = ''" style="margin-bottom: 16px">
      {{ error }}
    </NAlert>

    <NCard>
      <NForm ref="formRef" :model="{ appName: store.state.appName }" :rules="rules" label-placement="top">
        <NFormItem label="应用名称" path="appName">
          <NInput
            :value="store.state.appName"
            @update:value="store.setAppName($event)"
            placeholder="将作为 exe 文件名，例如 MyApp"
            maxlength="50"
            show-count
          />
        </NFormItem>

        <NFormItem label="版本号">
          <NInput
            :value="store.state.version"
            @update:value="store.setVersion($event)"
            placeholder="1.0.0"
          />
        </NFormItem>

        <NFormItem label="描述">
          <NInput
            :value="store.state.description"
            @update:value="store.setDescription($event)"
            placeholder="可选，对应属性-详细信息-文件说明"
          />
        </NFormItem>
        <div class="hint">写入 exe 的 FileDescription（文件说明）</div>

        <NFormItem label="公司/组织">
          <NInput
            :value="store.state.company"
            @update:value="store.setCompany($event)"
            placeholder="可选，对应属性-详细信息-版权"
          />
        </NFormItem>
        <div class="hint">写入公司名，并自动生成版权 Copyright (c) 年份 公司名</div>

        <NFormItem label="应用图标">
          <div class="icon-row">
            <div v-if="store.state.iconPreview" class="icon-preview">
              <NImage :src="store.state.iconPreview" width="64" height="64" object-fit="contain" />
            </div>
            <div v-else class="icon-placeholder">默认图标</div>
            <NSpace>
              <NButton @click="selectIcon">选择图标</NButton>
              <NButton v-if="store.state.iconPath" @click="clearIcon">清除</NButton>
            </NSpace>
          </div>
        </NFormItem>
        <div class="hint">支持 .png（正方形，建议 ≥256×256）或 .ico</div>

        <NDivider>发布平台</NDivider>

        <div class="platform-row">
          <div class="platform-item active">
            <span class="platform-check">✓</span>Windows
            <span class="platform-note">当前支持</span>
          </div>
          <div class="platform-item soon">macOS<span class="platform-note">即将支持</span></div>
          <div class="platform-item soon">Linux<span class="platform-note">即将支持</span></div>
        </div>
        <div class="hint">当前版本仅支持打包 Windows 应用；此处为后续平台拓展预留。</div>

        <NDivider>窗口设置</NDivider>

        <NFormItem label="窗口标题">
          <NInput
            :value="store.state.windowTitle"
            @update:value="store.setWindowTitle($event)"
            placeholder="留空则使用应用名称"
            maxlength="100"
          />
        </NFormItem>

        <div class="window-grid">
          <NFormItem label="宽度">
            <NInputNumber
              :value="store.state.windowWidth"
              @update:value="store.setWindowWidth($event || 1024)"
              :min="400"
              :max="3840"
              :disabled="store.state.windowFullscreen || store.state.windowMaximized"
              style="width: 100%"
            />
          </NFormItem>
          <NFormItem label="高度">
            <NInputNumber
              :value="store.state.windowHeight"
              @update:value="store.setWindowHeight($event || 768)"
              :min="300"
              :max="2160"
              :disabled="store.state.windowFullscreen || store.state.windowMaximized"
              style="width: 100%"
            />
          </NFormItem>
        </div>

        <NSpace>
          <NFormItem label="最大化" label-placement="left">
            <NSwitch
              :value="store.state.windowMaximized"
              @update:value="store.setWindowMaximized($event)"
            />
          </NFormItem>
          <NFormItem label="全屏" label-placement="left">
            <NSwitch
              :value="store.state.windowFullscreen"
              @update:value="store.setWindowFullscreen($event)"
            />
          </NFormItem>
          <NFormItem label="关闭前确认" label-placement="left">
            <NSwitch
              :value="store.state.confirmClose"
              @update:value="store.setConfirmClose($event)"
            />
          </NFormItem>
        </NSpace>

        <NSpace>
          <NFormItem label="单实例锁" label-placement="left">
            <NSwitch
              :value="store.state.singleInstance"
              @update:value="store.setSingleInstance($event)"
            />
          </NFormItem>
          <NFormItem label="记住窗口位置" label-placement="left">
            <NSwitch
              :value="store.state.rememberWindow"
              @update:value="store.setRememberWindow($event)"
            />
          </NFormItem>
        </NSpace>
        <div class="hint">
          单实例锁：重复双击 exe 不会打开第二个窗口，而是把已有窗口带到前台；记住窗口位置：生成应用关闭时保存窗口大小和位置，下次启动恢复。
        </div>

        <NDivider>代码签名（可选）</NDivider>

        <NFormItem label="签名证书 (PFX)">
          <div class="icon-row">
            <NInput
              :value="store.state.signPfxPath"
              readonly
              placeholder="未选择，构建后不生成签名脚本"
              size="small"
              style="flex: 1"
            />
            <NSpace>
              <NButton size="small" @click="selectPfx">选择证书</NButton>
              <NButton v-if="store.state.signPfxPath" size="small" @click="clearPfx">清除</NButton>
            </NSpace>
          </div>
        </NFormItem>
        <NFormItem label="时间戳服务器">
          <NInput
            :value="store.state.signTimestamp"
            @update:value="store.setSignTimestamp($event)"
            placeholder="可选，如 http://timestamp.digicert.com"
          />
        </NFormItem>
        <div class="hint">
          构建完成后会在输出 exe 旁生成同名的 <code>-sign.cmd</code> 签名脚本，双击运行并输入证书密码即可完成签名；密码不会保存。
        </div>
      </NForm>

      <div class="form-actions">
        <NSpace justify="space-between">
          <NButton @click="prevStep">上一步</NButton>
          <NButton type="primary" @click="nextStep">下一步</NButton>
        </NSpace>
      </div>
    </NCard>
  </div>
</template>

<style scoped>
.step-container {
  padding: 32px;
  max-width: 680px;
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
.icon-row {
  display: flex;
  align-items: center;
  gap: 16px;
  width: 100%;
}
.icon-preview {
  width: 64px;
  height: 64px;
  border: 1px solid #eee;
  border-radius: 8px;
  overflow: hidden;
  background: #fff;
}
.icon-placeholder {
  width: 64px;
  height: 64px;
  border: 1px dashed #ccc;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #999;
  font-size: 12px;
}
.hint {
  display: block;
  margin: 2px 0 20px;
  color: #909399;
  font-size: 12px;
  line-height: 1.7;
  padding-left: 2px;
}
.window-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}
.platform-row {
  display: flex;
  gap: 12px;
  margin-bottom: 20px;
}

.platform-item {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 10px 18px;
  border-radius: 10px;
  border: 1px solid #e5e6eb;
  background: #fafbfc;
  font-size: 13px;
  color: #4e5358;
}

.platform-item.active {
  border-color: #18a058;
  background: #f0faf4;
  color: #18a058;
  font-weight: 600;
}

.platform-item.active .platform-check {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  border-radius: 50%;
  background: #18a058;
  color: #fff;
  font-size: 11px;
}

.platform-item.soon {
  color: #b0b6bf;
  background: #fafbfc;
}

.platform-note {
  font-size: 11px;
  color: #9aa0a8;
  font-weight: 400;
}
.form-actions {
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid #eee;
}
</style>