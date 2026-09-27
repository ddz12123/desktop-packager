import {defineConfig, type Plugin} from 'vite'
import vue from '@vitejs/plugin-vue'
import {resolve} from 'path'
import {writeFileSync} from 'fs'

// vite 清空 outDir 时会删掉 dist/.gitkeep；该占位文件用于让
// 主工程的 go:embed all:frontend/dist 在未构建前端时也能编译，构建结束后补回。
function preserveGitkeep(): Plugin {
  return {
    name: 'preserve-dist-gitkeep',
    closeBundle() {
      try {
        writeFileSync(resolve(__dirname, 'dist/.gitkeep'), '')
      } catch {
        // 忽略：正常使用时不影响构建产物
      }
    },
  }
}

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [vue(), preserveGitkeep()],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
    },
  },
  server: {
    port: 5173,
    strictPort: true,
  },
})
