<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { RepoService } from '../../bindings/mvnmanager/services'
import { Events } from '@wailsio/runtime'
import { Delete, Loading } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { GarbageItem, RemoteRepository } from '../types'

const props = defineProps<{
  repoPath: string
  mavenPath: string
  remoteRepositories: RemoteRepository[]
}>()

const emit = defineEmits(['cleanup-done', 'open-artifact'])

const isScanning = ref(false)
const scanFinished = ref(false)
const isDeleting = ref(false)
const repairingItemId = ref('')
const moveToTrash = ref(true)
const results = ref<GarbageItem[]>([])
const selectedRows = ref<GarbageItem[]>([])
const resultsTableRef = ref<any>(null)
const scannedFiles = ref(0)
const totalFiles = ref(0)
const scanPhase = ref('counting')
const currentPath = ref('')
const unsubscribeEvents: Array<() => void> = []
const scanPercentage = computed(() => {
  if (scanPhase.value !== 'processing' || totalFiles.value === 0) return 0
  return Math.min(100, Math.floor(scannedFiles.value / totalFiles.value * 100))
})

const categoryOptions = [
  { key: 'lastUpdated', label: '下载失败标记', optionLabel: '下载失败标记 (.lastUpdated)' },
  { key: 'remote', label: '来源记录', optionLabel: '来源记录 (_remote.repositories)' },
  { key: 'inProgress', label: '中断残留', optionLabel: '中断残留 (*.in-progress)' },
  { key: 'zeroByte', label: '零字节文件', optionLabel: '零字节文件 (0B Jar/Pom)' },
  { key: 'emptyDir', label: '空文件夹', optionLabel: '空文件夹' },
  { key: 'missingJar', label: '缺失 JAR', optionLabel: 'POM 声明为 JAR 但缺少 JAR' }
] as const
type GarbageCategory = typeof categoryOptions[number]['key']
type CategoryFilter = 'all' | GarbageCategory

// 扫描选项
const options = ref({
  lastUpdated: true,
  remote: true,
  inProgress: true,
  zeroByte: true,
  emptyDir: true,
  missingJar: true
})
const scannedCategories = ref<GarbageCategory[]>([])
const activeCategory = ref<CategoryFilter>('all')
const hasSelectedOptions = computed(() => Object.values(options.value).some(Boolean))
const categoryCounts = computed(() =>
  categoryOptions
    .filter(({ key }) => scannedCategories.value.includes(key))
    .map(({ key, label }) => ({
      key,
      label,
      count: results.value.filter(item => item.category === key).length
    }))
    .filter(category => category.count > 0)
)
const filteredResults = computed(() =>
  activeCategory.value === 'all'
    ? results.value
    : results.value.filter(item => item.category === activeCategory.value)
)
const categoryLabel = (category: string) =>
  categoryOptions.find(option => option.key === category)?.label ?? category

const selectCategory = (category: CategoryFilter) => {
  if (activeCategory.value === category) return
  activeCategory.value = category
  resultsTableRef.value?.clearSelection()
  selectedRows.value = []
  totalSelectedSize.value = 0
}

// 计算属性：全选状态
const isAllSelected = computed(() => {
  return Object.values(options.value).every(v => v)
})

// 计算属性：是否有部分选中（用于半选状态显示）
const isIndeterminate = computed(() => {
  const vals = Object.values(options.value)
  const selectedCount = vals.filter(v => v).length
  return selectedCount > 0 && selectedCount < vals.length
})

// 全选/取消全选
const handleSelectAll = (val: any) => {
  const checked = !!val
  Object.keys(options.value).forEach(key => {
    options.value[key as keyof typeof options.value] = checked
  })
}

// 反选
const handleInvertSelect = () => {
  Object.keys(options.value).forEach(key => {
    options.value[key as keyof typeof options.value] = !options.value[key as keyof typeof options.value]
  })
}

