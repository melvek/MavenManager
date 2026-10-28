<script setup lang="ts">
import { ref, watch } from 'vue'
import { Setting, Document } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import type { AppConfig, RemoteRepository } from '../types'
import RemoteRepositoryEditor from './RemoteRepositoryEditor.vue'

const props = defineProps<{
  visible: boolean
  config: AppConfig | null
}>()

const emit = defineEmits(['update:visible', 'save'])

const localConfig = ref<AppConfig & { remoteRepositories: RemoteRepository[] }>({
  mavenPath: '',
  repositories: [],
  activeRepoId: '',
  remoteRepositories: []
})
const activeMenu = ref('maven')

watch(() => props.visible, (val) => {
  if (val && props.config) {
    localConfig.value = JSON.parse(JSON.stringify(props.config))
    localConfig.value.remoteRepositories ??= []
  }
})

const handleSave = () => {
  const invalidRemoteRepository = localConfig.value.remoteRepositories?.find(
    repository => !repository.name.trim() ||
      !/^https?:\/\/[^/\s]+(?:\/[^\s]*)?$/i.test(repository.url.trim())
  )
  if (invalidRemoteRepository) {
    ElMessage.warning('请为每个额外远程仓库填写名称和有效的 HTTP(S) URL')
    activeMenu.value = 'repositories'
    return
  }
  emit('save', localConfig.value)
  emit('update:visible', false)
}
</script>

<template>
  <el-dialog 
    :model-value="visible" 
    @update:model-value="(val: any) => emit('update:visible', val)"
    title="系统设置" 
    width="700px"
    body-class="settings-dialog-body"
    :lock-scroll="false" 
  >
    <div class="settings-container">
      <!-- 左侧导航 -->
      <div class="settings-sidebar">
        <el-menu :default-active="activeMenu" @select="(key: string) => activeMenu = key" class="settings-menu">
          <el-menu-item index="maven">
            <el-icon><Setting /></el-icon>
            <span>Maven 命令</span>
          </el-menu-item>
          <el-menu-item index="repositories">
            <el-icon><Document /></el-icon>
            <span>额外远程仓库</span>
          </el-menu-item>
          <el-menu-item index="general">
            <el-icon><Document /></el-icon>
            <span>通用设置</span>
          </el-menu-item>
        </el-menu>
      </div>

      <!-- 右侧内容 -->
      <div class="settings-content">
        <div v-if="activeMenu === 'maven'" class="pane">
          <h3 class="pane-title">Maven 环境配置</h3>
          
          <el-form label-position="top" size="default">
            <el-form-item label="Maven 根目录或命令路径 (可选)">
              <el-input v-model="localConfig.mavenPath" placeholder="如 D:\apache-maven-3.3.3 或 D:\apache-maven-3.3.3\bin\mvn.cmd" />
              <div class="form-tip">修复依赖时会读取根目录下 conf/settings.xml；留空则使用系统环境变量中的 mvn</div>
            </el-form-item>
          </el-form>
        </div>

        <div v-show="activeMenu === 'repositories'" class="pane">
          <RemoteRepositoryEditor v-model="localConfig.remoteRepositories" />
        </div>

        <div v-if="activeMenu === 'general'" class="pane">
          <h3 class="pane-title">通用设置</h3>
          <el-empty description="更多设置开发中..." :image-size="60" />
        </div>
      </div>
    </div>

    <template #footer>
      <span class="dialog-footer">
        <el-button @click="emit('update:visible', false)">取消</el-button>
        <el-button type="primary" @click="handleSave">保存</el-button>
      </span>
    </template>
  </el-dialog>
</template>

<style scoped>
/* 覆盖默认的弹窗内容 padding */
:deep(.settings-dialog-body) {
  padding: 0;
}

.settings-container {
  display: flex;
  height: 400px;
}

.settings-sidebar {
  width: 180px;
  background: #f9fafc;
  border-right: 1px solid #ebeef5;
}

.settings-menu {
  border-right: none;
  background: transparent;
}
.settings-menu .el-menu-item.is-active {
  background-color: #e6f2ff;
  color: #409eff;
}

.settings-content {
  flex: 1;
  padding: 24px;
  overflow-y: auto;
}

.pane-title {
  font-size: 16px;
  color: #303133;
  margin-bottom: 20px;
  padding-bottom: 10px;
  border-bottom: 1px solid #ebeef5;
}

.form-tip {
  font-size: 12px;
  color: #909399;
  line-height: 1.4;
  margin-top: 4px;
}

</style>