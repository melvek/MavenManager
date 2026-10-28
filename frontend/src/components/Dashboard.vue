<script setup lang="ts">
import { computed } from 'vue'
import { Collection, Folder, Refresh, Timer } from '@element-plus/icons-vue'
import type { Repository, RepositoryInfo, RepositoryOverview } from '../types'

const props = defineProps<{
  repositories: Repository[]
  repositoryInfos: Record<string, RepositoryInfo | undefined>
  activeRepoId: string
  overview: RepositoryOverview | null
  isScanning: boolean
}>()

const emit = defineEmits<{
  'select-repository': [repoId: string]
  scan: []
  'open-cleanup': []
  'open-repo-manager': []
}>()

const activeRepository = computed(() =>
  props.repositories.find(repo => repo.id === props.activeRepoId) ?? null
)

const activeRepositoryInfo = computed(() =>
  props.repositoryInfos[props.activeRepoId] ?? {}
)

const formatDate = (value?: string) => {
  if (!value) return '尚未清理'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '时间不可用' : date.toLocaleString()
}

const scannedLabel = (repoId: string) => {
  const scannedAt = props.repositoryInfos[repoId]?.lastScannedAt
  return scannedAt ? `扫描于 ${formatDate(scannedAt)}` : '尚未扫描'
}
</script>

<template>
  <section class="dashboard">
    <header class="dashboard-heading">
      <div>
        <p class="eyebrow">MAVEN REPOSITORY</p>
        <h1>仓库概览</h1>
        <p class="subtitle">查看本地依赖仓库状态，快速扫描或清理缓存。</p>
      </div>
      <el-button type="primary" :icon="Refresh" :loading="isScanning" @click="emit('scan')">
        {{ overview ? '重新扫描' : '扫描仓库' }}
      </el-button>
    </header>

    <div v-if="overview" class="stats-grid">
      <article class="stat-card">
        <div class="stat-label"><el-icon><Folder /></el-icon> Group 数量</div>
        <strong>{{ overview.totalGroups.toLocaleString() }}</strong>
      </article>
      <article class="stat-card">
        <div class="stat-label"><el-icon><Collection /></el-icon> 包数量</div>
        <strong>{{ overview.totalPackages.toLocaleString() }}</strong>
      </article>
      <article class="stat-card">
        <div class="stat-label"><el-icon><Collection /></el-icon> 包版本总数</div>
        <strong>{{ overview.totalVersions.toLocaleString() }}</strong>
      </article>
      <article class="stat-card">
        <div class="stat-label"><el-icon><Folder /></el-icon> 文件总数</div>
        <strong>{{ overview.totalFiles.toLocaleString() }}</strong>
      </article>
    </div>

    <div v-else class="welcome-card">
      <div class="welcome-icon"><el-icon><Folder /></el-icon></div>
      <div>
        <h2>{{ activeRepository ? '仓库尚未扫描' : '还没有配置仓库' }}</h2>
        <p>
          {{ activeRepository
            ? '扫描仓库后，这里会显示 Group、包、版本及文件统计。'
            : '先添加一个本地 Maven 仓库，然后开始扫描。' }}
        </p>
      </div>
      <el-button
        v-if="activeRepository"
        type="primary"
        :loading="isScanning"
        @click="emit('scan')"
      >
        开始扫描
      </el-button>
      <el-button v-else type="primary" @click="emit('open-repo-manager')">
        添加仓库
      </el-button>
    </div>

    <div class="dashboard-columns">
      <section class="dashboard-card repositories-card">
        <div class="card-heading">
          <div>
            <h2>仓库列表</h2>
            <span>{{ repositories.length }} 个本地仓库</span>
          </div>
          <el-button text type="primary" @click="emit('open-repo-manager')">管理仓库</el-button>
        </div>

        <div v-if="repositories.length" class="repository-list">
          <button
            v-for="repo in repositories"
            :key="repo.id"
            class="repository-item"
            :class="{ active: repo.id === activeRepoId }"
            :disabled="isScanning"
            @click="emit('select-repository', repo.id)"
          >
            <el-icon class="repository-icon"><Folder /></el-icon>
            <span class="repository-details">
              <strong>{{ repo.name }}</strong>
              <span class="repository-path" :title="repo.path">{{ repo.path }}</span>
            </span>
            <span class="repository-meta">
              {{ scannedLabel(repo.id) }}
            </span>
          </button>
        </div>
        <el-empty v-else description="暂无仓库" :image-size="64" />
      </section>

      <section class="dashboard-card">
        <div class="card-heading">
          <div>
            <h2>当前仓库</h2>
            <span>{{ activeRepository?.name || '未选择' }}</span>
          </div>
        </div>

        <div v-if="activeRepository" class="current-repository">
          <div class="path-block">
            <span class="detail-label">仓库位置</span>
            <span class="detail-value" :title="activeRepository.path">{{ activeRepository.path }}</span>
          </div>
          <div class="cleanup-time">
            <el-icon><Timer /></el-icon>
            <span>上次垃圾清理</span>
            <strong>{{ formatDate(activeRepositoryInfo.lastCleanupAt) }}</strong>
          </div>
          <el-button class="cleanup-button" :icon="Collection" @click="emit('open-cleanup')">
            打开垃圾清理
          </el-button>
        </div>
        <el-empty v-else description="请先添加并选择仓库" :image-size="64" />
      </section>
    </div>
  </section>
