<template>
  <div v-if="w">
    <!-- 运单信息: 计划/实际/破损/合同运费 并列 -->
    <div class="card">
      <h3>运单 {{ w.waybill_no }}
        <span class="tag" :class="statusClass(w.status)" style="margin-left:8px">{{ WAYBILL_STATUS[w.status] }}</span>
      </h3>
      <p class="muted" style="margin-bottom:14px">
        {{ w.customer_name }} · {{ w.route_from }} → {{ w.route_to }} · 司机 {{ w.driver?.name }} · 业务发生日 {{ fmtDate(w.business_date) }}
      </p>
      <div class="stat-row">
        <div class="stat"><div class="label">计划件数</div><div class="value">{{ w.planned_pieces }}</div></div>
        <div class="stat"><div class="label">实际签收</div>
          <div class="value" :class="{ warn: w.actual_pieces !== null && w.actual_pieces < w.planned_pieces }">
            {{ w.actual_pieces === null ? '未签收' : w.actual_pieces }}
          </div>
        </div>
        <div class="stat"><div class="label">破损件数</div>
          <div class="value" :class="{ warn: w.damaged_pieces > 0 }">{{ w.damaged_pieces }}</div>
        </div>
        <div class="stat"><div class="label">合同运费(原结算)</div>
          <div class="value money">{{ fmtMoney(w.settlement?.amount) }}</div>
        </div>
        <div class="stat"><div class="label">结算费率版本</div>
          <div class="value" style="font-size:16px">{{ w.settlement?.rate_version?.version || '—' }}</div>
        </div>
      </div>
    </div>

    <!-- 原始结算: 不可覆盖,只能支付或走调整单 -->
    <div class="card" v-if="w.settlement">
      <h3>原始结算(不可覆盖)</h3>
      <table class="grid">
        <thead><tr><th>结算件数</th><th>单价</th><th>金额</th><th>费率版本</th><th>支付状态</th><th>交易号</th><th></th></tr></thead>
        <tbody><tr>
          <td>{{ w.settlement.pieces }}</td>
          <td class="mono">{{ fmtMoney(w.settlement.unit_price) }}</td>
          <td class="mono"><b>{{ fmtMoney(w.settlement.amount) }}</b></td>
          <td><span class="tag gray">{{ w.settlement.rate_version?.version }}</span></td>
          <td><span class="tag" :class="w.settlement.status === 'paid' ? 'green' : 'orange'">{{ w.settlement.status === 'paid' ? '已支付' : '待支付' }}</span></td>
          <td class="mono">{{ w.settlement.pay_trade_no || '—' }}</td>
          <td>
            <button v-if="user.role === 'finance' && w.settlement.status === 'unpaid'" class="btn sm" @click="pay('settlement', w.settlement.id)">模拟支付</button>
          </td>
        </tr></tbody>
      </table>
      <p class="muted" style="margin-top:8px">已付款后若发现差额,请通过下方调整单处理,原结算记录始终保留。</p>
    </div>

    <!-- 司机: 回单补录 -->
    <div class="card" v-if="canUpload">
      <h3>回单补录(司机)</h3>
      <div class="alert info">实际补录时间由服务器自动记录,无法伪造为原签收时刻;“签收时间”仅作为您的声明保存。</div>
      <div v-if="err" class="alert error">{{ err }}</div>
      <div v-if="ok" class="alert ok">{{ ok }}</div>
      <div class="form-row">
        <div class="form-item"><label>实际签收件数(计划 {{ w.planned_pieces }})</label>
          <input type="number" v-model.number="form.signed" min="0" :max="w.planned_pieces" /></div>
        <div class="form-item"><label>破损件数</label>
          <input type="number" v-model.number="form.damaged" min="0" /></div>
        <div class="form-item"><label>客户签收时间(声明)</label>
          <input type="datetime-local" v-model="form.claimed" /></div>
        <div class="form-item"><label>盖章状态</label>
          <select v-model="form.stamp"><option value="stamped">客户已盖章</option><option value="refused">客户拒绝盖章</option></select></div>
        <div class="form-item" style="flex:1;min-width:220px"><label>备注</label>
          <input v-model="form.note" placeholder="如: 短少原因、现场情况" /></div>
        <div class="form-item"><label>回单照片(可多选)</label>
          <input type="file" multiple accept="image/*" @change="e => form.files = e.target.files" /></div>
      </div>
      <button class="btn" :disabled="submitting" @click="submitReceipt">{{ submitting ? '提交中…' : '提交补录' }}</button>
    </div>

    <!-- 回单记录 + 运营审核 -->
    <div class="card">
      <h3>回单记录</h3>
      <div v-if="receipts.length === 0" class="muted">暂无回单</div>
      <div v-for="r in receipts" :key="r.id" class="receipt">
        <div class="receipt-head">
          <span class="mono">RC-{{ r.id }}</span>
          <span class="tag" :class="{ pending: 'orange', approved: 'green', rejected: 'red' }[r.status]">{{ RECEIPT_STATUS[r.status] }}</span>
          <span class="muted">上传人: {{ r.uploader?.name }}</span>
          <span v-if="r.stamp_status === 'refused'" class="tag red">客户拒盖章</span>
        </div>
        <div class="receipt-body">
          <div><span class="muted">签收件数</span><b>{{ r.signed_pieces }}</b> / 计划 {{ w.planned_pieces }}
            <span v-if="r.signed_pieces < w.planned_pieces" class="tag orange" style="margin-left:6px">部分签收 短少 {{ w.planned_pieces - r.signed_pieces }} 件</span>
          </div>
          <div><span class="muted">破损</span><b :style="r.damaged_pieces > 0 ? 'color:#c2410c' : ''">{{ r.damaged_pieces }}</b> 件</div>
          <div><span class="muted">司机声称签收时间</span>{{ fmtTime(r.claimed_sign_time) }} <span class="tag gray">声明</span></div>
          <div><span class="muted">实际补录时间</span>{{ fmtTime(r.entry_time) }} <span class="tag blue">服务器盖章</span></div>
          <div v-if="r.note"><span class="muted">备注</span>{{ r.note }}</div>
          <div v-if="r.reviewer"><span class="muted">审核</span>{{ r.reviewer.name }} · {{ fmtTime(r.reviewed_at) }} · {{ r.review_note || '无意见' }}</div>
          <div v-if="r.attachments?.length" class="atts">
            <a v-for="a in r.attachments" :key="a.id" :href="'/uploads/' + a.stored_name" target="_blank">
              <img :src="'/uploads/' + a.stored_name" :alt="a.file_name" />
            </a>
          </div>
        </div>
        <!-- 运营审核: 自己上传的不能审 -->
        <div v-if="user.role === 'ops' && r.status === 'pending'" class="receipt-actions">
          <template v-if="r.uploader_id !== user.id">
            <input v-model="reviewNotes[r.id]" placeholder="审核意见" style="flex:1" />
            <button class="btn sm" @click="review(r, 'approve')">通过</button>
            <button class="btn sm danger" @click="review(r, 'reject')">驳回</button>
          </template>
          <span v-else class="tag red">不能审核自己上传的补录</span>
        </div>
        <div v-if="user.role === 'driver' && r.status === 'pending' && r.uploader_id === user.id" class="receipt-actions">
          <span class="muted">等待运营审核(司机不能审核自己的补录)</span>
        </div>
      </div>
    </div>

    <!-- 财务: 调整单 -->
    <div class="card">
      <h3>运费调整单(财务)</h3>
      <div v-if="user.role === 'finance' && approvedReceipt && adjustments.length === 0" style="margin-bottom:12px">
        <div class="alert info">
          将按业务发生日 <b>{{ fmtDate(w.business_date) }}</b> 匹配费率版本,签收件数以审核通过的回单({{ approvedReceipt.signed_pieces }} 件)为准。
        </div>
        <button class="btn" @click="createAdjustment">生成调整单</button>
      </div>
      <div v-if="adjustments.length === 0" class="muted">暂无调整单</div>
      <table v-else class="grid">
        <thead><tr><th>调整单号</th><th>签收件数</th><th>破损</th><th>单价</th><th>调整后金额</th><th>原结算</th><th>差额</th><th>费率版本</th><th>状态</th><th>计算依据</th><th></th></tr></thead>
        <tbody>
          <tr v-for="a in adjustments" :key="a.id">
            <td class="mono">{{ a.adjust_no }}</td>
            <td><b>{{ a.signed_pieces }}</b></td>
            <td>{{ a.damaged_pieces }}</td>
            <td class="mono">{{ fmtMoney(a.unit_price) }}</td>
            <td class="mono"><b>{{ fmtMoney(a.adjusted_amount) }}</b></td>
            <td class="mono">{{ fmtMoney(a.original_amount) }}</td>
            <td class="mono" :style="{ color: a.difference < 0 ? '#b91c1c' : '#047857', fontWeight: 700 }">{{ a.difference > 0 ? '+' : '' }}{{ fmtMoney(a.difference) }}</td>
            <td><span class="tag gray">{{ a.rate_version?.version }}</span></td>
            <td><span class="tag" :class="a.status === 'paid' ? 'green' : 'blue'">{{ a.status === 'paid' ? '差额已支付' : '已确认' }}</span></td>
            <td><button class="btn sm ghost" @click="showBasis(a)">查看</button></td>
            <td><button v-if="user.role === 'finance' && a.status !== 'paid'" class="btn sm" @click="pay('adjustment', a.id)">模拟支付差额</button></td>
          </tr>
        </tbody>
      </table>
      <div v-if="basis" class="alert info mono" style="white-space:pre-wrap;margin-top:12px">{{ basis }}</div>
    </div>

    <!-- 争议 -->
    <div class="card">
      <h3>争议记录</h3>
      <div v-if="disputes.length === 0" class="muted">暂无争议</div>
      <table v-else class="grid">
        <thead><tr><th>编号</th><th>类型</th><th>标题</th><th>详情</th><th>状态</th><th>处理结果</th><th></th></tr></thead>
        <tbody>
          <tr v-for="d in disputes" :key="d.id">
            <td class="mono">{{ d.dispute_no }}</td>
            <td><span class="tag" :class="{ pieces: 'orange', damage: 'red', rate: 'blue' }[d.type]">{{ DISPUTE_TYPE[d.type] }}</span></td>
            <td>{{ d.title }}</td>
            <td style="max-width:280px">{{ d.detail }}</td>
            <td><span class="tag" :class="d.status === 'resolved' ? 'green' : 'orange'">{{ DISPUTE_STATUS[d.status] }}</span></td>
            <td>{{ d.resolution || '—' }}</td>
            <td>
              <template v-if="(user.role === 'ops' || user.role === 'finance') && d.status !== 'resolved'">
                <input v-model="resolveNotes[d.id]" placeholder="处理结果" style="width:140px" />
                <button class="btn sm" @click="resolveDispute(d)">办结</button>
              </template>
            </td>
          </tr>
        </tbody>
      </table>
      <div class="form-row" style="margin-top:14px;align-items:flex-end">
        <div class="form-item"><label>类型</label>
          <select v-model="newDispute.type"><option value="pieces">件数争议</option><option value="damage">破损争议</option><option value="rate">费率争议</option></select></div>
        <div class="form-item" style="flex:1"><label>标题</label><input v-model="newDispute.title" /></div>
        <div class="form-item" style="flex:2"><label>详情</label><input v-model="newDispute.detail" /></div>
        <button class="btn ghost" @click="addDispute">登记争议</button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { api, currentUser, fmtDate, fmtMoney, fmtTime, WAYBILL_STATUS, RECEIPT_STATUS, DISPUTE_TYPE, DISPUTE_STATUS } from '../api'