// 表格勾选
const totalSelectedSize = ref(0)
const formatSize = (bytes: number) => {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

const handleSelectionChange = (val: GarbageItem[]) => {
  selectedRows.value = val
  const wholeVersionKeys = new Set(
    val.filter(item => item.category === 'missingJar').map(getVersionKey)
  )
  const countedVersions = new Set<string>()
  totalSelectedSize.value = val.reduce((sum, item) => {
    const versionKey = getVersionKey(item)
    if (versionKey && wholeVersionKeys.has(versionKey)) {
      if (item.category !== 'missingJar' || countedVersions.has(versionKey)) return sum
      countedVersions.add(versionKey)
      return sum + (item.versionSize ?? item.size)
    }
    return sum + item.size
  }, 0)
}

const getVersionKey = (item: GarbageItem) =>
  item.groupId && item.artifactId && item.version
    ? `${item.groupId}:${item.artifactId}:${item.version}`
    : ''

const removeDeletedResults = (paths: string[], deletedVersions: Set<string> = new Set()) => {
  results.value = results.value.filter(item =>
    !paths.includes(item.path) && !deletedVersions.has(getVersionKey(item))
  )
  if (activeCategory.value !== 'all' && !results.value.some(item => item.category === activeCategory.value)) {
    activeCategory.value = 'all'
  }
  resultsTableRef.value?.clearSelection()
  selectedRows.value = []
  totalSelectedSize.value = 0
}

const confirmPermanentDelete = async (count: number, includesWholeVersion = false) => {
  if (moveToTrash.value) return true
  try {
    await ElMessageBox.confirm(
      `将永久删除选中的 ${count} 项${includesWholeVersion ? '，其中缺失 JAR 诊断项会删除整个依赖版本目录' : ''}，无法恢复。是否继续？`,
      '确认直接删除',
      {
        type: 'warning',
        confirmButtonText: '直接删除',
        cancelButtonText: '取消'
      }
    )
    return true
  } catch {
    return false
  }
}

const deleteGarbageItem = async (item: GarbageItem) => {
  const isMissingJar = item.category === 'missingJar'
  if (isMissingJar) {
    try {
      await ElMessageBox.confirm(
        `将删除 ${item.packageName} 的整个版本目录，包括 POM、下载标记及该版本的其他文件。是否继续？`,
        '删除整个依赖版本',
        {
          type: 'warning',
          confirmButtonText: moveToTrash.value ? '移入回收站' : '直接删除',
          cancelButtonText: '取消'
        }
      )
    } catch {
      return
    }
  } else if (!(await confirmPermanentDelete(1))) {
    return
  }

  isDeleting.value = true
  try {
    const result = isMissingJar
      ? await RepoService.DeleteArtifactVersion(
          props.repoPath,
          item.groupId!,
          item.artifactId!,
          item.version!,
          moveToTrash.value
        )
      : await RepoService.DeleteFiles(props.repoPath, [item.path], moveToTrash.value)
    const deleted = Number(result?.count || 0)
    const failures = Number(result?.failures || 0)
    if (deleted > 0) {
      removeDeletedResults(
        isMissingJar ? [] : [item.path],
        isMissingJar ? new Set([getVersionKey(item)]) : new Set()
      )
      ElMessage.success(isMissingJar ? '已删除整个依赖版本' : '已删除所选项目')
      emit('cleanup-done')
    } else {
      ElMessage.warning('未能删除该项目，请查看后台控制台日志或检查文件是否被占用。')
    }
    if (failures > 0) ElMessage.warning(`${failures} 项未能删除，请查看后台控制台日志。`)
  } catch (error) {
    ElMessage.error(`删除失败：${String(error)}`)
  } finally {
    isDeleting.value = false
  }
}

const repairArtifact = async (item: GarbageItem) => {
  if (!item.groupId || !item.artifactId || !item.version) {
    ElMessage.warning('无法从文件路径解析 Maven 坐标，不能自动修复。')
    return
  }

  repairingItemId.value = item.id
  try {
    await RepoService.RepairArtifact(
      props.repoPath,
      item.groupId,
      item.artifactId,
      item.version,
      props.mavenPath,
      props.remoteRepositories
    )
    if (item.category === 'missingJar') {
      removeDeletedResults([], new Set([getVersionKey(item)]))
    } else if (item.category === 'lastUpdated') {
      removeDeletedResults([item.path])
    }
    ElMessage.success(`已从 Maven 仓库修复 ${item.groupId}:${item.artifactId}:${item.version}`)
  } catch (error) {
    ElMessage.error(`依赖修复失败：${String(error)}`)
  } finally {
    repairingItemId.value = ''
  }
}

onMounted(() => {
  unsubscribeEvents.push(
    Events.On('cleanup_scan_progress', (event: any) => {
      scannedFiles.value = event.data.scannedFiles
      totalFiles.value = event.data.totalFiles
      scanPhase.value = event.data.phase
      currentPath.value = event.data.currentPath
    }),
    Events.On('cleanup_scan_results', (event: any) => {
      results.value = [...results.value, ...event.data.items]
    }),
    Events.On('cleanup_scan_done', (event: any) => {
      scannedFiles.value = event.data.scannedFiles
      totalFiles.value = event.data.totalFiles
      scanPhase.value = 'processing'
      currentPath.value = ''
      isScanning.value = false
      scanFinished.value = true
    }),
    Events.On('cleanup_scan_error', (event: any) => {
      currentPath.value = ''
      isScanning.value = false
      ElMessage.error(`扫描失败：${event.data.message}`)
    })
  )
})

onUnmounted(() => {
  unsubscribeEvents.forEach(unsubscribe => unsubscribe())
})

// 扫描与删除
const startScan = async () => {
  if (!props.repoPath || isScanning.value) return
  if (!hasSelectedOptions.value) {
    ElMessage.warning('请至少选择一种扫描类型。')
    return
  }
  scannedCategories.value = categoryOptions
    .filter(({ key }) => options.value[key])
    .map(({ key }) => key)
  activeCategory.value = 'all'
  isScanning.value = true
  scanFinished.value = false
  results.value = []
  selectedRows.value = []
  totalSelectedSize.value = 0
  scannedFiles.value = 0
  totalFiles.value = 0
  scanPhase.value = 'counting'
  currentPath.value = ''

  try {
    await RepoService.StartGarbageScan(props.repoPath, { ...options.value })
  } catch (e) {
    isScanning.value = false
    ElMessage.error(`无法开始扫描：${String(e)}`)
  }
}

const handleDelete = async () => {
  if (selectedRows.value.length === 0) return

  const selected = [...selectedRows.value]
  const versionItems = new Map<string, GarbageItem>()
  selected.filter(item => item.category === 'missingJar').forEach(item => {
    versionItems.set(getVersionKey(item), item)
  })
  const affectedVersionKeys = new Set(versionItems.keys())
  const ordinaryItems = selected.filter(item => !affectedVersionKeys.has(getVersionKey(item)))
  if (!(await confirmPermanentDelete(ordinaryItems.length + versionItems.size, versionItems.size > 0))) return

  isDeleting.value = true
  
  try {
    let deletedCount = 0
    let failures = 0
    const deletedPaths: string[] = []
    const deletedVersions = new Set<string>()
    if (ordinaryItems.length > 0) {
      const paths = ordinaryItems.map(item => item.path)
      const result = await RepoService.DeleteFiles(props.repoPath, paths, moveToTrash.value)
      deletedCount += Number(result?.count || 0)
      failures += Number(result?.failures || 0)
      if (Number(result?.count || 0) > 0) deletedPaths.push(...paths)
    }
    for (const [key, item] of versionItems) {
      const result = await RepoService.DeleteArtifactVersion(
        props.repoPath, item.groupId!, item.artifactId!, item.version!, moveToTrash.value
      )
      const count = Number(result?.count || 0)
      deletedCount += count
      failures += Number(result?.failures || 0)
      if (count > 0) deletedVersions.add(key)
    }
    
    if (deletedCount === 0) {
      ElMessage.warning('未能删除任何项目，请查看后台控制台日志，或检查文件是否被占用。')
    } else {
      const action = moveToTrash.value ? '移入回收站' : '永久删除'
      ElMessage.success(`成功${action} ${deletedCount} 项`)
      emit('cleanup-done')
      removeDeletedResults(deletedPaths, deletedVersions)
    }
    if (failures > 0) {
      ElMessage.warning(`${failures} 项未能删除，请查看后台控制台日志。`)
    }
  } catch (e) {
    ElMessage.error('删除失败')
  } finally {
    isDeleting.value = false
  }
}
</script>

<template>
  <div class="cleanup-panel">
    <!-- 顶部选项区 -->
    <div class="options-section">
      <!-- 操作控制栏 -->
      <div class="options-control">
        <el-checkbox 
          :model-value="isAllSelected" 
          :indeterminate="isIndeterminate"
          :disabled="isScanning"
          @change="handleSelectAll"
        >
          全选
        </el-checkbox>
        <el-button link type="primary" size="small" :disabled="isScanning" @click="handleInvertSelect">反选</el-button>
        
        <div class="spacer"></div>
        
        <el-button type="primary" :loading="isScanning" :disabled="isScanning || !hasSelectedOptions" @click="startScan">
          {{ isScanning ? '扫描中...' : '开始扫描' }}
        </el-button>
      </div>
      
      <!-- 具体的复选框列表 -->
      <div class="checkboxes">
        <el-checkbox v-model="options.lastUpdated" :disabled="isScanning">下载失败标记 (.lastUpdated)</el-checkbox>
        <el-checkbox v-model="options.remote" :disabled="isScanning">来源记录 (_remote.repositories)</el-checkbox>
        <el-checkbox v-model="options.inProgress" :disabled="isScanning">中断残留 (*.in-progress)</el-checkbox>
        <el-checkbox v-model="options.zeroByte" :disabled="isScanning">零字节文件 (0B Jar/Pom)</el-checkbox>
        <el-checkbox v-model="options.emptyDir" :disabled="isScanning">空文件夹</el-checkbox>
        <el-checkbox v-model="options.missingJar" :disabled="isScanning">POM 声明为 JAR 但缺少 JAR</el-checkbox>
      </div>
    </div>

    <div v-if="scanFinished" class="category-filters" aria-label="按扫描类型筛选">
      <button
        type="button"
        class="category-filter"
        :class="{ active: activeCategory === 'all' }"
        @click="selectCategory('all')"
      >
        全部 <span class="category-count">{{ results.length }}</span>
      </button>
      <button
        v-for="category in categoryCounts"
        :key="category.key"
        type="button"
        class="category-filter"
        :class="{ active: activeCategory === category.key }"
        @click="selectCategory(category.key)"
      >
        {{ category.label }} <span class="category-count">{{ category.count }}</span>
      </button>
    </div>

    <!-- 扫描进度与结果操作区保持同一位置 -->
    <div class="action-bar" v-if="isScanning || (scanFinished && filteredResults.length > 0)">
      <div v-if="isScanning" class="info scan-info" role="status">
        <el-icon class="is-loading"><Loading /></el-icon>
        <span v-if="scanPhase === 'counting'">
          统计中：{{ scannedFiles.toLocaleString() }} 个文件
        </span>
        <span v-else>
          扫描中 {{ scannedFiles.toLocaleString() }} / {{ totalFiles.toLocaleString() }} ({{ scanPercentage }}%)
        </span>
        <span v-if="currentPath" class="scan-current-path" :title="currentPath">
          {{ currentPath }}
        </span>
      </div>
      <template v-else>
        <div class="info">
          当前类型发现 <strong>{{ filteredResults.length }}</strong> 项。
          已选 <strong>{{ selectedRows.length }}</strong> 项，
          预计释放 <strong>{{ formatSize(totalSelectedSize) }}</strong>
        </div>
        <div class="delete-actions">
          <el-checkbox v-model="moveToTrash">移入回收站</el-checkbox>
          <el-button type="danger" :loading="isDeleting" :disabled="selectedRows.length === 0" @click="handleDelete">
            {{ moveToTrash ? '移入回收站' : '直接删除' }}
          </el-button>
        </div>
      </template>
    </div>

    <!-- 结果表格 -->
    <div v-if="filteredResults.length > 0" class="table-wrapper">
      <el-table 
        ref="resultsTableRef"
        :data="filteredResults"
        row-key="id"
        width="100%"
        height="100%" 
        @selection-change="handleSelectionChange"
        class="custom-table"
      >
        <el-table-column
          type="selection"
          width="55"
          :reserve-selection="true"
        />
        <el-table-column label="包名" width="220">
          <template #default="scope">
            <el-tooltip :content="scope.row.path" placement="top">
              <el-button
                v-if="scope.row.groupId && scope.row.artifactId && scope.row.version"
                link
                type="primary"
                @click="emit('open-artifact', {
                  groupId: scope.row.groupId,
                  artifactId: scope.row.artifactId,
                  version: scope.row.version
                })"
              >
                {{ scope.row.packageName }}
              </el-button>
              <span v-else>{{ scope.row.packageName }}</span>
            </el-tooltip>
          </template>
        </el-table-column>
        <el-table-column prop="category" label="分类" width="150">
          <template #default="scope">
            <el-tag size="small" type="warning">
              {{ categoryLabel(scope.row.category) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="reason" label="原因" min-width="150" />
        <el-table-column
          prop="size"
          label="大小"
          width="100"
          sortable
          :sort-method="(a: GarbageItem, b: GarbageItem) =>
            (a.versionSize ?? a.size) - (b.versionSize ?? b.size)"
        >
          <template #default="scope">{{ formatSize(scope.row.versionSize ?? scope.row.size) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="100" fixed="right" align="center">
          <template #default="scope">
            <el-tooltip :content="scope.row.category === 'missingJar' ? '删除整个版本' : '删除'" placement="top">
              <el-button
                link
                type="danger"
                :icon="Delete"
                :disabled="isDeleting || !!repairingItemId"
                :aria-label="scope.row.category === 'missingJar' ? '删除整个版本' : '删除'"
                @click="deleteGarbageItem(scope.row)"
              />
            </el-tooltip>
            <el-tooltip
              v-if="(scope.row.category === 'missingJar' || scope.row.category === 'lastUpdated')
                && repairingItemId !== scope.row.id"
              :content="scope.row.category === 'missingJar' ? '修复 JAR' : '重新下载'"
              placement="top"
            >
              <el-button
                link
                type="primary"
                :disabled="isDeleting || !!repairingItemId"
                :aria-label="scope.row.category === 'missingJar' ? '修复 JAR' : '重新下载'"
                @click="repairArtifact(scope.row)"
              >
                <el-icon>
                  <svg viewBox="0 0 24 24" aria-hidden="true">
                    <path
                      fill="currentColor"
                      d="M22.7 19 13.6 9.9a6.5 6.5 0 0 0-8.5-8.2l4.8 4.8-3.2 3.2-4.8-4.8a6.5 6.5 0 0 0 8.2 8.5l9.1 9.1a1 1 0 0 0 1.4 0l2.1-2.1a1 1 0 0 0 0-1.4Z"
                    />
                  </svg>
                </el-icon>
              </el-button>
            </el-tooltip>
            <el-icon
              v-else-if="repairingItemId === scope.row.id"
              class="is-loading"
              aria-label="正在修复"
              role="status"
            >
              <Loading />
            </el-icon>
          </template>
        </el-table-column>
      </el-table>
    </div>
    
    <div v-if="!isScanning && filteredResults.length === 0" class="empty-tip">
      <el-empty
        :description="scanFinished
          ? (activeCategory === 'all' ? '所选扫描类型均未发现问题' : '该类型暂无扫描结果')
          : '暂无扫描结果，请点击右上角开始扫描'"
        :image-size="60"
      />
    </div>
  </div>
</template>

<style scoped>
.cleanup-panel {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: #fff;
  padding: 16px;
  /* 统一全局基础字体大小 */
  font-size: 13px;
}

/* 强制覆盖所有的按钮、复选框、文字字体大小 */
:deep(.el-checkbox__label),
:deep(.el-button),
:deep(.el-tag),
.info,
:deep(.el-empty__description p) {
  font-size: 13px !important;
}

/* ============== 顶部选项区 ============== */
.options-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding-bottom: 16px;
  border-bottom: 1px solid #ebeef5;
  margin-bottom: 16px;
  flex-shrink: 0;
}

.options-control {
  display: flex;
  align-items: center;
  gap: 16px;
  height: 32px; /* 固定高度防止抖动 */
}
.spacer {
  flex: 1;
}

/* Grid 布局确保各复选框宽度一致 */
.checkboxes {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 220px), 1fr));
  align-items: center;
  gap: 12px 20px;
  background: #f9fafc;
  padding: 12px 16px;
  border-radius: 4px;
  border: 1px solid #ebeef5;
}

