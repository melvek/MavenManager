<script setup lang="ts">
import { ref, watch } from 'vue'
import { Delete, Plus } from '@element-plus/icons-vue'
import type { RemoteRepository } from '../types'

const props = defineProps<{
  modelValue: RemoteRepository[]
}>()

const emit = defineEmits<{
  'update:modelValue': [repositories: RemoteRepository[]]
}>()

const localRepositories = ref<RemoteRepository[]>([])

watch(
  () => props.modelValue,
  repositories => {
    localRepositories.value = repositories.map(repository => ({ ...repository }))
  },
  { immediate: true }
)

const emitChange = () => emit('update:modelValue', localRepositories.value.map(repository => ({ ...repository })))

const addRepository = () => {
  localRepositories.value.push({ name: '', url: '' })
  emitChange()
}

const updateFromInput = (event: Event) => {
  const input = event.currentTarget
  if (!(input instanceof HTMLInputElement)) return
  const row = input.closest<HTMLElement>('.remote-repository-row')
  const index = Number(row?.dataset.index)
  const repository = localRepositories.value[index]
  if (!Number.isInteger(index) || !repository) return
  if (input.dataset.field === 'name') repository.name = input.value
  else if (input.dataset.field === 'url') repository.url = input.value
  else return
  emitChange()
}

const removeFromButton = (event: Event) => {
  const button = event.currentTarget
  if (!(button instanceof HTMLElement)) return
  const row = button.closest<HTMLElement>('.remote-repository-row')
  const index = Number(row?.dataset.index)
  if (!Number.isInteger(index) || !localRepositories.value[index]) return
  localRepositories.value.splice(index, 1)
  emitChange()
}
</script>

<template>
  <div>
    <div class="remote-repository-heading">
      <div>
        <h3 class="pane-title">额外远程仓库</h3>
        <p class="form-tip">
          修复依赖时优先使用 Maven settings.xml 中已配置的仓库和镜像；这里的仓库作为额外来源，不会修改 settings.xml。
        </p>
      </div>
      <el-button type="primary" plain :icon="Plus" @click="addRepository">添加仓库</el-button>
    </div>

    <div
      v-for="repository in localRepositories"
      :key="localRepositories.indexOf(repository)"
      class="remote-repository-row"
      :data-index="localRepositories.indexOf(repository)"
    >
      <input :value="repository.name" data-field="name" placeholder="名称" @input="updateFromInput" />
      <input
        :value="repository.url"
        data-field="url"
        placeholder="https://repo.example.com/maven2"
        @input="updateFromInput"
      />
      <el-button type="danger" plain :icon="Delete" @click="removeFromButton" />
    </div>
    <el-empty
      v-if="localRepositories.length === 0"
      description="未配置额外远程仓库，将仅使用 Maven settings.xml 中的配置"
      :image-size="60"
    />
  </div>
</template>

<style scoped>
.remote-repository-heading {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
  margin-bottom: 16px;
}

.pane-title {
  margin: 0 0 8px;
  padding-bottom: 10px;
  border-bottom: 1px solid #ebeef5;
  color: #303133;
  font-size: 16px;
}

.form-tip {
  margin: 0;
  color: #909399;
  font-size: 12px;
  line-height: 1.4;
}

.remote-repository-row {
  display: flex;
  gap: 8px;
  margin-bottom: 10px;
}

.remote-repository-row input {
  flex: 1;
  min-width: 0;
  height: 32px;
  padding: 0 10px;
  color: #606266;
  background: #fff;
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  outline: none;
}

.remote-repository-row input:focus {
  border-color: #409eff;
}
</style>
