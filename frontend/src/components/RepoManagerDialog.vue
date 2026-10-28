<script setup lang="ts">
import { ref, watch } from 'vue'
import { Delete, Plus } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { ConfigService } from '../../bindings/mvnmanager/services'
import type { AppConfig, Repository } from '../types'

const props = defineProps<{
  visible: boolean
  config: AppConfig | null
}>()

const emit = defineEmits(['update:visible', 'save'])

const localRepos = ref<Repository[]>([])

watch(() => props.visible, (val) => {
  if (val && props.config) {
    localRepos.value = JSON.parse(JSON.stringify(props.config.repositories))
  }
})

const addRepo = async () => {
  try {
    const id = await ConfigService.NewRepositoryID()
    localRepos.value.push({ id, name: '新仓库', path: '' })
  } catch (error) {
    ElMessage.error(`生成仓库 ID 失败：${String(error)}`)
  }
}

const removeRepo = (index: number) => {
  localRepos.value.splice(index, 1)
}

const handleSave = () => {
  // 简单校验
  const hasEmpty = localRepos.value.some(r => !r.name || !r.path)
  if (hasEmpty) {
    ElMessage.warning('请填写完整的仓库名称和路径')
    return
  }

  const newConfig = { ...props.config!, repositories: localRepos.value }
  emit('save', newConfig)
  emit('update:visible', false)
}
</script>

<template>
  <el-dialog 
    :model-value="visible" 
    @update:model-value="(val: any) => emit('update:visible', val)"
    title="本地仓库管理" 
    width="650px"
    align-center
  >
    <div class="repo-manager">
      <div class="header">
        <span class="tip">在这里维护你的本地 Maven 仓库路径。</span>
        <el-button size="small" type="primary" plain :icon="Plus" @click="addRepo">新增仓库</el-button>
      </div>
      
      <div class="repo-list">
        <div v-for="(repo, index) in localRepos" :key="repo.id" class="repo-item">
          <div class="input-group">
            <span class="label">名称</span>
            <el-input v-model="repo.name" placeholder="例如: 默认本地库" size="small" />
          </div>
          <div class="input-group flex-1">
            <span class="label">路径</span>
            <el-input v-model="repo.path" placeholder="例如: C:\Users\xxx\.m2\repository" size="small" />
          </div>
          <el-button type="danger" plain size="small" :icon="Delete" @click="removeRepo(index)" />
        </div>
        <div v-if="localRepos.length === 0" class="empty">
          <el-empty description="暂无仓库，请点击右上角新增" :image-size="60" />
        </div>
      </div>
    </div>

    <template #footer>
      <span class="dialog-footer">
        <el-button @click="emit('update:visible', false)">取消</el-button>
        <el-button type="primary" @click="handleSave">保存配置</el-button>
      </span>
    </template>
  </el-dialog>
</template>

<style scoped>
.repo-manager { display: flex; flex-direction: column; gap: 12px; }
.header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; }
.tip { font-size: 13px; color: #909399; }
.repo-list { 
  max-height: 400px; 
  overflow-y: auto; 
  display: flex; 
  flex-direction: column; 
  gap: 12px; 
  padding-right: 4px;
}
.repo-item {
  display: flex;
  align-items: center;
  gap: 12px;
  background: #fafafa;
  border: 1px solid #ebeef5;
  padding: 12px;
  border-radius: 4px;
}
.input-group { display: flex; flex-direction: column; gap: 6px; width: 160px; }
.input-group.flex-1 { flex: 1; width: auto; }
.label { font-size: 12px; color: #606266; font-weight: 600; }
.empty { padding: 30px 0; }
</style>