</template>

<style scoped>
.dashboard {
  flex: 1;
  width: min(100%, 1120px);
  min-height: 0;
  margin: 0 auto;
  padding: 38px 40px;
  overflow: auto;
  color: #303133;
  box-sizing: border-box;
}

.dashboard-heading,
.card-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.dashboard-heading { margin-bottom: 24px; }
.eyebrow { margin: 0 0 6px; color: #909399; font-size: 11px; letter-spacing: 0.1em; }
h1 { margin: 0; font-size: 23px; font-weight: 600; }
.subtitle { margin: 8px 0 0; color: #909399; }

.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
  margin-bottom: 20px;
}

.stat-card,
.dashboard-card,
.welcome-card {
  background: #fff;
  border: 1px solid #e4e7ed;
  border-radius: 8px;
}

.stat-card { min-height: 100px; padding: 16px 18px; }
.stat-label { display: flex; align-items: center; gap: 7px; color: #909399; }
.stat-card strong { display: block; margin-top: 12px; font-size: 24px; font-weight: 600; }

.welcome-card {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 20px;
  margin-bottom: 20px;
  background: #f8fafc;
}

.welcome-icon {
  display: grid;
  width: 42px;
  height: 42px;
  place-items: center;
  color: var(--el-color-primary);
  background: #ecf5ff;
  border-radius: 8px;
  font-size: 20px;
}
.welcome-card h2 { margin: 0 0 6px; font-size: 15px; }
.welcome-card p { margin: 0; color: #909399; }
.welcome-card > :last-child { margin-left: auto; }

.dashboard-columns { display: grid; grid-template-columns: minmax(0, 1.35fr) minmax(280px, 1fr); gap: 16px; }
.dashboard-card { min-width: 0; padding: 18px; }
.card-heading { padding-bottom: 14px; border-bottom: 1px solid #ebeef5; }
.card-heading h2 { margin: 0 0 5px; font-size: 15px; font-weight: 600; }
.card-heading span { color: #909399; font-size: 12px; }

.repository-list { display: flex; flex-direction: column; }
.repository-item {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  min-width: 0;
  padding: 14px 8px;
  color: inherit;
  text-align: left;
  background: transparent;
  border: 0;
  border-bottom: 1px solid #f0f2f5;
  cursor: pointer;
}
.repository-item:last-child { border-bottom: 0; }
.repository-item:hover,
.repository-item.active { background: #f5f7fa; }
.repository-item:disabled { cursor: default; }
.repository-icon { color: var(--el-color-primary); font-size: 18px; }
.repository-details { display: flex; flex: 1; flex-direction: column; gap: 4px; min-width: 0; }
.repository-details strong { font-weight: 500; }
.repository-path { overflow: hidden; color: #909399; text-overflow: ellipsis; white-space: nowrap; }
.repository-meta { flex-shrink: 0; color: #909399; font-size: 11px; }

.current-repository { display: flex; flex-direction: column; gap: 18px; padding-top: 18px; }
.path-block { display: flex; flex-direction: column; gap: 7px; }
.detail-label { color: #909399; font-size: 12px; }
.detail-value { overflow-wrap: anywhere; line-height: 1.5; }
.cleanup-time { display: flex; align-items: center; gap: 8px; color: #909399; }
.cleanup-time strong { margin-left: auto; color: #606266; font-weight: 400; text-align: right; }
.cleanup-button { align-self: flex-start; }

@media (max-width: 900px) {
  .dashboard { padding: 24px; }
  .stats-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .dashboard-columns { grid-template-columns: 1fr; }
}

@media (max-width: 560px) {
  .dashboard { padding: 18px; }
  .dashboard-heading { align-items: flex-start; }
  .repository-meta { display: none; }
  .welcome-card { align-items: flex-start; flex-wrap: wrap; }
  .welcome-card > :last-child { margin-left: 58px; }
}
</style>
