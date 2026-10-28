<script setup lang="ts">
import { ref } from 'vue'
import { Close, Grid, DocumentCopy, CopyDocument, Delete } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import CleanupPanel from './CleanupPanel.vue'
import Dashboard from './Dashboard.vue'
import type { RemoteRepository, Repository, RepositoryInfo, RepositoryOverview, TabItem, VersionGroup } from '../types'

const props = defineProps<{
  openedTabs: TabItem[]
  activeTab: string
  tabDataMap: Record<string, VersionGroup[]>
  selectedVersionMap: Record<string, VersionGroup>
  repoPath: string
  mavenPath: string
  remoteRepositories: RemoteRepository[]
  repositories: Repository[]
  repositoryInfos: Record<string, RepositoryInfo | undefined>
  activeRepoId: string
  overview: RepositoryOverview | null
  isScanning: boolean
}>()

const emit = defineEmits([
  'update:activeTab', 'tab-change', 'tab-remove', 
  'row-click', 'close-detail', 'cleanup-done', 'cleanup-open-artifact',
  'select-repository', 'scan', 'open-cleanup', 'open-repo-manager'
])

const tableRefs = ref<Record<string, any>>({})
const setTableRef = (el: any, tabId: string) => {
  if (el) tableRefs.value[tabId] = el
}