const route = useRoute()
const user = currentUser()
const w = ref(null)
const adjustments = ref([])
const disputes = ref([])
const err = ref('')
const ok = ref('')
const submitting = ref(false)
const reviewNotes = ref({})
const resolveNotes = ref({})
const basis = ref('')
const form = ref({ signed: 0, damaged: 0, claimed: '', stamp: 'stamped', note: '', files: null })
const newDispute = ref({ type: 'pieces', title: '', detail: '' })

const receipts = computed(() => w.value?.receipts || [])
const approvedReceipt = computed(() => receipts.value.find(r => r.status === 'approved'))
const hasPending = computed(() => receipts.value.some(r => r.status === 'pending'))
const canUpload = computed(() =>
  user.role === 'driver' && w.value && w.value.driver_id === user.id && !approvedReceipt.value && !hasPending.value
)

async function load() {
  const data = await api('/waybills/' + route.params.id)
  w.value = data.waybill
  adjustments.value = data.adjustments
  disputes.value = data.disputes
}

async function submitReceipt() {
  err.value = ''; ok.value = ''
  if (!form.value.files || form.value.files.length === 0) { err.value = '请至少上传一张回单照片'; return }
  if (!form.value.claimed) { err.value = '请填写客户签收时间'; return }
  const fd = new FormData()
  fd.append('signed_pieces', form.value.signed)
  fd.append('damaged_pieces', form.value.damaged)
  fd.append('claimed_sign_time', form.value.claimed)
  fd.append('stamp_status', form.value.stamp)
  fd.append('note', form.value.note)
  for (const f of form.value.files) fd.append('files', f)
  submitting.value = true
  try {
    const data = await api('/waybills/' + w.value.id + '/receipts', { method: 'POST', body: fd })
    ok.value = data.message
    await load()
  } catch (e) { err.value = e.message } finally { submitting.value = false }
}

