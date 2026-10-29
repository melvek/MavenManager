<script setup lang="ts">
import { ref, onMounted, computed, nextTick } from 'vue'
import { RepoService, ConfigService } from '../bindings/mvnmanager/services'
import { Events } from '@wailsio/runtime'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { ArtifactCoordinate, ArtifactVersionSummary, TabItem, TreeNode, VersionGroup, AppConfig, RepositoryInfo, RepositoryOverview } from './types'

import TopBar from './components/TopBar.vue'
import ScanDialog from './components/ScanDialog.vue'
import SidebarTree from './components/SidebarTree.vue'
import MainContent from './components/MainContent.vue'
import SettingsDialog from './components/SettingsDialog.vue'
import RepoManagerDialog from './components/RepoManagerDialog.vue'

const appConfig = ref<AppConfig | null>(null)
const showSettings = ref(false)
const showRepoManager = ref(false)
const scanningRepoId = ref('')

const activeRepoId = computed({
  get: () => appConfig.value?.activeRepoId || '',
  set: (val) => { 
    if (appConfig.value) appConfig.value.activeRepoId = val
  }
})

const currentRepoPath = computed(() => {
  if (!appConfig.value) return ''
  const repo = appConfig.value.repositories.find(r => r.id === activeRepoId.value)
  return repo ? repo.path : ''
})

const isScanning = ref(false)
const treeData = ref<TreeNode[]>([])
const scannedCount = ref(0)
const totalFileCount = ref(0)
const scanPhase = ref('processing')
const scanCurrentPath = ref('')
const repositoryOverview = ref<RepositoryOverview | null>(null)
const repositoryInfos = ref<Record<string, RepositoryInfo | undefined>>({})
const showScanDialog = ref(false)
const isRepositoryLoaded = ref(false)
const allArtifactVersions = ref<ArtifactVersionSummary[]>([])

const dashboardTab: TabItem = { id: 'dashboard', title: '仓库概览', type: 'dashboard' }
const packagesTab: TabItem = { id: 'packages', title: '依赖列表', type: 'packages' }
const openedTabs = ref<TabItem[]>([{ ...dashboardTab }])
const activeTab = ref<string>(dashboardTab.id)
const exportItems = ref<ArtifactCoordinate[]>([])
const tabDataMap = ref<Record<string, VersionGroup[]>>({})
const selectedVersionMap = ref<Record<string, VersionGroup>>({})
const mainContentRef = ref<InstanceType<typeof MainContent> | null>(null)
const sidebarWidth = ref(280)

onMounted(async () => {
  Events.On('load_assets_progress', (event: any) => {
    scannedCount.value = event.data.scannedFiles
    totalFileCount.value = event.data.totalFiles
    scanPhase.value = event.data.phase
    scanCurrentPath.value = event.data.currentPath
  })

  Events.On('load_assets_done', async (event: any) => {
    isScanning.value = false
    repositoryOverview.value = event.data
    if (scanningRepoId.value) {
      await saveRepositoryInfo(scanningRepoId.value, {
        ...repositoryInfos.value[scanningRepoId.value],
        overview: event.data,
        lastScannedAt: new Date().toISOString()
      })
    }
    try {
      const loadedTree = await RepoService.GetTree()
      treeData.value = (loadedTree ?? []).filter((node): node is TreeNode => node !== null)
      allArtifactVersions.value = (await RepoService.GetAllArtifactVersions()) ?? []
      isRepositoryLoaded.value = true
    } catch (error) {
      isRepositoryLoaded.value = false
      showScanDialog.value = false
      ElMessage.error(`加载依赖索引失败：${String(error)}`)
      return
    }
    setTimeout(() => { showScanDialog.value = false }, 2000)
  })

  Events.On('load_assets_error', (event: any) => {
    isScanning.value = false
    isRepositoryLoaded.value = false
    showScanDialog.value = false
    ElMessage.error(`仓库扫描失败：${event.data.message}`)
  })

  const cfg = await ConfigService.GetConfig()
  if (cfg) {
    cfg.repositories = cfg.repositories || []
    appConfig.value = cfg as unknown as AppConfig
    try {
      repositoryInfos.value = (await ConfigService.GetRepositoryInfos()) ?? {}
    } catch (error) {
      ElMessage.error(`读取仓库信息失败：${String(error)}`)
    }
    repositoryOverview.value =
      repositoryInfos.value[activeRepoId.value]?.overview ?? null
  }
})

