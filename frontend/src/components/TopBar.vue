<script setup lang="ts">
import { computed } from 'vue'
import { Setting, Folder } from '@element-plus/icons-vue'
import type { Repository } from '../types'

const props = defineProps<{
  repositories: Repository[]
  activeRepoId: string
}>()

const emit = defineEmits(['open-settings'])
const activeRepository = computed(() =>
  props.repositories.find(repo => repo.id === props.activeRepoId) ?? null
)
</script>

<template>
  <header class="top-bar">
    <div class="repository-heading">
      <el-icon color="#e6a23c"><Folder /></el-icon> 
      <div class="repository-identity">
        <strong>{{ activeRepository?.name || 'Maven Manager' }}</strong>
        <span v-if="activeRepository" class="repository-path" :title="activeRepository.path">
          {{ activeRepository.path }}
        </span>
        <span v-else class="repository-path">尚未选择仓库</span>
      </div>
    </div>
    <div class="actions">
      <el-button 
        size="small" 
        :icon="Setting" 
        @click="emit('open-settings')"
        title="系统设置"
        aria-label="系统设置"
        class="settings-button"
      />
    </div>
  </header>
</template>

<style scoped>
.top-bar { 
  height: 40px; display: flex; justify-content: space-between; align-items: center; 
  padding: 0 12px; background: #fff; border-bottom: 1px solid #dcdfe6; flex-shrink: 0; 
  box-shadow: 0 1px 2px rgba(0,0,0,0.02);
  --wails-draggable: drag;
}
.repository-heading { display: flex; align-items: center; gap: 8px; min-width: 0; color: #303133; }
.repository-identity { display: flex; align-items: baseline; gap: 10px; min-width: 0; }
.repository-identity strong { flex-shrink: 0; font-size: 14px; font-weight: 600; }
.repository-path { overflow: hidden; color: #909399; font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.actions { display: flex; align-items: center; gap: 8px; }
.settings-button { --wails-draggable: no-drag; }
</style>