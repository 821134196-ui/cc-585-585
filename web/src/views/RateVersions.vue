<template>
  <el-card class="page-card">
    <div class="section-title">费率版本管理
      <span class="muted" style="font-weight:400">财务调整单按"原业务发生时间"生效的版本计算,版本快照随调整单保存</span>
    </div>
    <el-form inline>
      <el-form-item label="查询业务日期适用版本">
        <el-date-picker v-model="queryDate" type="date" value-format="YYYY-MM-DD" placeholder="选择业务发生日期" />
      </el-form-item>
      <el-form-item>
        <el-button type="primary" @click="resolve">查询适用版本</el-button>
      </el-form-item>
      <el-form-item v-if="resolved">
        <el-alert type="success" :closable="false" style="padding:4px 10px">
          {{ queryDate }} 适用:<b>{{ resolved.version }}</b>(单价 ¥{{ resolved.unit_price }}/件,破损扣 ¥{{ resolved.damage_deduction_per_piece }}/件)
        </el-alert>
      </el-form-item>
    </el-form>
    <el-table :data="rows" stripe :row-class-name="rowClass">
      <el-table-column prop="version" label="版本号" width="120" />
      <el-table-column label="生效日期" width="130">
        <template #default="{ row }">{{ row.effective_from.slice(0, 10) }}</template>
      </el-table-column>
      <el-table-column label="运费单价" width="120" align="right">
        <template #default="{ row }">¥{{ row.unit_price.toFixed(2) }} / 件</template>
      </el-table-column>
      <el-table-column label="破损扣款" width="120" align="right">
        <template #default="{ row }">¥{{ row.damage_deduction_per_piece.toFixed(2) }} / 件</template>
      </el-table-column>
      <el-table-column prop="remark" label="备注" />
    </el-table>
  </el-card>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import api, { showError } from '../api'

const rows = ref([])
const queryDate = ref('')
const resolved = ref(null)

async function load() {
  try {
    const { data } = await api.get('/rate-versions')
    rows.value = data
  } catch (e) { showError(e) }
}
async function resolve() {
  if (!queryDate.value) return
  try {
    const { data } = await api.get('/rate-versions/resolve', { params: { at: queryDate.value } })
    resolved.value = data
  } catch (e) {
    resolved.value = null
    showError(e)
  }
}
function rowClass({ row }) {
  return resolved.value && row.id === resolved.value.id ? 'highlight-row' : ''
}
onMounted(load)
</script>

<style>
.el-table .highlight-row { --el-table-tr-bg-color: #f0f9eb; }
</style>
