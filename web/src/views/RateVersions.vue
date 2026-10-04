<template>
  <div class="card">
    <h3>合同费率版本(财务调整按业务发生日匹配)</h3>
    <table class="grid">
      <thead><tr><th>版本</th><th>生效起</th><th>生效止</th><th>每件运费</th><th>每件破损扣减</th><th>备注</th></tr></thead>
      <tbody>
        <tr v-for="v in list" :key="v.id">
          <td><span class="tag blue">{{ v.version }}</span></td>
          <td>{{ fmtDate(v.effective_from) }}</td>
          <td>{{ v.effective_to ? fmtDate(v.effective_to) : '至今' }}</td>
          <td class="mono">{{ fmtMoney(v.unit_price) }}</td>
          <td class="mono">{{ fmtMoney(v.damage_deduction) }}</td>
          <td>{{ v.remark }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { api, fmtDate, fmtMoney } from '../api'
const list = ref([])
onMounted(async () => { list.value = await api('/rate-versions') })
</script>