async function review(r, action) {
  err.value = ''
  try {
    const data = await api('/receipts/' + r.id + '/review', { method: 'POST', json: { action, note: reviewNotes.value[r.id] || '' } })
    ok.value = data.message
    await load()
  } catch (e) { alert(e.message) }
}

async function createAdjustment() {
  try {
    const data = await api('/waybills/' + w.value.id + '/adjustments', { method: 'POST' })
    ok.value = data.message
    await load()
  } catch (e) { alert(e.message) }
}

function showBasis(a) {
  basis.value = JSON.stringify(JSON.parse(a.basis), null, 2)
}

async function pay(type, id) {
  try {
    const data = await api('/mock/pay', { method: 'POST', json: { type, id } })
    alert(data.message + ' 交易号: ' + data.trade_no)
    await load()
  } catch (e) { alert(e.message) }
}

async function addDispute() {
  if (!newDispute.value.title) { alert('请填写标题'); return }
  await api('/waybills/' + w.value.id + '/disputes', { method: 'POST', json: newDispute.value })
  newDispute.value = { type: 'pieces', title: '', detail: '' }
  await load()
}

async function resolveDispute(d) {
  const resolution = resolveNotes.value[d.id]
  if (!resolution) { alert('请填写处理结果'); return }
  await api('/disputes/' + d.id + '/resolve', { method: 'POST', json: { resolution } })
  await load()
}

function statusClass(s) {
  return { in_transit: 'gray', receipt_pending: 'orange', signed: 'green', adjusted: 'blue' }[s] || 'gray'
}

onMounted(load)
</script>

<style scoped>
.receipt { border: 1px solid #e4e9f2; border-radius: 8px; padding: 14px 16px; margin-bottom: 12px; }
.receipt-head { display: flex; align-items: center; gap: 10px; margin-bottom: 10px; }
.receipt-body { display: flex; flex-direction: column; gap: 6px; font-size: 13px; }
.receipt-body .muted { display: inline-block; width: 110px; }
.receipt-actions { display: flex; gap: 8px; margin-top: 10px; align-items: center; }
.atts { display: flex; gap: 8px; flex-wrap: wrap; }
.atts img { width: 90px; height: 90px; object-fit: cover; border-radius: 6px; border: 1px solid #e4e9f2; }
</style>
