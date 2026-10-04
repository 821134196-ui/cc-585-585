<template>
  <div>
    <el-card class="page-card">
      <div class="section-title">运单管理</div>
      <el-form inline>
        <el-form-item label="状态">
          <el-select v-model="filters.status" clearable placeholder="全部" style="width:140px" @change="load">
            <el-option label="待回单" value="pending_receipt" />
            <el-option label="回单待审核" value="receipt_pending" />
            <el-option label="已签收" value="signed" />
            <el-option label="部分签收" value="partial_signed" />
          </el-select>
        </el-form-item>
        <el-form-item label="关键字">
          <el-input v-model="filters.q" placeholder="运单号 / 客户" style="width:200px" clearable @change="load" />
        </el-form-item>
        <el-form-item v-if="store.user?.role === 'driver'">
          <el-checkbox v-model="filters.mine" label="只看我承运的" @change="load" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="load">查询</el-button>
        </el-form-item>
      </el-form>
      <el-table :data="rows" v-loading="loading" stripe @row-click="go" style="cursor:pointer">
        <el-table-column prop="waybill_no" label="运单号" width="150" />
        <el-table-column prop="customer" label="客户" width="110" />
        <el-table-column label="线路" width="140">
          <template #default="{ row }">{{ row.origin }} → {{ row.destination }}</template>
        </el-table-column>
        <el-table-column label="业务日期" width="105">
          <template #default="{ row }">{{ fmtDate(row.business_date) }}</template>
        </el-table-column>
        <el-table-column prop="driver.name" label="司机" width="120" />
        <el-table-column label="计划件数" width="85" align="center" prop="planned_pieces" />
        <el-table-column label="实收/破损" width="95" align="center">
          <template #default="{ row }">
            <span v-if="row.received_pieces != null">
              {{ row.received_pieces }} / <span :style="{ color: row.damaged_pieces > 0 ? '#f56c6c' : 'inherit' }">{{ row.damaged_pieces }}</span>
            </span>
            <span v-else class="muted">—</span>
          </template>
        </el-table-column>
        <el-table-column label="合同运费" width="100" align="right">
          <template #default="{ row }">¥{{ row.contract_freight.toFixed(2) }}</template>
        </el-table-column>
        <el-table-column label="运单状态" width="105" align="center">
          <template #default="{ row }">
            <el-tag :type="statusTag(row.status)" size="small">{{ row.status_name }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="结算" width="80" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.settlement_status === 'paid'" type="success" size="small">已付款</el-tag>
            <el-tag v-else type="info" size="small">未付款</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="回单" width="90" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.receipt_status === 'approved'" type="success" size="small">已审核</el-tag>
            <el-tag v-else-if="row.receipt_status === 'pending'" type="warning" size="small">待审核</el-tag>
            <el-tag v-else-if="row.receipt_status === 'rejected'" type="danger" size="small">已驳回</el-tag>
            <span v-else class="muted">无</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="80" align="center">
          <template #default="{ row }">
            <el-button size="small" text type="primary" @click.stop="go(row)">详情</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import api, { showError } from '../api'
import { store } from '../store'

const router = useRouter()
const rows = ref([])
const loading = ref(false)
const filters = reactive({ status: '', q: '', mine: false })

function fmtDate(s) { return s ? s.slice(0, 10) : '' }
function statusTag(s) {
  return { pending_receipt: 'info', receipt_pending: 'warning', signed: 'success', partial_signed: 'danger' }[s] || 'info'
}
async function load() {
  loading.value = true
  try {
    const { data } = await api.get('/waybills', {
      params: { status: filters.status, q: filters.q, mine: filters.mine ? 1 : '' }
    })
    rows.value = data
  } catch (e) {
    showError(e)
  } finally {
    loading.value = false
  }
}
function go(row) { router.push('/waybills/' + row.id) }
onMounted(load)
</script>