.options-section :deep(.el-checkbox__label) {
  color: var(--el-text-color-regular);
  line-height: 1.4;
}

.options-section :deep(.el-checkbox__input.is-checked + .el-checkbox__label) {
  color: var(--el-text-color-regular);
}

.options-section :deep(.el-checkbox__input.is-checked .el-checkbox__inner),
.options-section :deep(.el-checkbox__input.is-indeterminate .el-checkbox__inner) {
  background-color: var(--el-color-primary);
  border-color: var(--el-color-primary);
}

.checkboxes :deep(.el-checkbox) {
  height: 24px;
  margin-right: 0;
}

.category-filters {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 12px;
  flex-shrink: 0;
}

.category-filter {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 5px 10px;
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  background: #fff;
  color: var(--el-text-color-regular);
  font: inherit;
  cursor: pointer;
}

.category-filter:hover {
  border-color: var(--el-color-primary-light-5);
  color: var(--el-color-primary);
}

.category-filter.active {
  border-color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
}

.category-count {
  min-width: 20px;
  padding: 0 5px;
  border-radius: 10px;
  background: #f0f2f5;
  color: var(--el-text-color-secondary);
  text-align: center;
}

.category-filter.active .category-count {
  background: #fff;
  color: var(--el-color-primary);
}

.scan-info,
.delete-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.scan-info {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  white-space: nowrap;
}

.scan-info > span {
  flex-shrink: 0;
}

.scan-current-path {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  color: var(--el-text-color-secondary);
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* ============== 结果操作区 ============== */
.action-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  height: 58px;
  min-height: 58px;
  background: #fdf6ec;
  border: 1px solid #faecd8;
  padding: 8px 16px;
  border-radius: 4px;
  margin-bottom: 16px;
  flex-shrink: 0;
}
.info {
  color: #606266;
}

/* ============== 表格区 ============== */
.table-wrapper {
  flex: 1;
  width: 100%;
  min-width: 0;
  overflow: hidden;
}
.empty-tip {
  flex: 1;
  display: flex;
  justify-content: center;
  align-items: center;
}

:deep(.custom-table) { width: 100%; --el-table-border-color: #ebeef5; --el-table-header-bg-color: #fafafa; }
:deep(.custom-table th.el-table__cell) { padding: 8px 0; font-weight: 600; color: #909399; font-size: 12px !important; }
:deep(.custom-table td.el-table__cell) { padding: 8px 0; font-size: 13px !important; }
</style>