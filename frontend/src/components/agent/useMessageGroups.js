/**
 * 消息分组与展开状态（自 FloatingAgentAssistant.vue 原样搬迁）。
 * messages 经 ctx 传入（ref 共享引用）。
 */
import { ref, computed } from 'vue'

export function useMessageGroups(ctx) {
  const { messages } = ctx

  const expandedGroups = ref(new Set())
  const reasoningExpandedMap = ref({})

  const messageGroups = computed(() => {
    const groups = []
    let currentGroup = null

    for (let i = 0; i < messages.value.length; i++) {
      const msg = messages.value[i]
      if (msg.role === 'user') {
        if (currentGroup) {
          groups.push(currentGroup)
        }
        currentGroup = {
          id: i,
          userMsg: msg,
          userIndex: i,
          assistantMsg: null,
          assistantIndex: -1
        }
      } else if (msg.role === 'assistant' && currentGroup) {
        currentGroup.assistantMsg = msg
        currentGroup.assistantIndex = i
      }
    }
    if (currentGroup) {
      groups.push(currentGroup)
    }
    return groups
  })

  function isGroupExpanded(groupIndex) {
    return expandedGroups.value.has(groupIndex)
  }

  function toggleGroup(groupIndex) {
    const newSet = new Set(expandedGroups.value)
    if (newSet.has(groupIndex)) {
      newSet.delete(groupIndex)
    } else {
      newSet.add(groupIndex)
    }
    expandedGroups.value = newSet
  }

  function initDefaultExpanded() {
    if (messageGroups.value.length > 0 && expandedGroups.value.size === 0) {
      expandedGroups.value = new Set([messageGroups.value.length - 1])
    }
  }

  function ensureLatestGroupExpanded() {
    if (messageGroups.value.length > 0) {
      const lastIndex = messageGroups.value.length - 1
      const newSet = new Set(expandedGroups.value)
      newSet.add(lastIndex)
      expandedGroups.value = newSet
    }
  }

  function toggleReasoning(index) {
    reasoningExpandedMap.value = {
      ...reasoningExpandedMap.value,
      [index]: !reasoningExpandedMap.value[index]
    }
  }

  return {
    messageGroups, expandedGroups, reasoningExpandedMap,
    isGroupExpanded, toggleGroup, initDefaultExpanded, ensureLatestGroupExpanded, toggleReasoning,
  }
}
