/**
 * AI 个股分析/复核触发（含策略选择、超时兜底）。
 * 自 stock.vue 原样搬迁；依赖经 ctx 传入。
 */
import * as stockApi from '../../api/stock'

export function useAiCheck(ctx) {
  const { message, modalShow4, enableTools, thinkingMode, strategyCode, data, aiAnalysisTimeout } = ctx

  function aiReCheckStock(stock, stockCode) {
    if (!data.aiConfigId) {
      message.error("请先选择AI模型配置")
      return
    }
    // 清除之前的超时定时器
    if (aiAnalysisTimeout.value) {
      clearTimeout(aiAnalysisTimeout.value)
      aiAnalysisTimeout.value = null
    }
    data.modelName = ""
    data.airesult = ""
    data.time = ""
    data.name = stock
    data.code = stockCode
    data.loading = true
    modalShow4.value = true
    data.analysisStatus = "正在连接AI服务..."
    message.loading("ai检测中...", {
      duration: 0,
    })
    //
  
    //message.info("sysPromptId:"+data.sysPromptId)
    stockApi.newChatStream(stock, stockCode, data.question, data.aiConfigId, data.sysPromptId, enableTools.value,thinkingMode.value, '', strategyCode.value)
      .catch(err => {
        data.loading = false
        data.analysisStatus = ""
        message.destroyAll()
        const errMsg = err?.message || err || "未知错误"
        message.error("AI分析请求失败: " + errMsg)
        data.airesult = "❌ AI分析请求失败: " + errMsg
      })
  
    // 设置超时兜底（5分钟）
    aiAnalysisTimeout.value = setTimeout(() => {
      if (data.loading) {
        data.loading = false
        data.analysisStatus = ""
        message.destroyAll()
        message.error("AI分析超时，请检查网络连接或AI服务配置")
        if (!data.airesult) {
          data.airesult = "❌ AI分析超时，请检查网络连接或AI服务配置是否正确。"
        }
      }
      aiAnalysisTimeout.value = null
    }, 5 * 60 * 1000)
  }
  
  function aiCheckStock(stock, stockCode) {
    stockApi.getAIResponseResult(stockCode).then(({data: result}) => {
      if (result.content) {
        data.modelName = result.modelName
        data.chatId = result.chatId
        data.question = result.question
        data.name = stock
        data.code = stockCode
        data.loading = false
        modalShow4.value = true
        data.airesult = result.content
        const date = new Date(result.CreatedAt);
        const year = date.getFullYear();
        const month = String(date.getMonth() + 1).padStart(2, '0');
        const day = String(date.getDate()).padStart(2, '0');
        const hours = String(date.getHours()).padStart(2, '0');
        const minutes = String(date.getMinutes()).padStart(2, '0');
        const seconds = String(date.getSeconds()).padStart(2, '0');
        data.time = `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`
      } else {
        data.modelName = ""
        data.question = ""
        data.airesult = ""
        data.time = ""
        data.name = stock
        data.code = stockCode
        data.loading = false
        modalShow4.value = true
        // message.loading("ai检测中...", {
        //   duration: 0,
        // })
        // NewChatStream(stock, stockCode, "", data.sysPromptId)
      }
    })
  }

  return { aiReCheckStock, aiCheckStock }
}
