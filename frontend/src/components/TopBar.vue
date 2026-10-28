<script setup lang="ts">
import { Refresh, Setting, Folder, Brush, Management } from '@element-plus/icons-vue'
import type { Repository } from '../types'

defineProps<{
  repositories: Repository[]
  activeRepoId: string
  isScanning: boolean
}>()

const emit = defineEmits(['update:activeRepoId', 'scan', 'open-settings', 'open-cleanup', 'open-repo-manager'])

const handleRepoChange = (val: string) => {
  emit('update:activeRepoId', val)
}
</script>

<template>
  <header class="top-bar">
    <div class="logo">
      <el-icon color="#e6a23c"><Folder /></el-icon> 
      <span>Maven Manager</span>
    </div>
    <div class="actions">
      <!-- 顶部下拉框选择仓库 -->
      <el-select 
        :model-value="activeRepoId" 
        @update:model-value="handleRepoChange"
        style="width: 280px; margin-right: 10px;"
        size="small"
        placeholder="请选择要管理的仓库"
        :disabled="isScanning"
      >
        <el-option 
          v-for="repo in repositories" 
          :key="repo.id" 
          :label="repo.name + ' (' + repo.path + ')'" 
          :value="repo.id" 
        />
      </el-select>

      <el-button 
        type="warning" 
        size="small" 
        :icon="Brush" 
        plain
        @click="emit('open-cleanup')"
      >
        清理工具
      </el-button>
      
      <el-button 
        type="primary" 
        size="small" 
        :icon="Refresh" 
        @click="emit('scan')" 
        :loading="isScanning"
        plain
      >
        {{ isScanning ? '加载中...' : '重新加载' }}
      </el-button>

      <div class="divider"></div>

      <el-button 
        size="small" 
        :icon="Management" 
        @click="emit('open-repo-manager')"
        title="仓库管理"
      />
      
      <el-button 
        size="small" 
        :icon="Setting" 
        @click="emit('open-settings')"
        title="系统设置"
      />
    </div>
  </header>
</template>

<style scoped>
.top-bar { 
  height: 40px; display: flex; justify-content: space-between; align-items: center; 
  padding: 0 12px; background: #fff; border-bottom: 1px solid #dcdfe6; flex-shrink: 0; 
  box-shadow: 0 1px 2px rgba(0,0,0,0.02);
}
.logo { font-weight: 600; font-size: 14px; display: flex; align-items: center; gap: 6px; color: #303133; }
.actions { display: flex; align-items: center; gap: 8px; }
.divider { width: 1px; height: 16px; background: #dcdfe6; margin: 0 4px; }
</style>