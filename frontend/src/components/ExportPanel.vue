<script setup lang="ts">
import { ref } from 'vue'
import { Delete, Download } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { RepoService } from '../../bindings/mvnmanager/services'
import type { ArtifactCoordinate } from '../types'

const props = defineProps<{
  repoPath: string
  artifacts: ArtifactCoordinate[]
}>()

const emit = defineEmits<{
  remove: [artifact: ArtifactCoordinate]
  clear: []
}>()

const scriptType = ref<'bat' | 'sh'>('bat')
const isExporting = ref(false)

const exportArtifacts = async () => {
  if (!props.artifacts.length || isExporting.value) return
  isExporting.value = true
  try {
    const outputPath = await RepoService.ExportArtifactVersions(
      props.repoPath,
      props.artifacts,
      scriptType.value
    )
    if (outputPath) ElMessage.success(`依赖包已导出到 ${outputPath}`)
  } catch (error) {
    ElMessage.error(`导出依赖包失败：${String(error)}`)
  } finally {
    isExporting.value = false
  }
}
</script>

<template>
  <div class="export-panel">
    <div class="export-toolbar">
      <div class="script-options">
        <span>生成安装脚本：</span>
        <el-radio-group v-model="scriptType" size="small">
          <el-radio-button value="bat">Windows (.bat)</el-radio-button>
          <el-radio-button value="sh">Linux / macOS (.sh)</el-radio-button>
        </el-radio-group>
        <el-tooltip content="ZIP 保留 Maven 仓库目录结构，可解压到另一个仓库根目录；也可运行生成的脚本导入本地 Maven 仓库。" placement="top">
          <el-text type="info" size="small">支持直接复制或脚本导入</el-text>
        </el-tooltip>
      </div>
      <div class="export-actions">
        <span class="artifact-count">已添加 {{ artifacts.length }} 个版本</span>
        <el-button :disabled="!artifacts.length || isExporting" @click="emit('clear')">清空列表</el-button>
        <el-button
          type="primary"
          :icon="Download"
          :loading="isExporting"
          :disabled="!artifacts.length"
          @click="exportArtifacts"
        >
          统一导出
        </el-button>
      </div>
    </div>

    <el-table v-if="artifacts.length" :data="artifacts" height="100%" class="export-table">
      <el-table-column label="包名" min-width="240">
        <template #default="scope">
          {{ scope.row.groupId }}:{{ scope.row.artifactId }}
        </template>
      </el-table-column>
      <el-table-column prop="version" label="版本" width="180" />
      <el-table-column label="操作" width="90" align="center">
        <template #default="scope">
          <el-button
            link
            type="danger"
            :icon="Delete"
            aria-label="从导出列表移除"
            @click="emit('remove', scope.row)"
          />
        </template>
      </el-table-column>
    </el-table>
    <el-empty v-else description="请在包详情的版本操作列中添加要导出的依赖版本" />
  </div>
</template>

<style scoped>
.export-panel {
  display: flex;
  flex-direction: column;
  gap: 12px;
  height: 100%;
  padding: 16px;
  min-width: 0;
}

.export-toolbar,
.script-options,
.export-actions {
  display: flex;
  align-items: center;
}

.export-toolbar {
  justify-content: space-between;
  gap: 16px;
  min-height: 36px;
  flex-wrap: wrap;
}

.script-options,
.export-actions {
  gap: 10px;
}

.script-options {
  color: var(--el-text-color-regular);
  font-size: 13px;
}

.export-actions {
  margin-left: auto;
}

.artifact-count {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.export-table {
  width: 100%;
  flex: 1;
}
</style>