const saveRepositoryInfo = async (repoId: string, info: RepositoryInfo) => {
  try {
    await ConfigService.SaveRepositoryInfo(repoId, info)
    repositoryInfos.value[repoId] = info
  } catch (error) {
    ElMessage.error(`保存仓库信息失败：${String(error)}`)
  }
}

const refreshAllArtifactVersions = async () => {
  if (!isRepositoryLoaded.value) return
  allArtifactVersions.value = (await RepoService.GetAllArtifactVersions()) ?? []
}

// ================= 核心：Tab 管理逻辑 =================
const openArtifactTab = async (data: any, isNewTab: boolean) => {
  const tabId = `${data.groupId}:${data.artifactId}`;
  
  const existingTab = openedTabs.value.find(t => t.id === tabId)
  if (existingTab) {
    activeTab.value = tabId;
    await loadTabData(tabId, data.groupId, data.artifactId)
    if (data.version) await selectArtifactVersion(tabId, data.version)
    return;
  }

  const newTab: TabItem = { 
    id: tabId, 
    title: data.artifactId, 
    type: 'artifact', 
    groupId: data.groupId, 
    artifactId: data.artifactId 
  };

  if (isNewTab) {
    // 双击：追加新标签页
    openedTabs.value.push(newTab);
    activeTab.value = tabId;
  } else {
    // 单击：替换当前的 artifact 标签页
    const currentIdx = openedTabs.value.findIndex(t => t.id === activeTab.value);
    
    if (currentIdx !== -1 && openedTabs.value[currentIdx].type === 'artifact') {
      // 替换前，清空旧标签的缓存
      const oldTabId = openedTabs.value[currentIdx].id;
      delete tabDataMap.value[oldTabId];
      delete selectedVersionMap.value[oldTabId];
      
      openedTabs.value[currentIdx] = newTab;
      activeTab.value = tabId;
    } else {
      // 如果当前是清理面板，或者没有打开任何 Tab，则等同于双击行为
      openedTabs.value.push(newTab);
      activeTab.value = tabId;
    }
  }

  await loadTabData(tabId, data.groupId, data.artifactId);
  if (data.version) await selectArtifactVersion(tabId, data.version)
}

const handleSingleClick = (data: any) => openArtifactTab(data, false);
const handleDoubleClick = (data: any) => openArtifactTab(data, true);
const handleCleanupOpenArtifact = (data: { groupId: string; artifactId: string; version: string }) =>
  openArtifactTab(data, true)
const handlePackageOpen = (data: { groupId: string; artifactId: string }) =>
  openArtifactTab(data, true)

const handleAddExportArtifact = (artifact: ArtifactCoordinate) => {
  const exists = exportItems.value.some(item =>
    item.groupId === artifact.groupId &&
    item.artifactId === artifact.artifactId &&
    item.version === artifact.version
  )
  if (!exists) exportItems.value.push({ ...artifact })

  const tabId = 'export-tool'
  if (!openedTabs.value.some(tab => tab.id === tabId)) {
    openedTabs.value.push({ id: tabId, title: '导出列表', type: 'export' })
  }
  activeTab.value = tabId
  if (exists) ElMessage.info('该依赖版本已在导出列表中')
  else ElMessage.success(`已添加 ${artifact.artifactId}-${artifact.version}`)
}

const handleAddExportArtifacts = (artifacts: ArtifactCoordinate[]) => {
  const existing = new Set(exportItems.value.map(item =>
    `${item.groupId}:${item.artifactId}:${item.version}`
  ))
  let added = 0
  for (const artifact of artifacts) {
    const key = `${artifact.groupId}:${artifact.artifactId}:${artifact.version}`
    if (existing.has(key)) continue
    existing.add(key)
    exportItems.value.push({ ...artifact })
    added++
  }
  const tabId = 'export-tool'
  if (!openedTabs.value.some(tab => tab.id === tabId)) {
    openedTabs.value.push({ id: tabId, title: '导出列表', type: 'export' })
  }
  activeTab.value = tabId
  if (added === 0) ElMessage.info('所选依赖版本已全部在导出列表中')
  else ElMessage.success(`已加入 ${added} 个依赖版本到导出列表`)
}

const handleViewAllPackages = () => {
  if (!isRepositoryLoaded.value) return
  if (!openedTabs.value.some(tab => tab.id === packagesTab.id)) {
    openedTabs.value.push({ ...packagesTab })
  }
  activeTab.value = packagesTab.id
}

const handleRemoveExportArtifact = (artifact: ArtifactCoordinate) => {
  exportItems.value = exportItems.value.filter(item =>
    !(item.groupId === artifact.groupId &&
      item.artifactId === artifact.artifactId &&
      item.version === artifact.version)
  )
}

