<template>
  <div class="card">
    <h3>争议记录</h3>
    <div class="form-row">
      <div class="form-item">
        <label>状态</label>
        <select v-model="status" @change="load">
          <option value="">全部</option>
          <option value="open">待处理</option>
          <option value="resolved">已办结</option>
        </select>
      </div>
    </div>
    <table class="grid">
      <thead><tr><th>编号</th><th>运单</th><th>类型</th><th>标题</th><th>详情</th><th>登记人</th><th>状态</th><th>处理结果</th></tr></thead>
      <tbody>
        <tr v-for="d in list" :key="d.id">
          <td class="mono">{{ d.dispute_no }}</td>
          <td><router-link :to="'/waybills/' + d.waybill_id" class="mono">{{ d.waybill?.waybill_no }}</router-link></td>
          <td><span class="tag" :class="{ pieces: 'orange', damage: 'red', rate: 'blue' }[d.type]">{{ DISPUTE_TYPE[d.type] }}</span></td>
          <td>{{ d.title }}</td>
          <td style="max-width:320px">{{ d.detail }}</td>
          <td>{{ d.created_by?.name }}</td>
          <td><span class="tag" :class="d.status === 'resolved' ? 'green' : 'orange'">{{ DISPUTE_STATUS[d.status] }}</span></td>
          <td>{{ d.resolution || '—' }}</td>
        </tr>
        <tr v-if="list.length === 0"><td colspan="8" class="muted" style="text-align:center;padding:30px">暂无争议</td></tr>
      </tbody>
    </table>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { api, DISPUTE_TYPE, DISPUTE_STATUS } from '../api'
const list = ref([])
const status = ref('')
async function load() {
  const p = status.value ? '?status=' + status.value : ''
  list.value = await api('/disputes' + p)
}
onMounted(load)
</script>
