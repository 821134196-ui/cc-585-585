<template>
  <div>
    <div class="card">
      <h3>模拟扫码(本地接口)</h3>
      <div class="form-row" style="align-items:flex-end">
        <div class="form-item" style="flex:1">
          <label>扫描运单号或回单编号(如 WB2026-0001 / RC-1)</label>
          <input v-model="scanCode" placeholder="扫码枪输入,回车查询" @keyup.enter="doScan" />
        </div>
        <button class="btn" @click="doScan">查询</button>
      </div>
      <div v-if="scanResult" class="alert info mono" style="white-space:pre-wrap">{{ scanResult }}</div>
    </div>

    <div class="card">
      <h3>运单列表</h3>
      <div class="form-row">
        <div class="form-item">
          <label>状态</label>
          <select v-model="status" @change="load">
            <option value="">全部</option>
            <option value="in_transit">运输中</option>
            <option value="receipt_pending">回单待审核</option>
            <option value="signed">已签收</option>
            <option value="adjusted">已调整</option>
          </select>
        </div>
        <div class="form-item" style="flex:1">
          <label>搜索</label>
          <input v-model="q" placeholder="运单号 / 客户" @keyup.enter="load" />
        </div>
        <div class="form-item" style="justify-content:flex-end">
          <button class="btn ghost" @click="load">刷新</button>
        </div>
      </div>
      <table class="grid">
        <thead>
          <tr>
            <th>运单号</th><th>客户</th><th>线路</th><th>司机</th>
            <th>计划件数</th><th>实际签收</th><th>破损</th><th>合同运费</th>
            <th>业务发生日</th><th>状态</th><th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="w in list" :key="w.id">
            <td class="mono">{{ w.waybill_no }}</td>
            <td>{{ w.customer_name }}</td>
            <td>{{ w.route_from }} → {{ w.route_to }}</td>
            <td>{{ w.driver?.name }}</td>
            <td><b>{{ w.planned_pieces }}</b></td>
            <td>
              <b v-if="w.actual_pieces !== null" :class="{ short: w.actual_pieces < w.planned_pieces }">{{ w.actual_pieces }}</b>
              <span v-else class="muted">未签收</span>
            </td>
            <td><span v-if="w.damaged_pieces > 0" class="tag red">{{ w.damaged_pieces }} 件</span><span v-else class="muted">0</span></td>
            <td class="mono">{{ fmtMoney(w.settlement?.amount) }}</td>
            <td>{{ fmtDate(w.business_date) }}</td>
            <td>
              <span class="tag" :class="statusClass(w.status)">{{ WAYBILL_STATUS[w.status] }}</span>
              <span v-if="w.pending_receipts > 0" class="tag orange" style="margin-left:4px">回单待审</span>
            </td>
            <td><router-link class="btn sm ghost" :to="'/waybills/' + w.id">详情</router-link></td>
          </tr>
          <tr v-if="list.length === 0"><td colspan="11" class="muted" style="text-align:center;padding:30px">暂无运单</td></tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { api, fmtDate, fmtMoney, WAYBILL_STATUS } from '../api'

const list = ref([])
const status = ref('')
const q = ref('')
const scanCode = ref('')
const scanResult = ref('')

async function load() {
  const params = new URLSearchParams()
  if (status.value) params.set('status', status.value)
  if (q.value) params.set('q', q.value)
  list.value = await api('/waybills?' + params)
}

async function doScan() {
  scanResult.value = ''
  if (!scanCode.value) return
  try {
    const data = await api('/mock/scan', { method: 'POST', json: { code: scanCode.value } })
    scanResult.value = JSON.stringify(data, null, 2)
  } catch (e) {
    scanResult.value = '扫码失败: ' + e.message
  }
}

function statusClass(s) {
  return { in_transit: 'gray', receipt_pending: 'orange', signed: 'green', adjusted: 'blue' }[s] || 'gray'
}

onMounted(load)
</script>

<style scoped>
.short { color: #c2410c; }
</style>
