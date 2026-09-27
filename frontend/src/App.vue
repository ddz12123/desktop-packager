<script lang="ts" setup>
import {ref} from 'vue'
import {NButton, NConfigProvider, NDialogProvider, NMessageProvider, NSpace, NTooltip, zhCN, dateZhCN} from 'naive-ui'
import {useStore} from './store'
import {canEnterStep} from './validation'
import StepImport from './components/StepImport.vue'
import StepSettings from './components/StepSettings.vue'
import StepProxy from './components/StepProxy.vue'
import StepBuild from './components/StepBuild.vue'
import GlobalPanels from './components/GlobalPanels.vue'

const store = useStore()
const panels = ref<InstanceType<typeof GlobalPanels>>()

const steps = [
  {label: '导入构建产物', desc: '选择 dist 文件夹或 ZIP'},
  {label: '应用配置', desc: '设置名称和图标'},
  {label: '反向代理', desc: '配置 nginx 风格代理'},
  {label: '构建生成', desc: '生成桌面应用'},
]

function stepDisabled(index: number) {
  return index > store.state.currentStep
    && !canEnterStep(index, {
      distPath: store.state.distPath,
      appName: store.state.appName,
    })
}

function handleStepClick(index: number) {
  // Allow going back freely; forward only when prerequisites are met.
  if (stepDisabled(index)) return
  store.setCurrentStep(index)
}
</script>

<template>
  <NConfigProvider :locale="zhCN" :date-locale="dateZhCN">
    <NMessageProvider>
      <NDialogProvider>
        <div class="app-layout">
          <div class="sidebar">
            <div class="sidebar-header">
              <h2>应用生成器</h2>
              <NSpace :size="4" :wrap="false">
                <NTooltip trigger="hover">
                  <template #trigger>
                    <NButton
                      class="header-icon-button"
                      quaternary
                      circle
                      size="small"
                      aria-label="构建方案"
                      @click="panels?.openProfiles()"
                    >
                      <svg viewBox="0 0 24 24" aria-hidden="true">
                        <path d="M3 5.5A2.5 2.5 0 0 1 5.5 3h4.09c.66 0 1.3.26 1.77.74L12.8 5.2c.28.28.66.44 1.06.44h4.64A2.5 2.5 0 0 1 21 8.14v10.36a2.5 2.5 0 0 1-2.5 2.5h-13A2.5 2.5 0 0 1 3 18.5v-13Zm2.5-.5a.5.5 0 0 0-.5.5v13c0 .28.22.5.5.5h13a.5.5 0 0 0 .5-.5V8.14a.5.5 0 0 0-.5-.5h-4.64c-.93 0-1.82-.37-2.48-1.02l-1.43-1.46a.5.5 0 0 0-.36-.16H5.5Z" />
                      </svg>
                    </NButton>
                  </template>
                  构建方案
                </NTooltip>
                <NTooltip trigger="hover">
                  <template #trigger>
                    <NButton
                      class="header-icon-button"
                      quaternary
                      circle
                      size="small"
                      aria-label="设置"
                      @click="panels?.openSettings()"
                    >
                      <svg viewBox="0 0 24 24" aria-hidden="true">
                        <path
                          d="M12 15.5A3.5 3.5 0 1 0 12 8a3.5 3.5 0 0 0 0 7.5Zm7.2-3.5c0-.4 0-.8-.1-1.2l2-1.5-2-3.4-2.4 1a8 8 0 0 0-2-1.2L14.4 3h-4.8l-.4 2.7a8 8 0 0 0-2 1.2l-2.4-1-2 3.4 2 1.5a8.2 8.2 0 0 0 0 2.4l-2 1.5 2 3.4 2.4-1a8 8 0 0 0 2 1.2l.4 2.7h4.8l.4-2.7a8 8 0 0 0 2-1.2l2.4 1 2-3.4-2-1.5c.1-.4.1-.8.1-1.2Z"
                        />
                      </svg>
                    </NButton>
                  </template>
                  设置
                </NTooltip>
              </NSpace>
            </div>
            <div class="step-list">
              <div
                v-for="(step, index) in steps"
                :key="index"
                class="step-item"
                :class="{
                  active: store.state.currentStep === index,
                  completed: index < store.state.currentStep,
                  disabled: stepDisabled(index)
                }"
                role="button"
                :tabindex="stepDisabled(index) ? -1 : 0"
                :aria-disabled="stepDisabled(index)"
                @click="handleStepClick(index)"
                @keydown.enter.prevent="handleStepClick(index)"
                @keydown.space.prevent="handleStepClick(index)"
              >
                <div class="step-number">
                  <span v-if="index < store.state.currentStep">✓</span>
                  <span v-else>{{ index + 1 }}</span>
                </div>
                <div class="step-info">
                  <div class="step-label">{{ step.label }}</div>
                  <div class="step-desc">{{ step.desc }}</div>
                </div>
              </div>
            </div>
            <div class="sidebar-footer">
              <div class="sidebar-tip">支持把 dist 文件夹或 ZIP 直接拖入窗口导入</div>
            </div>
          </div>

          <div class="main-content">
            <StepImport v-if="store.state.currentStep === 0" />
            <StepSettings v-else-if="store.state.currentStep === 1" />
            <StepProxy v-else-if="store.state.currentStep === 2" />
            <StepBuild v-else-if="store.state.currentStep === 3" />
          </div>
        </div>

        <GlobalPanels ref="panels" />
      </NDialogProvider>
    </NMessageProvider>
  </NConfigProvider>
</template>

<style scoped>
.app-layout {
  display: flex;
  height: 100vh;
  overflow: hidden;
}

.sidebar {
  width: 240px;
  min-width: 240px;
  background: #1e1e2e;
  color: #cdd6f4;
  display: flex;
  flex-direction: column;
  user-select: none;
}

.sidebar-header {
  padding: 18px 14px 14px 16px;
  border-bottom: 1px solid #313244;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.sidebar-header h2 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: #cdd6f4;
}

.header-icon-button {
  color: #cdd6f4;
  flex-shrink: 0;
}

.header-icon-button svg {
  width: 17px;
  height: 17px;
  fill: currentColor;
}

.step-list {
  padding: 12px 8px;
  flex: 1;
}

.step-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s;
  margin-bottom: 4px;
}

.step-item:hover {
  background: #313244;
}

.step-item.active {
  background: #45475a;
}

.step-item.disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.step-item.completed .step-number {
  background: #a6e3a1;
  color: #1e1e2e;
}

.step-number {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  background: #585b70;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 13px;
  font-weight: 600;
  flex-shrink: 0;
}

.step-item.active .step-number {
  background: #89b4fa;
  color: #1e1e2e;
}

.step-info {
  flex: 1;
  min-width: 0;
}

.step-label {
  font-size: 14px;
  font-weight: 500;
  color: #cdd6f4;
}

.step-desc {
  font-size: 12px;
  color: #6c7086;
  margin-top: 2px;
}

.sidebar-footer {
  padding: 12px 16px;
  border-top: 1px solid #313244;
}

.sidebar-tip {
  font-size: 12px;
  color: #6c7086;
  line-height: 1.6;
}

.main-content {
  flex: 1;
  overflow-y: auto;
  background: #f8f9fa;
}
</style>
