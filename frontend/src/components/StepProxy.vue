<script lang="ts" setup>
import {ref} from 'vue'
import {NCard, NButton, NSpace, NInput, NSwitch, NEmpty, NAlert, NTooltip, useMessage} from 'naive-ui'
import {useStore} from '../store'
import {validateProxyRule} from '../validation'
import {TestProxyTarget} from '../../wailsjs/go/main/App'

const store = useStore()
const message = useMessage()
const error = ref('')
const testing = ref<Record<number, boolean>>({})

function addRule() {
  store.addProxyRule()
}

function removeRule(index: number) {
  store.removeProxyRule(index)
}

async function testRule(index: number) {
  const rule = store.state.proxyRules[index]
  const target = (rule.target || '').trim()
  if (!target) {
    message.warning('请先填写目标地址')
    return
  }
  error.value = ''
  testing.value[index] = true
  try {
    const status = await TestProxyTarget(target)
    message.success(`规则 #${index + 1} 连通正常（${status}）`)
  } catch (e: any) {
    message.error(`规则 #${index + 1} 连接失败：${e?.message || String(e)}`)
  } finally {
    testing.value[index] = false
  }
}

function prevStep() {
  store.setCurrentStep(1)
}

function nextStep() {
  error.value = ''
  for (let i = 0; i < store.state.proxyRules.length; i++) {
    const msg = validateProxyRule(store.state.proxyRules[i], i)
    if (msg) {
      error.value = msg
      return
    }
  }
  store.setCurrentStep(3)
}
</script>

<template>
  <div class="step-container">
    <div class="step-title">
      <h3>反向代理配置</h3>
      <p>按 nginx location + proxy_pass 语义配置，解决跨域与本地转发</p>
    </div>

    <NAlert v-if="error" type="error" closable @close="error = ''" style="margin-bottom: 16px">
      {{ error }}
    </NAlert>

    <NCard>
      <div v-if="store.state.proxyRules.length === 0" class="empty-state">
        <NEmpty description="暂无代理规则">
          <template #extra>
            <NButton type="primary" @click="addRule" dashed>
              + 添加代理规则
            </NButton>
          </template>
        </NEmpty>
      </div>

      <div v-else class="rule-list">
        <div class="rule-header">
          <span class="col-path">路径前缀 (location)</span>
          <span class="col-target">目标地址 (proxy_pass)</span>
          <span class="col-rewrite">重写为（可选）</span>
          <span class="col-enabled">启用</span>
          <span class="col-action">操作</span>
        </div>

        <div
          v-for="(rule, index) in store.state.proxyRules"
          :key="index"
          class="rule-row"
        >
          <div class="col-path">
            <NInput
              :value="rule.path"
              @update:value="store.updateProxyRule(index, {path: $event})"
              placeholder="/api/"
              size="small"
            />
          </div>
          <div class="col-target">
            <NInput
              :value="rule.target"
              @update:value="store.updateProxyRule(index, {target: $event})"
              placeholder="http://localhost:8080/"
              size="small"
            />
          </div>
          <div class="col-rewrite">
            <NTooltip trigger="hover">
              <template #trigger>
                <NInput
                  :value="rule.rewrite"
                  @update:value="store.updateProxyRule(index, {rewrite: $event})"
                  placeholder="可选 /v2"
                  size="small"
                />
              </template>
              非空时作为 proxy_pass 的 URI 替换前缀，覆盖目标地址中的路径部分
            </NTooltip>
          </div>
          <div class="col-enabled">
            <NSwitch
              :value="rule.enabled"
              @update:value="store.updateProxyRule(index, {enabled: $event})"
              size="small"
            />
          </div>
          <div class="col-action">
            <NSpace :size="4" :wrap="false">
              <NButton text type="primary" size="small" :loading="testing[index]" @click="testRule(index)">
                测试
              </NButton>
              <NButton text type="error" size="small" @click="removeRule(index)">
                删除
              </NButton>
            </NSpace>
          </div>
        </div>

        <div class="add-rule">
          <NButton type="primary" dashed @click="addRule" size="small">
            + 添加规则
          </NButton>
        </div>
      </div>

      <NAlert type="info" style="margin-top: 16px" title="nginx 兼容说明">
        <p style="margin: 0; font-size: 13px; line-height: 1.7;">
          对应关系：<code>路径前缀</code> = <code>location</code>，<code>目标地址</code> = <code>proxy_pass</code>。<br/>
          • 目标 <code>http://host:8080/</code>（含 URI）→ 剥离 location 前缀。例：<code>/api/users</code> → <code>/users</code><br/>
          • 目标 <code>http://host:8080</code>（无 URI）→ 保留完整路径。例：<code>/api/users</code> → <code>/api/users</code><br/>
          • 目标 <code>http://host:8080/v2/</code> → 前缀替换为 <code>/v2/</code>。例：<code>/api/users</code> → <code>/v2/users</code><br/>
          • 填写“重写为”时，等价于自定义 proxy_pass URI，优先级高于目标地址中的路径<br/>
          • 自动设置 Host / X-Forwarded-*；支持 WebSocket Upgrade
        </p>
      </NAlert>

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
  max-width: 900px;
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
.empty-state {
  padding: 60px 0;
  text-align: center;
}
.rule-list {
  width: 100%;
}
.rule-header {
  display: flex;
  gap: 8px;
  padding: 8px 0;
  border-bottom: 2px solid #eee;
  font-size: 13px;
  font-weight: 600;
  color: #666;
}
.rule-row {
  display: flex;
  gap: 8px;
  padding: 8px 0;
  border-bottom: 1px solid #f0f0f0;
  align-items: center;
}
.col-path { flex: 1.2; }
.col-target { flex: 2; }
.col-rewrite { flex: 1.2; }
.col-enabled { width: 50px; text-align: center; }
.col-action { width: 92px; text-align: center; }
.add-rule {
  padding: 12px 0;
}
.form-actions {
  margin-top: 24px;
  padding-top: 16px;
  border-top: 1px solid #eee;
}
code {
  background: #f5f5f5;
  padding: 2px 6px;
  border-radius: 4px;
}
</style>