const clearExportList = () => {
  exportItems.value = []
}


// ================= 系统与仓库管理 =================
const handleSwitchRepo = async (repoId: string) => {
  if (!appConfig.value || activeRepoId.value === repoId || isScanning.value) return
  const previousRepoId = activeRepoId.value
  activeRepoId.value = repoId
  repositoryOverview.value = repositoryInfos.value[repoId]?.overview ?? null
  try {
    await ConfigService.SaveConfig(appConfig.value)
  } catch (error) {
    activeRepoId.value = previousRepoId
    repositoryOverview.value = repositoryInfos.value[previousRepoId]?.overview ?? null
    ElMessage.error(`保存当前仓库失败：${String(error)}`)
    return
  }
  treeData.value = []
  allArtifactVersions.value = []
  isRepositoryLoaded.value = false
  openedTabs.value = [{ ...dashboardTab }]
  activeTab.value = dashboardTab.id
  exportItems.value = []
  
  tabDataMap.value = {}
  selectedVersionMap.value = {}
  const repository = appConfig.value.repositories.find(repo => repo.id === repoId)
  try {
    await ElMessageBox.confirm(
      `已切换到“${repository?.name || '所选仓库'}”。是否立即扫描并加载仓库内容？选择暂不扫描时，将显示已有统计数据，依赖列表需要扫描后才能浏览。`,
      '扫描仓库',
      {
        type: 'info',
        confirmButtonText: '立即扫描',
        cancelButtonText: '暂不扫描'
      }
    )
  } catch {
    return
  }
  await startScan()
}

const startScan = async () => {
  if (isScanning.value) return
  if (!currentRepoPath.value) return alert('请先配置仓库路径')
  scanningRepoId.value = activeRepoId.value
  isScanning.value = true
  showScanDialog.value = true
  scannedCount.value = 0
  totalFileCount.value = 0
  scanPhase.value = 'counting'
  scanCurrentPath.value = currentRepoPath.value
  treeData.value = []
  allArtifactVersions.value = []
  isRepositoryLoaded.value = false
  try {
    await RepoService.LoadAssets(currentRepoPath.value)
  } catch (error) {
    isScanning.value = false
    showScanDialog.value = false
    ElMessage.error(`无法开始仓库扫描：${String(error)}`)
  }
}

const openCleanupPanel = () => {
  const tabId = 'cleanup-tool'
  if (openedTabs.value.find(t => t.id === tabId)) {
    activeTab.value = tabId
    return
  }
  openedTabs.value.push({ id: tabId, title: '垃圾清理', type: 'cleanup' })
  activeTab.value = tabId
}

const handleCleanupDone = async () => {
  if (activeRepoId.value) {
    await saveRepositoryInfo(activeRepoId.value, {
      ...repositoryInfos.value[activeRepoId.value],
      lastCleanupAt: new Date().toISOString()
    })
  }
}

const handleSaveRepoManager = async (newConfig: AppConfig) => {
  const previousActiveRepoId = activeRepoId.value
  const previousRepositories = appConfig.value?.repositories ?? []
  const changedPaths = newConfig.repositories.filter(repo => {
    const previous = previousRepositories.find(item => item.id === repo.id)
    return previous && previous.path !== repo.path
  })

  if (!newConfig.repositories.find(r => r.id === activeRepoId.value)) {
    if (newConfig.repositories.length > 0) {
      newConfig.activeRepoId = newConfig.repositories[0].id
    } else {
      newConfig.activeRepoId = ''
    }
  }
  const activeRepositoryPathChanged = changedPaths.some(repo => repo.id === newConfig.activeRepoId)

  try {
    await ConfigService.SaveConfig(newConfig)
    for (const repo of changedPaths) {
      await ConfigService.DeleteRepositoryInfo(repo.id)
      delete repositoryInfos.value[repo.id]
    }
    repositoryInfos.value = (await ConfigService.GetRepositoryInfos()) ?? {}
  } catch (error) {
    ElMessage.error(`保存仓库配置失败：${String(error)}`)
    return
  }

  appConfig.value = newConfig
  repositoryOverview.value = repositoryInfos.value[newConfig.activeRepoId]?.overview ?? null
  const activeRepositoryChanged =
    previousActiveRepoId !== newConfig.activeRepoId || activeRepositoryPathChanged
  if (activeRepositoryChanged) {
    treeData.value = []
    allArtifactVersions.value = []
    isRepositoryLoaded.value = false
    openedTabs.value = [{ ...dashboardTab }]
    activeTab.value = dashboardTab.id
    exportItems.value = []
    tabDataMap.value = {}
    selectedVersionMap.value = {}
  }
}

