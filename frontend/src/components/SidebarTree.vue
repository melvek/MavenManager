<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted, nextTick } from 'vue'
import { Search, Folder, Document, Grid } from '@element-plus/icons-vue'
import type { TreeNode } from '../types'

const props = defineProps<{
  treeData: TreeNode[]
  width: number
  isScanning: boolean
  isRepositoryLoaded: boolean
}>()

// 修改抛出的事件
const emit = defineEmits(['node-single-click', 'node-double-click', 'update:width', 'view-all'])

// ================= 搜索过滤逻辑 =================
const filterText = ref('')
const treeRef = ref<any>(null)

watch(filterText, (val) => {
  treeRef.value?.filter(val)
})

const filterNode = (value: string, data: any) => {
  if (!value) return true
  return data.label.includes(value)
}

// ================= 树的高度动态计算 =================
const treeWrapperRef = ref<HTMLElement | null>(null)
const treeHeight = ref(500)
let resizeObserver: ResizeObserver | null = null

const updateHeight = () => {
  if (treeWrapperRef.value) {
    const rect = treeWrapperRef.value.getBoundingClientRect()
    const style = window.getComputedStyle(treeWrapperRef.value)
    const pt = parseFloat(style.paddingTop) || 0
    const pb = parseFloat(style.paddingBottom) || 0
    treeHeight.value = Math.max(100, Math.floor(rect.height - pt - pb))
  }
}

onMounted(() => {
  nextTick(updateHeight)
  if (treeWrapperRef.value) {
    resizeObserver = new ResizeObserver(updateHeight)
    resizeObserver.observe(treeWrapperRef.value)
  }
})

onUnmounted(() => {
  if (resizeObserver) resizeObserver.disconnect()
})

// ================= 单击/双击分离逻辑 =================
let clickTimer: ReturnType<typeof setTimeout> | null = null;

const handleNodeClick = (data: any) => {
  if (!data.isLeaf) return; // 目录只负责展开/折叠
  
  if (clickTimer) return; // 已经有定时器在等待，说明可能是双击的第一次点击
  
  clickTimer = setTimeout(() => {
    clickTimer = null;
    emit('node-single-click', data);
  }, 250); // 250ms 内没有第二次点击，判定为单击
}

const handleNodeDblClick = (data: any) => {
  if (!data.isLeaf) return;
  
  if (clickTimer) {
    clearTimeout(clickTimer); // 清除单击定时器
    clickTimer = null;
  }
  emit('node-double-click', data);
}

// ================= 拖拽逻辑 =================
const isDragging = ref(false)
const startResize = (e: MouseEvent) => {
  e.preventDefault()
  isDragging.value = true
  document.body.style.cursor = 'col-resize'
  document.body.style.userSelect = 'none'

  const startX = e.clientX
  const startWidth = props.width

  const onMouseMove = (moveEvent: MouseEvent) => {
    moveEvent.preventDefault()
    const newWidth = Math.max(200, Math.min(600, startWidth + (moveEvent.clientX - startX)))
    emit('update:width', newWidth)
  }

  const onMouseUp = () => {
    isDragging.value = false
    document.body.style.cursor = ''
    document.body.style.userSelect = ''
    document.removeEventListener('mousemove', onMouseMove)
    document.removeEventListener('mouseup', onMouseUp)
  }

  document.addEventListener('mousemove', onMouseMove)
  document.addEventListener('mouseup', onMouseUp)
}
</script>

<template>
  <aside class="sidebar" :style="{ width: width + 'px' }">
    <!--<div class="function-bar">
      <el-button
        class="view-all-button"
        size="small"
        :icon="Grid"
        :disabled="!isRepositoryLoaded || isScanning"
        @click="emit('view-all')"
      >
        查看全部
      </el-button>
    </div>-->
    <div class="search-box">
      <el-input 
        v-model="filterText" 
        placeholder="搜索当前仓库依赖..." 
        size="small" 
        :prefix-icon="Search" 
        clearable
      />
    </div>
    
    <div class="tree-wrapper" ref="treeWrapperRef">
      <el-empty
        v-if="isScanning || !isRepositoryLoaded"
        :description="isScanning ? '正在扫描仓库…' : '请扫描仓库后查看依赖'"
        :image-size="56"
      />
      <el-empty
        v-else-if="treeData.length === 0"
        description="仓库中没有可展示的依赖"
        :image-size="56"
      />
      <el-tree-v2
        v-else
        ref="treeRef"
        :data="treeData"
        :props="{ value: 'id', label: 'label', children: 'children' }"
        :height="treeHeight" 
        :item-size="28"
        :filter-method="filterNode"
        highlight-current
        @node-click="handleNodeClick"
      >
        <template #default="{ node, data }">
          <!-- 在自定义节点上绑定双击事件 -->
          <span 
            class="custom-tree-node" 
            @dblclick.stop="handleNodeDblClick(data)"
            :title="data.isLeaf ? '单击预览，双击在新标签页打开' : ''"
          >
            <el-icon v-if="!data.isLeaf" color="#e6a23c"><Folder /></el-icon>
            <el-icon v-else color="#909399"><Document /></el-icon>
            <span class="node-label">{{ node.label }}</span>
          </span>
        </template>
      </el-tree-v2>
    </div>

    <div class="resizer" :class="{ 'is-dragging': isDragging }" @mousedown="startResize"></div>
  </aside>
</template>

<style scoped>
/* 保持原有样式不变 */
.sidebar { 
  background: #fff; 
  border-right: 1px solid #dcdfe6; 
  display: flex; 
  flex-direction: column; 
  flex-shrink: 0; 
  position: relative; 
  --wails-draggable: no-drag;
}
.search-box { 
  padding: 10px 12px; 
  border-bottom: 1px solid #ebeef5; 
  flex-shrink: 0;
  display: flex; 
  align-items: center;
  gap: 8px; 
  --wails-draggable: no-drag; 
}
.function-bar {
  display: flex;
  padding: 8px 12px 0;
  flex-shrink: 0;
  --wails-draggable: no-drag;
}
.view-all-button { width: 100%; }
.tree-wrapper { 
  flex: 1; 
  position: relative; 
  overflow: hidden; 
  padding: 6px 0; 
  --wails-draggable: no-drag; 
}
.tree-wrapper :deep(.el-empty) { height: 100%; }
.custom-tree-node { 
  display: flex; align-items: center; gap: 6px; 
  user-select: none; width: 100%; height: 100%;
  font-size: 13px; padding-right: 8px; 
}
.node-label { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
:deep(.el-tree-node__content) { height: 28px; border-radius: 0; }
:deep(.el-tree-node__content:hover) { background-color: #f5f7fa; }
:deep(.el-tree-node.is-current > .el-tree-node__content) {
  background-color: #fdf6ec !important;
  color: #e6a23c;
  font-weight: bold;
}
.resizer {
  position: absolute;
  top: 0;
  right: -3px; 
  width: 6px;  
  height: 100%;
  cursor: col-resize;
  z-index: 99; 
  background: transparent;
  transition: background 0.2s;
  --wails-draggable: no-drag; 
}
.resizer:hover, .resizer.is-dragging { background: #c0c4cc; }
</style>