/**
 * 分组标签页拖拽排序（HTML5 DnD + .n-tabs-tab DOM 类名联动）。
 * 自 stock.vue 原样搬迁；stockApi/message/groupList 经 ctx 传入。
 */
import { ref } from 'vue'

export function useDraggableTabs(ctx) {
  const { stockApi, message, groupList } = ctx

  // 拖拽相关变量
  const dragSourceIndex = ref(null)
  const dragTargetIndex = ref(null)
  
  // 拖拽处理函数
  function handleTabDragStart(event, name) {
    // "全部"标签（name=0）不应该触发拖拽
    if (name === 0) {
      event.preventDefault();
      return;
    }
    dragSourceIndex.value = name;
    event.dataTransfer.effectAllowed = 'move';
    event.target.classList.add('tab-dragging');
  }
  
  
  function handleTabDragOver(event) {
    event.preventDefault()
    event.dataTransfer.dropEffect = 'move'
  }
  
  function handleTabDragEnter(event, name) {
    event.preventDefault();
    // "全部"标签（name=0）不应该作为拖拽目标
    if (name > 0) {
      dragTargetIndex.value = name;
      if (event.target.classList) {
        // 查找最近的标签元素并添加高亮样式
        let tabElement = event.target.closest('.n-tabs-tab');
        if (tabElement) {
          tabElement.classList.add('tab-drag-over');
        }
      }
    }
  }
  
  function handleTabDragLeave(event) {
    // 查找最近的标签元素并移除高亮样式
    let tabElement = event.target.closest('.n-tabs-tab')
    if (tabElement && tabElement.classList) {
      tabElement.classList.remove('tab-drag-over')
    }
    // 不要重置 dragTargetIndex，因为可能会在元素间快速移动
  }
  
  function handleTabDrop(event) {
    event.preventDefault();
  
    // 移除所有高亮样式
    const tabs = document.querySelectorAll('.n-tabs-tab');
    if(!tabs || tabs.length === 0){
      return
    }
    tabs.forEach(tab => {
      tab.classList.remove('tab-drag-over');
    });
  
    if (dragSourceIndex.value !== null && dragTargetIndex.value !== null &&
        dragSourceIndex.value !== dragTargetIndex.value) {
  
      // 确保索引有效（排除"全部"选项卡）
      if (dragSourceIndex.value > 0 && dragTargetIndex.value > 0) {
        // 查找源分组和目标分组
        const sourceGroup = groupList.value.find(g => g.ID === dragSourceIndex.value);
        const targetGroup = groupList.value.find(g => g.ID === dragTargetIndex.value);
  
        if (sourceGroup && targetGroup) {
          // 计算新的位置序号（使用目标分组的sort值）
          const newSortPosition = targetGroup.sort;
  
          // 调用后端API更新组排序
          stockApi.updateGroupSort(sourceGroup.ID, newSortPosition).then(({data: result}) => {
            if (result) {
              message.success('分组排序更新成功');
              // 重新获取分组列表以更新界面
              stockApi.getGroupList().then(({data}) => {
                groupList.value = data;
              });
            } else {
              message.error('分组排序更新失败');
            }
          }).catch(error => {
            message.error('分组排序更新失败: ' + error.message);
          });
        }
      }
    }
  
    // 重置状态
    dragSourceIndex.value = null;
    dragTargetIndex.value = null;
  }
  
  function handleTabDragEnd(event) {
    // 移除所有高亮样式
    const tabs = document.querySelectorAll('.n-tabs-tab')
    if(!tabs || tabs.length === 0){
      return
    }
    tabs.forEach(tab => {
      tab.classList.remove('tab-drag-over', 'tab-dragging')
    })
  
    dragSourceIndex.value = null
    dragTargetIndex.value = null
  }

  // 清理拖拽事件监听器
  // 清理拖拽事件监听器
  function cleanupDraggableTabs() {
    const tabs = document.querySelectorAll('.n-tabs-tab');
    if(!tabs || tabs.length === 0){
      return
    }
    tabs.forEach((tab) => {
      // 移除所有可能的拖拽事件监听器
      tab.removeEventListener('dragstart', handleTabDragStart);
      tab.removeEventListener('dragover', handleTabDragOver);
      tab.removeEventListener('dragenter', handleTabDragEnter);
      tab.removeEventListener('dragleave', handleTabDragLeave);
      tab.removeEventListener('drop', handleTabDrop);
      tab.removeEventListener('dragend', handleTabDragEnd);
      // 移除draggable属性
      tab.removeAttribute('draggable');
    });
  }
  
  // 初始化可拖拽选项卡
  function initDraggableTabs() {
    // 移除之前可能添加的事件监听器
    cleanupDraggableTabs();
  
    // 添加拖拽事件监听器到选项卡元素
    setTimeout(() => {
      const tabs = document.querySelectorAll('.n-tabs-tab');
      if(!tabs || tabs.length === 0){
        return
      }
      tabs.forEach((tab, index) => {
        const dataIndex = tab.getAttribute('data-name');
        const name = parseInt(dataIndex);
  
        // 只为分组标签（name > 0）添加拖拽功能
        if (name > 0) {
          tab.setAttribute('draggable', 'true');
          tab.addEventListener('dragstart', (e) => handleTabDragStart(e, name));
          tab.addEventListener('dragover', handleTabDragOver);
          tab.addEventListener('dragenter', (e) => handleTabDragEnter(e, name));
          tab.addEventListener('dragleave', handleTabDragLeave);
          tab.addEventListener('drop', handleTabDrop);
          tab.addEventListener('dragend', handleTabDragEnd);
        }
      });
    }, 100);
  }

  return { cleanupDraggableTabs, initDraggableTabs }
}