const handleSaveSettings = async (newConfig: AppConfig) => {
  await ConfigService.SaveConfig(newConfig)
  appConfig.value = newConfig
}

const loadTabData = async (tabId: string, groupId: string, artifactId: string, forceReload = false) => {
  let versionGroups = tabDataMap.value[tabId]
  const selectedVersion = selectedVersionMap.value[tabId]?.version
  if (!versionGroups || forceReload) {
    const res = (await RepoService.GetArtifactVersions(groupId, artifactId)) || []
    
    const groups: Record<string, any[]> = {}
    res.forEach((file: any) => {
      if (!groups[file.version]) groups[file.version] = []
      groups[file.version].push(file)
    })

    versionGroups = Object.keys(groups).map(version => {
      const files = groups[version].sort((a, b) => a.fileName.localeCompare(b.fileName))
      const isPom = files.some(file =>
        file.fileName.endsWith('.pom') && file.packaging?.toLowerCase() === 'pom'
      )
      return { 
        version, files, 
        totalSize: files.reduce((sum, f) => sum + f.size, 0),
        latestModTime: files.reduce((latest, f) => new Date(f.modTime) > new Date(latest) ? f.modTime : latest, files[0]?.modTime || ''),
        isPom: files.some(f => f.fileName.endsWith('.pom') && (f.packaging ?? '').toLowerCase() === 'pom'),
        hasJar: files.some(f => f.fileName.endsWith('.jar') && !f.fileName.endsWith('-sources.jar')),
        hasSource: files.some(f => f.fileName.endsWith('-sources.jar')),
        hasSha1: files.some(f => f.fileName.endsWith('.sha1')),
        hasPom: files.some(f => f.fileName.endsWith('.pom')),
        hasRemote: files.some(f => f.fileName === '_remote.repositories'),
        hasLastUpdated: files.some(f => f.fileName.endsWith('.lastUpdated'))
      }
    })

    versionGroups.sort((a, b) => b.version.localeCompare(a.version))
    tabDataMap.value[tabId] = versionGroups
    const refreshedSelectedVersion = versionGroups.find(item => item.version === selectedVersion)
    if (refreshedSelectedVersion) {
      selectedVersionMap.value[tabId] = refreshedSelectedVersion
      await nextTick()
      mainContentRef.value?.setCurrentRow(tabId, refreshedSelectedVersion)
    }
  }

  if (!selectedVersionMap.value[tabId] && versionGroups.length > 0) {
    const firstVersion = versionGroups[0]
    selectedVersionMap.value[tabId] = firstVersion
    await nextTick()
    mainContentRef.value?.setCurrentRow(tabId, firstVersion)
  }
}

const handleArtifactRefresh = async (data: {
  tabId: string
  groupId: string
  artifactId: string
}) => {
  await loadTabData(data.tabId, data.groupId, data.artifactId, true)
  await refreshAllArtifactVersions()
}

const handleArtifactVersionDeleted = async (data: { tabId: string; version: string }) => {
  const selectedVersion = selectedVersionMap.value[data.tabId]?.version
  const remainingVersions = (tabDataMap.value[data.tabId] ?? []).filter(
    item => item.version !== data.version
  )
  tabDataMap.value[data.tabId] = remainingVersions
  const nextSelection = remainingVersions.find(item => item.version === selectedVersion) ??
    (selectedVersion === data.version ? remainingVersions[0] : undefined)
  if (nextSelection) {
    selectedVersionMap.value[data.tabId] = nextSelection
    await nextTick()
    mainContentRef.value?.setCurrentRow(data.tabId, nextSelection)
  } else {
    delete selectedVersionMap.value[data.tabId]
    mainContentRef.value?.clearHighlight(data.tabId)
  }
  await refreshAllArtifactVersions()
}

const handleRowClick = ({ tabId, row }: { tabId: string, row: VersionGroup }) => {
  if (row) selectedVersionMap.value[tabId] = row
}

const selectArtifactVersion = async (tabId: string, version: string) => {
  const row = tabDataMap.value[tabId]?.find(item => item.version === version)
  if (!row) {
    ElMessage.warning(`详情中未找到版本 ${version}`)
    return
  }
  selectedVersionMap.value[tabId] = row
  await nextTick()
  mainContentRef.value?.setCurrentRow(tabId, row)
}

