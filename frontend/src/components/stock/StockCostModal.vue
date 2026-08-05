<script setup>
// 股票成本/数量/提醒设置弹窗（自 stock.vue 原样抽离）
const show = defineModel('show', { type: Boolean })
defineProps({
  formModel: { type: Object, required: true },
})
const emit = defineEmits(['save'])
</script>

<template>
  <n-modal transform-origin="center" size="small" v-model:show="show" :title="formModel.name" style="width: 800px;max-width: calc(100vw - 32px);"
           :preset="'card'">
    <n-form :model="formModel" :rules="{
              costPrice: { required: true, message: '请输入成本'},
              volume: { required: true, message: '请输入数量'},
              alarm:{required: true, message: '涨跌报警值'} ,
              alarmPrice: { required: true, message: '请输入报警价格'},
              sort: { required: true, message: '请输入排序值'},
            }" label-placement="left" label-width="100px">
      <n-grid :cols="2" :x-gap="12">
        <n-gi>
          <n-form-item label="股票成本" path="costPrice">
            <n-input-number v-model:value="formModel.costPrice" min="0" placeholder="请输入股票成本" style="width: 100%">
              <template #suffix>
                {{ formModel.code.indexOf("hk") >= 0 ? "HK$" : "¥" }}
              </template>
            </n-input-number>
          </n-form-item>
        </n-gi>
        <n-gi>
          <n-form-item label="股票数量" path="volume">
            <n-input-number v-model:value="formModel.volume" min="0" step="100" placeholder="请输入股票数量" style="width: 100%">
              <template #suffix>
                股
              </template>
            </n-input-number>
          </n-form-item>
        </n-gi>
        <n-gi>
          <n-form-item label="涨跌提醒" path="alarm">
            <n-input-number v-model:value="formModel.alarm" min="0" placeholder="涨跌报警值(%)" style="width: 100%">
              <template #suffix>
                %
              </template>
            </n-input-number>
          </n-form-item>
        </n-gi>
        <n-gi>
          <n-form-item label="股价提醒" path="alarmPrice">
            <n-input-number v-model:value="formModel.alarmPrice" min="0" placeholder="股价报警值" style="width: 100%">
              <template #suffix>
                {{ formModel.code.indexOf("hk") >= 0 ? "HK$" : "¥" }}
              </template>
            </n-input-number>
          </n-form-item>
        </n-gi>
        <n-gi>
          <n-form-item label="开仓价" path="entryPrice">
            <n-input-number v-model:value="formModel.entryPrice" min="0" step="0.01" placeholder="请输入开仓价" style="width: 100%">
              <template #suffix>
                {{ formModel.code.indexOf("hk") >= 0 ? "HK$" : "¥" }}
              </template>
            </n-input-number>
          </n-form-item>
        </n-gi>
        <n-gi>
          <n-form-item label="股票排序" path="sort">
            <n-input-number v-model:value="formModel.sort" min="0" placeholder="排序值" style="width: 100%">
            </n-input-number>
          </n-form-item>
        </n-gi>
        <n-gi>
          <n-form-item label="止盈价" path="takeProfitPrice">
            <n-input-number v-model:value="formModel.takeProfitPrice" min="0" step="0.01" placeholder="请输入止盈价" style="width: 100%">
              <template #suffix>
                {{ formModel.code.indexOf("hk") >= 0 ? "HK$" : "¥" }}
              </template>
            </n-input-number>
          </n-form-item>
        </n-gi>
        <n-gi>
          <n-form-item label="止损价" path="stopLossPrice">
            <n-input-number v-model:value="formModel.stopLossPrice" min="0" step="0.01" placeholder="请输入止损价" style="width: 100%">
              <template #suffix>
                {{ formModel.code.indexOf("hk") >= 0 ? "HK$" : "¥" }}
              </template>
            </n-input-number>
          </n-form-item>
        </n-gi>
      </n-grid>
    </n-form>
    <template #footer>
      <n-button type="primary"
                @click="emit('save', formModel.code, formModel.costPrice, formModel.volume, formModel.alarm, formModel)">
        保存
      </n-button>
    </template>
  </n-modal>
</template>