const formatSize = (bytes: number) => {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

const getDependencyXml = (tab: TabItem, version: string) => {
  return `<dependency>\n    <groupId>${tab.groupId}</groupId>\n    <artifactId>${tab.artifactId}</artifactId>\n    <version>${version}</version>\n</dependency>`
}

const copyDependency = async (tab: TabItem, version: string) => {
  const xml = getDependencyXml(tab, version)
  try {
    await navigator.clipboard.writeText(xml)
    ElMessage.success('已复制到剪贴板')
  } catch {
    ElMessage.error('复制失败，请手动选中复制')
  }
}

const bottomPaneHeight = ref(300)
const isResizingY = ref(false)

const startResizeY = (e: MouseEvent) => {
  e.preventDefault()
  isResizingY.value = true
  document.body.style.cursor = 'row-resize'
  document.body.style.userSelect = 'none'

  const startY = e.clientY
  const startHeight = bottomPaneHeight.value

  const onMouseMove = (moveEvent: MouseEvent) => {
    moveEvent.preventDefault()
    const deltaY = startY - moveEvent.clientY
    const newHeight = Math.max(200, Math.min(800, startHeight + deltaY))
    bottomPaneHeight.value = newHeight
  }

  const onMouseUp = () => {
    isResizingY.value = false
    document.body.style.cursor = ''
    document.body.style.userSelect = ''
    document.removeEventListener('mousemove', onMouseMove)
    document.removeEventListener('mouseup', onMouseUp)
  }

  document.addEventListener('mousemove', onMouseMove)
  document.addEventListener('mouseup', onMouseUp)
}

defineExpose({
  clearHighlight: (tabId: string) => {
    const table = tableRefs.value[tabId]
    if (table) table.setCurrentRow()
  },
  setCurrentRow: (tabId: string, row: VersionGroup) => {
    const table = tableRefs.value[tabId]
    if (table) table.setCurrentRow(row)
  }
})
</script>

<template>
  <main class="content">
    <div v-if="openedTabs.length === 0" class="empty-state">
      <Dashboard
        :repositories="repositories"
        :repositoryInfos="repositoryInfos"
        :activeRepoId="activeRepoId"
        :overview="overview"
        :isScanning="isScanning"
        @select-repository="emit('select-repository', $event)"
        @scan="emit('scan')"
        @open-cleanup="emit('open-cleanup')"
        @open-repo-manager="emit('open-repo-manager')"
      />
    </div>

    <el-tabs 
      v-else
      :model-value="activeTab" 
      @update:model-value="(val: any) => emit('update:activeTab', val)"
      class="tabs-container" 
      @tab-change="(val: any) => emit('tab-change', val)" 
      @tab-remove="(val: any) => emit('tab-remove', val)"
    >
      <el-tab-pane 
        v-for="tab in openedTabs" 
        :key="tab.id" 
        :name="tab.id" 
        :closable="true"
      >
        <template #label>
          <span class="tab-title">
            <el-icon v-if="tab.type === 'cleanup'"><Delete /></el-icon>
            <el-icon v-else><Grid /></el-icon>
            {{ tab.title }}
          </span>
        </template>

        <!-- 清理面板 -->
        <div v-if="tab.type === 'cleanup'" class="tab-content-wrapper">
          <CleanupPanel
            :repoPath="repoPath"
            :mavenPath="mavenPath"
            :remoteRepositories="remoteRepositories"
            @cleanup-done="emit('cleanup-done')"
            @open-artifact="emit('cleanup-open-artifact', $event)"
          />
        </div>

        <!-- 依赖详情面板 -->
        <div v-else class="tab-content-wrapper">
          <div class="toolbar">
            <div class="breadcrumb">
              <span class="group-id">{{ tab.groupId }}</span>
              <span class="separator">/</span>
              <strong class="artifact-id">{{ tab.artifactId }}</strong> 
            </div>
          </div>
          
          <div class="split-container">
            <div class="top-pane" :class="{ 'full-height': !selectedVersionMap[tab.id] }">
              <el-table 
                :ref="(el) => setTableRef(el, tab.id)" 
                :data="tabDataMap[tab.id] || []" 
                height="100%" 
                highlight-current-row
                @current-change="(row: any) => emit('row-click', { tabId: tab.id, row })"
                class="custom-table"
              >
                <el-table-column prop="version" label="Version" sortable width="160" />
                <el-table-column label="Type" width="120">
                  <template #default="scope">
                    <el-tag v-if="scope.row.hasJar" size="small" type="primary" effect="plain" style="margin-right: 4px; border:none; background:#ecf5ff;">JAR</el-tag>
                    <el-tag v-if="scope.row.hasPom" size="small" type="info" effect="plain" style="border:none; background:#f4f4f5;">POM</el-tag>
                  </template>
                </el-table-column>
                <el-table-column label="Status" width="150">
                  <template #default="scope">
                    <span v-if="scope.row.hasLastUpdated" class="status-badge error">下载失效</span>
                    <span v-else-if="scope.row.hasRemote" class="status-badge success">Remote</span>
                    <span v-else class="status-badge normal">本地正常</span>
                  </template>
                </el-table-column>
                <el-table-column label="Size" width="100" sortable :sort-method="(a:any, b:any) => a.totalSize - b.totalSize">
                  <template #default="scope">{{ formatSize(scope.row.totalSize) }}</template>
                </el-table-column>
                <el-table-column prop="latestModTime" label="Last Modified" sortable>
                  <template #default="scope">{{ new Date(scope.row.latestModTime).toLocaleString() }}</template>
                </el-table-column>
              </el-table>
            </div>

            <div 
              v-if="selectedVersionMap[tab.id]" 
              class="horizontal-resizer" 
              :class="{ 'is-dragging': isResizingY }" 
              @mousedown="startResizeY"
            ></div>

            <div 
              class="bottom-pane" 
              v-if="selectedVersionMap[tab.id]"
              :style="{ height: bottomPaneHeight + 'px' }"
            >
              <div class="pane-header">
                <div class="header-left-part">
                  <el-icon><DocumentCopy /></el-icon>
                  <span>文件明细 - {{ selectedVersionMap[tab.id].version }}</span>
                </div>
                
                <div class="header-right-part">
                  <span class="config-title">Maven 依赖配置</span>
                  <div class="header-actions">
                    <el-button 
                      size="small" 
                      type="primary" 
                      plain 
                      :icon="CopyDocument" 
                      @click="copyDependency(tab, selectedVersionMap[tab.id].version)"
                    >
                      复制
                    </el-button>
                    <el-button link :icon="Close" @click="emit('close-detail', tab.id)" class="close-btn" title="关闭" />
                  </div>
                </div>
              </div>
              
              <div class="pane-body">
                <div class="file-detail-left">
                  <el-table :data="selectedVersionMap[tab.id].files" height="100%" class="custom-table detail-table">
                    <el-table-column prop="fileName" label="File Name" />
                    <el-table-column label="Size" width="120" sortable :sort-method="(a:any, b:any) => a.size - b.size">
                      <template #default="scope">{{ formatSize(scope.row.size) }}</template>
                    </el-table-column>
                    <el-table-column prop="modTime" label="Last Modified" width="180">
                      <template #default="scope">{{ new Date(scope.row.modTime).toLocaleString() }}</template>
                    </el-table-column>
                  </el-table>
                </div>

                <div class="file-detail-right">
                  <div class="config-code-box">
                    <pre><code>{{ getDependencyXml(tab, selectedVersionMap[tab.id].version) }}</code></pre>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </el-tab-pane>
    </el-tabs>
  </main>
</template>

<style scoped>
.content { flex: 1; display: flex; flex-direction: column; background: #fff; min-width: 0; }
.empty-state { flex: 1; display: flex; justify-content: center; align-items: stretch; overflow: auto; background: #f9fafc; }

.tabs-container { flex: 1; display: flex; flex-direction: column; height: 100%; }
:deep(.el-tabs__header) { margin: 0; background: #f3f4f6; padding: 0; border-bottom: 1px solid #dcdfe6; height: 34px; box-sizing: border-box; }
:deep(.el-tabs__nav-wrap::after) { display: none; }
:deep(.el-tabs__active-bar) { display: none; }
:deep(.el-tabs__nav) { border: none !important; height: 34px; }
:deep(.el-tabs__item) { 
  height: 34px; line-height: 34px; font-size: 13px; color: #606266; 
  background: #f3f4f6; border-right: 1px solid #dcdfe6; 
  border-radius: 0; padding: 0 16px !important; margin: 0 !important; 
  box-shadow: inset 0 2px 0 transparent; transition: all 0.1s;
}
:deep(.el-tabs__item.is-active) { background: #fff; color: #303133; font-weight: 600; box-shadow: inset 0 2px 0 var(--el-color-primary); }
:deep(.el-tabs__item:hover) { color: #303133; background: #eef0f4; }
:deep(.el-tabs__content) { flex: 1; padding: 0; overflow: hidden; }
:deep(.el-tab-pane) { height: 100%; display: flex; flex-direction: column; }
.tab-title { display: flex; align-items: center; gap: 6px; }
:deep(.el-icon.is-icon-close) { margin-left: 8px; font-size: 12px; }
:deep(.el-icon.is-icon-close:hover) { background-color: #f56c6c; color: #fff; border-radius: 50%; }

.tab-content-wrapper { flex: 1; display: flex; flex-direction: column; height: 100%; }
.toolbar { padding: 8px 16px; background: #fff; border-bottom: 1px solid #ebeef5; display: flex; align-items: center; flex-shrink: 0; }
.breadcrumb { font-size: 13px; color: #606266; }
.breadcrumb .group-id { color: #909399; }
.breadcrumb .separator { margin: 0 6px; color: #c0c4cc; }
.breadcrumb .artifact-id { color: #303133; font-weight: 600; }

.split-container { flex: 1; display: flex; flex-direction: column; overflow: hidden; background: #fff; padding: 0; }

.top-pane { flex: 1; display: flex; flex-direction: column; min-height: 0; background: #fff; }

.horizontal-resizer { height: 1px; background: #ebeef5; cursor: row-resize; z-index: 10; flex-shrink: 0; --wails-draggable: no-drag; position: relative; margin: 0; width: 100%; }
.horizontal-resizer::before { content: ''; position: absolute; top: -4px; bottom: -4px; left: 0; right: 0; z-index: 11; }
.horizontal-resizer:hover, .horizontal-resizer.is-dragging { background: var(--el-color-primary); }

.bottom-pane { flex-shrink: 0; display: flex; flex-direction: column; background: #fff; border-top: none; z-index: 10; }

.pane-header { height: 36px; background: #fafafa; border-bottom: 1px solid #ebeef5; display: flex; flex-shrink: 0; }

.header-left-part {
  flex: 1; min-width: 0; display: flex; align-items: center; gap: 6px; padding: 0 12px;
  font-size: 13px; font-weight: 600; color: #606266;
}
.header-right-part {
  width: 380px; display: flex; justify-content: space-between; align-items: center; padding: 0 12px;
  border-left: 1px solid #ebeef5; background: #f5f7fa;
}
.config-title { font-size: 13px; font-weight: 600; color: #606266; }
.header-actions { display: flex; align-items: center; gap: 8px; }
.close-btn { color: #909399; }
.close-btn:hover { color: #f56c6c; }

.pane-body { flex: 1; display: flex; overflow: hidden; background: #fff; }

.file-detail-left { flex: 1; min-width: 0; display: flex; flex-direction: column; }
.file-detail-right { width: 380px; display: flex; flex-direction: column; background: #fafafa; border-left: 1px solid #ebeef5; }

.config-code-box {
  flex: 1; padding: 16px; overflow: auto;
  user-select: text !important; -webkit-user-select: text !important; --wails-draggable: no-drag;
}
.config-code-box pre { margin: 0; }
.config-code-box code {
  font-family: Consolas, Monaco, monospace; font-size: 13px; color: #c7254e;
  background-color: #f9f2f4; padding: 8px 12px; border-radius: 4px;
  line-height: 1.6; display: block; white-space: pre-wrap; border: 1px solid #f2d8e0; cursor: text;
}

:deep(.custom-table) { --el-table-border-color: #ebeef5; --el-table-header-bg-color: #fafafa; --el-table-row-hover-bg-color: #f5f7fa; }
:deep(.custom-table::before), :deep(.custom-table::after) { display: none !important; }
:deep(.custom-table th.el-table__cell) { padding: 6px 0; font-weight: 600; color: #909399; font-size: 12px; border-right: none; border-bottom: 1px solid #ebeef5; }
:deep(.custom-table td.el-table__cell) { padding: 6px 0; font-size: 13px; border-right: none; border-bottom: 1px solid #ebeef5; }
:deep(.custom-table .el-table__body tr.current-row > td.el-table__cell) { background-color: #fdf6ec !important; color: #303133; }
:deep(.custom-table .el-table__body tr:hover > td.el-table__cell) { background-color: #f5f7fa !important; }

.status-badge { font-size: 12px; padding: 2px 6px; border-radius: 2px; }
.status-badge.error { background: #fef0f0; color: #f56c6c; }
.status-badge.success { background: #f0f9eb; color: #67c23a; }
.status-badge.normal { background: #f4f4f5; color: #909399; }

:deep(.detail-table th.el-table__cell) { background: #fff; }
</style>