<script setup lang="ts">
import { computed } from 'vue'
import { Folder } from '@element-plus/icons-vue'

const props = defineProps<{
  visible: boolean
  isScanning: boolean
  scannedCount: number
  totalFileCount: number
  scanPhase: string
  currentPath: string
}>()

const percentage = computed(() => {
  if (props.scanPhase !== 'processing' || props.totalFileCount === 0) return 0
  return Math.min(100, Math.floor(props.scannedCount / props.totalFileCount * 100))
})
</script>

<template>
  <el-dialog 
    :model-value="visible" 
    title="正在扫描本地仓库" 
    width="500px" 
    :close-on-click-modal="false" 
    :close-on-press-escape="false" 
    :show-close="false" 
    align-center
  >
    <div class="scan-dialog-content">
      <el-progress 
        :percentage="percentage"
        :indeterminate="scanPhase === 'counting'"
        :duration="3"
        :show-text="true"
        :status="isScanning ? '' : 'success'" 
        stroke-width="12" 
      />
      <div class="scan-stats">
        <span v-if="scanPhase === 'counting'">
          正在统计文件总数：<strong>{{ scannedCount.toLocaleString() }}</strong>
        </span>
        <span v-else>
          已处理：<strong>{{ scannedCount.toLocaleString() }}</strong> /
          <strong>{{ totalFileCount.toLocaleString() }}</strong> 个文件
        </span>
      </div>
      <div class="scan-path-box">
        <el-icon><Folder /></el-icon>
        <span class="path-text" :title="currentPath">{{ currentPath }}</span>
      </div>
    </div>
  </el-dialog>
</template>

<style scoped>
.scan-dialog-content { display: flex; flex-direction: column; gap: 20px; padding: 10px 0; }
.scan-stats { font-size: 14px; color: #606266; text-align: center; }
.scan-stats strong { color: #409EFF; font-size: 18px; margin: 0 4px; }
.scan-path-box { 
  display: flex; align-items: center; gap: 8px; background: #f4f4f5; 
  padding: 10px 14px; border-radius: 4px; color: #909399; font-family: monospace; font-size: 13px; 
}
.path-text { flex: 1; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
</style>