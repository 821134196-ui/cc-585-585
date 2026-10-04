<template>
  <div class="card">
    <h3>运费调整单</h3>
    <table class="grid">
      <thead><tr><th>调整单号</th><th>运单</th><th>签收件数</th><th>破损</th><th>调整后金额</th><th>原结算</th><th>差额</th><th>费率版本</th><th>经办</th><th>状态</th><th>计算依据</th></tr></thead>
      <tbody>
        <tr v-for="a in list" :key="a.id">
          <td class="mono">{{ a.adjust_no }}</td>
          <td><router-link :to="'/waybills/' + a.waybill_id" class="mono">{{ a.waybill?.waybill_no }}</router-link></td>
          <td><b>{{ a.signed_pieces }}</b></td>
          <td>{{ a.damaged_pieces }}</td>
          <td class="mono">{{ fmtMoney(a.adjusted_amount) }}</td>
          <td class="mono">{{ fmtMoney(a.original_amount) }}</td>
          <td class="mono" :style="{ color: a.difference < 0 ? '#b91c1c' : '#047857', fontWeight: 700 }">{{ a.difference > 0 ? '+' : '' }}{{ fmtMoney(a.difference) }}</td>
          <td><span class="tag gray">{{ a.rate_version?.version }}</span></td>
          <td>{{ a.created_by?.name }}</td>
          <td><span class="tag" :class="a.status === 'paid' ? 'green' : 'blue'">{{ a.status === 'paid' ? '差额已支付' : '已确认' }}</span></td>
          <td><button class="btn sm ghost" @click="toggle(a)">查看</button></td>
        </tr>
        <tr v-if="list.length === 0"><td colspan="11" class="muted" style="text-align:center;padding:30px">暂无调整单</td></tr>
      </tbody>
    </table>
    <div v-if="basis" class="alert info mono" style="white-space:pre-wrap;margin-top:12px">{{ basis }}</div>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { api, fmtMoney } from '../api'
const list = ref([])
const basis = ref('')
function toggle(a) { basis.value = JSON.stringify(JSON.parse(a.basis), null, 2) }
onMounted(async () => { list.value = await api('/adjustments') })
</script>
