/**
 * onBeforeMount 事件编排：基础数据加载、Wails 事件订阅（行情推送/AI 流/通知）。
 * 自 stock.vue 原样搬迁；跨块函数（fetchGroupList/updateData/buildFullReport/
 * scrollToAiResultBottom/updateTab）由组件注入。
 */
import { h, nextTick, onBeforeMount } from 'vue'
import { NAvatar, NButton } from 'naive-ui'
import { OpenURL, RestartAsAdmin } from '../../../wailsjs/go/main/App'
import { Environment, EventsOn, WindowReload } from '../../../wailsjs/runtime'
import * as stockApi from '../../api/stock'
import * as systemApi from '../../api/system'

export function useStockEvents(ctx) {
  const {
    route, message, notify, stockList, groupList, options, addBTN,
    promptTemplates, aiConfigs, sysPromptOptions, userPromptOptions, strategies,
    aiAnalysisTimeout, multiAgentState, currentGroupId, icon, data,
    fetchGroupList, updateData, buildFullReport, scrollToAiResultBottom, updateTab,
  } = ctx

  onBeforeMount(() => {
    stockApi.getGroupList().then(({data: result}) => {
      groupList.value = result
      const sorts = result.map(item => item.sort);
      const uniqueSorts = new Set(sorts);
      if (sorts.length !== uniqueSorts.size) {
        fetchGroupList();
      } else {
        if (route.query.groupId) {
          message.success("切换分组:" + route.query.groupName)
          currentGroupId.value = Number(route.query.groupId)
        }
      }
    }).catch(err => { console.error("GetGroupList error:", err) })
    stockApi.getStockList("").then(({data: result}) => {
      stockList.value = result
      options.value = result.map(item => {
        return {
          label: item.name + " - " + item.ts_code,
          value: item.ts_code
        }
      })
    }).catch(err => { console.error("GetStockList error:", err) })
    systemApi.getConfig().then(({data: result}) => {
      if (result.openAiEnable) {
        data.openAiEnable = true
      }
      if (result.darkTheme) {
        data.darkTheme = true
      }
    }).catch(err => { console.error("GetConfig error:", err) })
    systemApi.getPromptTemplates("", "").then(({data: res}) => {
      promptTemplates.value = res
  
      sysPromptOptions.value = promptTemplates.value.filter(item => item.type === '模型系统Prompt')
      userPromptOptions.value = promptTemplates.value.filter(item => item.type === '模型用户Prompt')
  
    }).catch(err => { console.error("GetPromptTemplates error:", err) })
  
    systemApi.getAiConfigs().then(({data: res}) => {
      aiConfigs.value = res
      if (res && res.length > 0) {
        data.aiConfigId = res[0].ID
      }
    }).catch(err => { console.error("GetAiConfigs error:", err) })
  
    systemApi.getAllStrategies().then(({data: res}) => {
      strategies.value = [
        { label: '📊 全维度分析', value: '' },
        ...res.map(s => ({ label: `📈 ${s.Name}`, value: s.Code }))
      ]
    }).catch(err => { console.error("GetAllStrategies error:", err) })
  
    EventsOn("loadingDone", (data) => {
      message.loading("刷新股票基础数据...")
      stockApi.getStockList("").then(({data: result}) => {
        stockList.value = result
        options.value = result.map(item => {
          return {
            label: item.name + " - " + item.ts_code,
            value: item.ts_code
          }
        })
      })
    })
  
    EventsOn("refresh", (data) => {
      message.success(data)
    })
  
    EventsOn("showSearch", (data) => {
      addBTN.value = data === 1;
    })
  
    EventsOn("stock_price", (data) => {
      updateData(data)
    })
  
    EventsOn("refreshFollowList", (data) => {
  
      WindowReload()
    })
  
    EventsOn("newChatStream", async (msg) => {
      if (msg === "DONE") {
        if (aiAnalysisTimeout.value) {
          clearTimeout(aiAnalysisTimeout.value)
          aiAnalysisTimeout.value = null
        }
        if (multiAgentState.active) {
          multiAgentState.done = true
        } else {
          systemApi.saveAiResponseResult(data.code, data.name, data.airesult, data.chatId, data.question, data.aiConfigId)
        }
        data.loading = false
        data.analysisStatus = "分析完成"
        message.destroyAll()
        notify.success({
          title: 'AI分析完成',
          content: `[${data.name}] 分析已完成`,
          duration: 3000,
        })
        setTimeout(() => {
          data.analysisStatus = ""
        }, 3000)
        return
      }
  
      // Try parsing as structured multi-agent event
      try {
        const rawContent = msg.Content || msg.content || ''
        let parsed = null
        if (typeof rawContent === 'string' && rawContent.startsWith('{')) {
          parsed = JSON.parse(rawContent)
        }
  
        if (parsed && parsed.type === 'agent:phase') {
          multiAgentState.active = true
          multiAgentState.currentPhase = parsed.phase
          multiAgentState.phaseLabel = parsed.label
          if (parsed.status === 'end') {
            multiAgentState.phases[parsed.phase] = true
          }
          data.analysisStatus = parsed.label || '分析中...'
          return
        }
  
        if (parsed && parsed.type === 'agent:token') {
          multiAgentState.active = true
          const agent = parsed.agent
          if (!multiAgentState.reports[agent]) {
            multiAgentState.reports[agent] = ''
          }
          multiAgentState.reports[agent] += parsed.token
          data.loading = false
          data.analysisStatus = `${agentTitle(agent)}分析师分析中...`
          return
        }
  
        if (parsed && parsed.type === 'agent:debate') {
          multiAgentState.active = true
          multiAgentState.debates.push({
            round: parsed.round,
            side: parsed.side,
            argument: parsed.argument,
          })
          return
        }
  
        if (parsed && parsed.type === 'agent:final') {
          multiAgentState.active = true
          multiAgentState.finalReport = parsed.report
          // Build full markdown report for export
          data.airesult = buildFullReport(multiAgentState)
          return
        }
      } catch (e) {
        // Not a multi-agent event, fall through to legacy handling
      }
  
      // Legacy flat-message handling
      if (msg.chatId) {
        data.chatId = msg.chatId
      }
      if (msg.question) {
        data.question = msg.question
      }
      if (msg.content || msg.reasoning_content || msg.extraContent) {
        if (!data.airesult) {
          data.analysisStatus = "AI正在分析中..."
        }
        data.loading = false
      }
      if (msg.content) {
        data.airesult = data.airesult + msg.content
      }
      if (msg.reasoning_content) {
        data.airesult = data.airesult + msg.reasoning_content
      }
      if (msg.extraContent) {
        data.airesult = data.airesult + msg.extraContent
      }
      scrollToAiResultBottom()
    })
  
    EventsOn("changeTab", async (msg) => {
      currentGroupId.value = Number(msg.ID)
      nextTick(() => {
        updateTab(currentGroupId.value);
      });
    })
  
  
    EventsOn("updateVersion", async (msg) => {
      const githubTimeStr = msg.published_at;
      const utcDate = new Date(githubTimeStr);
      const date = new Date(utcDate.getTime());
      const year = date.getFullYear();
      const month = String(date.getMonth() + 1).padStart(2, '0');
      const day = String(date.getDate()).padStart(2, '0');
      const hours = String(date.getHours()).padStart(2, '0');
      const minutes = String(date.getMinutes()).padStart(2, '0');
      const seconds = String(date.getSeconds()).padStart(2, '0');
      const formattedDate = `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`;
      notify.info({
        avatar: () =>
            h(NAvatar, {
              size: 'small',
              round: false,
              src: icon.value
            }),
        title: '发现新版本: ' + msg.tag_name,
        content: () => {
          return h('div', {
            style: {
              'text-align': 'left',
              'font-size': '14px',
            }
          }, {default: () => msg.commit?.message})
        },
        duration: 5000,
        meta: "发布时间:" + formattedDate,
        action: () => {
          return h(NButton, {
            type: 'primary',
            size: 'small',
            onClick: () => {
              Environment().then(env => {
                switch (env.platform) {
                  case 'windows':
                    window.open(msg.html_url)
                    break
                  default :
                    OpenURL(msg.html_url)
                }
              })
            }
          }, {default: () => '查看'})
        }
      })
    })
  
    EventsOn("updateNeedAdmin", (msg) => {
      notify.warning({
        avatar: () =>
            h(NAvatar, {
              size: 'small',
              round: false,
              src: icon.value
            }),
        title: '更新需要管理员权限',
        content: () => {
          return h('div', {
            style: {
              'text-align': 'left',
              'font-size': '14px',
            }
          }, { default: () => '新版本 ' + (msg.version || '') + ' 下载完成，但自动替换文件需要管理员权限。请以管理员身份重启程序后再次检查更新。' })
        },
        duration: 15000,
        action: () => {
          return h(NButton, {
            type: 'warning',
            size: 'small',
            onClick: () => {
              RestartAsAdmin()
            }
          }, { default: () => '以管理员身份重启' })
        }
      })
    })
  
    EventsOn("warnMsg", async (msg) => {
      notify.error({
        avatar: () =>
            h(NAvatar, {
              size: 'small',
              round: false,
              src: icon.value
            }),
        title: '警告',
        duration: 5000,
        content: () => {
          return h('div', {
            style: {
              'text-align': 'left',
              'font-size': '14px',
            }
          }, {default: () => msg})
        },
      })
    })
  })
}