const handleCloseDetail = (tabId: string) => {
  delete selectedVersionMap.value[tabId]
  mainContentRef.value?.clearHighlight(tabId)
}

const handleTabRemove = (targetName: string) => {
  if (targetName === dashboardTab.id) return
  const tabs = openedTabs.value
  let activeName = activeTab.value
  if (activeName === targetName) {
    tabs.forEach((tab, index) => {
      if (tab.id === targetName) {
        const nextTab = tabs[index + 1] || tabs[index - 1]
        activeName = nextTab ? nextTab.id : ''
      }
    })
  }
  activeTab.value = activeName
  openedTabs.value = tabs.filter(t => t.id !== targetName)
  delete tabDataMap.value[targetName]
  delete selectedVersionMap.value[targetName]
}

const handleTabChange = (tabId: string) => {
  const tab = openedTabs.value.find(t => t.id === tabId)
  if (tab && tab.type === 'artifact') loadTabData(tab.id, tab.groupId!, tab.artifactId!)
}
</script>

<template>
  <div class="app-container" style="--wails-draggable: no-drag;">
    <TopBar
      :repositories="appConfig?.repositories || []"
      :activeRepoId="activeRepoId"
      @open-settings="showSettings = true"
    />
    
    <ScanDialog 
      :visible="showScanDialog" 
      :isScanning="isScanning" 
      :scannedCount="scannedCount" 
      :totalFileCount="totalFileCount"
      :scanPhase="scanPhase"
      :currentPath="scanCurrentPath"
    />

    <SettingsDialog 
      v-model:visible="showSettings"
      :config="appConfig"
      @save="handleSaveSettings"
    />

    <RepoManagerDialog 
      v-model:visible="showRepoManager"
      :config="appConfig"
      @save="handleSaveRepoManager"
    />

    <div class="main-body">
      <SidebarTree 
        v-model:width="sidebarWidth" 
        :treeData="treeData" 
        :isScanning="isScanning"
        :isRepositoryLoaded="isRepositoryLoaded"
        @node-single-click="handleSingleClick"
        @node-double-click="handleDoubleClick"
        @view-all="handleViewAllPackages"
      />
      
      <MainContent 
        ref="mainContentRef"
        v-model:activeTab="activeTab"
        :openedTabs="openedTabs"
        :treeData="treeData"
        :allArtifactVersions="allArtifactVersions"
        :exportItems="exportItems"
        :tabDataMap="tabDataMap"
        :selectedVersionMap="selectedVersionMap"
        :repoPath="currentRepoPath"
        :mavenPath="appConfig?.mavenPath || 'mvn'"
        :remoteRepositories="appConfig?.remoteRepositories || []"
        :repositories="appConfig?.repositories || []"
        :repositoryInfos="repositoryInfos"
        :activeRepoId="activeRepoId"
        :overview="repositoryOverview"
        :isScanning="isScanning"
        @tab-change="handleTabChange"
        @tab-remove="handleTabRemove"
        @row-click="handleRowClick"
        @close-detail="handleCloseDetail"
        @cleanup-done="handleCleanupDone"
        @cleanup-open-artifact="handleCleanupOpenArtifact"
        @open-package="handlePackageOpen"
        @add-export-artifacts="handleAddExportArtifacts"
        @artifact-refresh="handleArtifactRefresh"
        @artifact-version-deleted="handleArtifactVersionDeleted"
        @add-export-artifact="handleAddExportArtifact"
        @remove-export-artifact="handleRemoveExportArtifact"
        @clear-export-list="clearExportList"
        @select-repository="handleSwitchRepo"
        @scan="startScan"
        @open-cleanup="openCleanupPanel"
        @open-repo-manager="showRepoManager = true"
      />
    </div>
    <footer class="status-bar">
      <span v-if="repositoryOverview">
        {{ repositoryOverview.totalGroups.toLocaleString() }} 个 Group ·
        {{ repositoryOverview.totalPackages.toLocaleString() }} 个包 ·
        {{ repositoryOverview.totalVersions.toLocaleString() }} 个版本 ·
        {{ repositoryOverview.totalFiles.toLocaleString() }} 个文件
      </span>
    </footer>
  </div>
</template>

<style scoped>
.app-container { display: flex; flex-direction: column; height: 100vh; }
.main-body { flex: 1; display: flex; overflow: hidden; }
.status-bar {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  height: 26px;
  padding: 0 12px;
  flex-shrink: 0;
  color: #909399;
  background: #fff;
  border-top: 1px solid #ebeef5;
  font-size: 12px;
}
</style>