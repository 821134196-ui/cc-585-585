<template>
  <div v-loading="loading">
    <template v-if="d.waybill">
      <!-- 运单概要 -->
      <el-card class="page-card">
        <div class="section-title">
          运单 {{ d.waybill.waybill_no }}
          <el-tag :type="statusTag(d.waybill.status)" effect="dark" size="small">{{ d.status_name }}</el-tag>
          <el-button size="small" text type="primary" style="margin-left:auto" @click="load">刷新</el-button>
        </div>
        <el-descriptions :column="4" border size="small">
          <el-descriptions-item label="客户">{{ d.waybill.customer }}</el-descriptions-item>
          <el-descriptions-item label="线路">{{ d.waybill.origin }} → {{ d.waybill.destination }}</el-descriptions-item>
          <el-descriptions-item label="承运司机">{{ d.waybill.driver?.name }}</el-descriptions-item>
          <el-descriptions-item label="业务发生时间">{{ fmtDate(d.waybill.business_date) }}</el-descriptions-item>
        </el-descriptions>
      </el-card>

      <!-- 并列显示:计划件数 / 实际签收 / 破损 / 合同运费 -->
      <el-row :gutter="16" class="stat-row">
        <el-col :span="6">
          <el-card class="stat-card">
            <div class="label">计划件数</div>
            <div class="value">{{ d.waybill.planned_pieces }}</div>
            <div class="sub">运单下达时的计划</div>
          </el-card>
        </el-col>
        <el-col :span="6">
          <el-card class="stat-card">
            <div class="label">实际签收件数</div>
            <div class="value" :style="{ color: signedColor }">{{ signedPiecesText }}</div>
            <div class="sub">{{ signedSub }}</div>
          </el-card>
        </el-col>
        <el-col :span="6">
          <el-card class="stat-card">
            <div class="label">破损情况</div>
            <div class="value" :style="{ color: damagedCount > 0 ? '#f56c6c' : '#67c23a' }">{{ damagedCount }} 件</div>
            <div class="sub">{{ damageNote || '无破损记录' }}</div>
          </el-card>
        </el-col>
        <el-col :span="6">
          <el-card class="stat-card">
            <div class="label">合同运费(原结算)</div>
            <div class="value" style="color:#e6a23c">¥{{ d.waybill.contract_freight.toFixed(2) }}</div>
            <div class="sub">原结算金额不可改,差额走调整单</div>
          </el-card>
        </el-col>
      </el-row>

      <el-row :gutter="16">
        <el-col :span="14">
          <!-- 回单区 -->
          <el-card class="page-card">
            <div class="section-title">回单记录
              <el-tag v-if="approvedReceipt" type="success" size="small">已生效回单 #{{ approvedReceipt.id }}</el-tag>
            </div>

            <!-- 补录表单 -->
            <template v-if="canSubmitReceipt">
              <el-alert type="warning" :closable="false" style="margin-bottom:12px">
                补录时间由服务器自动记录,<b>不能伪造为原签收时刻</b>;申报的签收时间仅作参考,晚于当前时间将被拒绝。
              </el-alert>
              <el-form label-width="110px" size="default">
                <el-form-item label="扫描识别">
                  <el-button @click="scanMock" :loading="scanning">📷 模拟扫描回单</el-button>
                  <span class="muted" style="margin-left:8px">本地模拟扫描接口,自动填充识别结果</span>
                </el-form-item>
                <el-form-item label="实际签收件数">
                  <el-input-number v-model="form.received" :min="0" :max="d.waybill.planned_pieces" />
                  <span class="muted" style="margin-left:8px">计划 {{ d.waybill.planned_pieces }} 件,少于计划即部分签收</span>
                </el-form-item>
                <el-form-item label="破损件数">
                  <el-input-number v-model="form.damaged" :min="0" :max="form.received" />
                </el-form-item>
                <el-form-item label="破损说明" v-if="form.damaged > 0">
                  <el-input v-model="form.damageNote" placeholder="破损情况说明" />
                </el-form-item>
                <el-form-item label="申报签收时间">
                  <el-date-picker v-model="form.declaredAt" type="datetime" placeholder="客户实际签收的时刻"
                    format="YYYY-MM-DD HH:mm" value-format="YYYY-MM-DDTHH:mm" :disabled-date="disableFuture" />
                </el-form-item>
                <el-form-item label="客户拒盖章">
                  <el-checkbox v-model="form.refusedSeal">客户拒绝盖章(将自动生成争议线索)</el-checkbox>
                </el-form-item>
                <el-form-item label="回单附件">
                  <input type="file" multiple accept="image/*,.pdf" @change="onFiles" />
                  <div class="muted">{{ form.files.length }} 个文件待上传,存本地 uploads/</div>
                </el-form-item>
                <el-form-item>
                  <el-button type="primary" :loading="submitting" @click="submitReceipt">提交补录</el-button>
                  <span class="muted" style="margin-left:8px">幂等键 {{ idemKey.slice(0, 8) }}…(重复提交不会产生第二条回单)</span>
                </el-form-item>
              </el-form>
            </template>
            <el-alert v-else-if="!approvedReceipt && d.receipts.length === 0" type="info" :closable="false"
              :title="isMyWaybill ? '可提交回单补录' : '等待司机提交回单'" style="margin-bottom:12px" />
            <el-alert v-else-if="hasPendingReceipt" type="warning" :closable="false" title="已有待审核回单,不能重复补录" style="margin-bottom:12px" />
            <el-alert v-else-if="approvedReceipt" type="success" :closable="false" title="回单已审核生效,重复回单将被系统拦截(不能生成第二次签收)" style="margin-bottom:12px" />

            <!-- 回单列表 -->
            <div v-for="r in d.receipts" :key="r.id" class="receipt-item">
              <div class="receipt-head">
                <el-tag :type="receiptTag(r.status)" effect="dark" size="small">{{ receiptStatusName(r.status) }}</el-tag>
                <b>回单 #{{ r.id }}</b>
                <span class="muted">提交人:{{ r.uploader?.name }}</span>
                <el-tag v-if="r.customer_refused_seal" type="danger" size="small" effect="plain">客户拒盖章</el-tag>
              </div>
              <el-descriptions :column="2" size="small" border style="margin:8px 0">
                <el-descriptions-item label="实收 / 破损">{{ r.received_pieces }} 件 / {{ r.damaged_pieces }} 件</el-descriptions-item>
                <el-descriptions-item label="破损说明">{{ r.damage_note || '—' }}</el-descriptions-item>
                <el-descriptions-item label="申报签收时间(司机填写)">{{ fmtTime(r.declared_signed_at) }}</el-descriptions-item>
                <el-descriptions-item label="实际补录时间(系统记录,不可伪造)">
                  {{ fmtTime(r.recorded_at) }}
                  <el-tag v-if="lagHours(r) > 24" type="warning" size="small" style="margin-left:6px">补录滞后 {{ lagHours(r) }} 小时</el-tag>
                </el-descriptions-item>
                <el-descriptions-item label="审核" :span="2">
                  <template v-if="r.status !== 'pending'">
                    {{ r.reviewer?.name }} 于 {{ fmtTime(r.reviewed_at) }}:{{ r.review_note || '—' }}
                  </template>
                  <template v-else>待审核</template>
                </el-descriptions-item>
                <el-descriptions-item label="附件" :span="2">
                  <template v-if="parseAtt(r).length">
                    <a v-for="(a, i) in parseAtt(r)" :key="i" :href="a" target="_blank" style="margin-right:10px">附件{{ i + 1 }}</a>
                  </template>
                  <span v-else class="muted">无</span>
                </el-descriptions-item>
              </el-descriptions>
              <!-- 审核按钮:后端强制校验,司机审核自己会被 403 拦截 -->
              <div v-if="r.status === 'pending'" class="review-bar">
                <el-button type="success" size="small" @click="review(r, true)">审核通过</el-button>
                <el-button type="danger" size="small" @click="review(r, false)">驳回</el-button>
                <span class="muted">仅运营/管理员可审核,且不能审核自己提交的回单(可切换司机账号点击验证拦截)</span>
              </div>
            </div>
          </el-card>

          <!-- 争议区 -->
          <el-card class="page-card">
            <div class="section-title">争议记录
              <el-button size="small" type="primary" plain style="margin-left:auto" @click="disputeDialog = true">发起争议</el-button>
            </div>
            <el-empty v-if="!d.disputes.length" description="暂无争议" :image-size="60" />
            <el-collapse v-else>
              <el-collapse-item v-for="dp in d.disputes" :key="dp.id">
                <template #title>
                  <el-tag :type="disputeTypeTag(dp.type)" size="small" effect="dark">{{ disputeTypeName(dp.type) }}</el-tag>
                  <span style="margin:0 8px">{{ dp.dispute_no }} · {{ dp.title }}</span>
                  <el-tag :type="disputeStatusTag(dp.status)" size="small">{{ disputeStatusName(dp.status) }}</el-tag>
                </template>
                <p class="muted">{{ dp.description }}</p>
                <p class="muted">发起人:{{ dp.creator?.name }} · {{ fmtTime(dp.created_at) }}</p>
                <el-timeline style="margin-top:10px">
                  <el-timeline-item v-for="hg in dp.handlings" :key="hg.id" :timestamp="fmtTime(hg.created_at)" placement="top">
                    <b>{{ hg.action }}</b>({{ hg.operator?.name }}):{{ hg.note }}
                  </el-timeline-item>
                </el-timeline>
                <div v-if="canHandle" class="handling-form">
                  <el-input v-model="handlingForms[dp.id].action" placeholder="处理动作,如:联系客户核实" style="width:220px" size="small" />
                  <el-input v-model="handlingForms[dp.id].note" placeholder="处理说明" style="width:260px" size="small" />
                  <el-select v-model="handlingForms[dp.id].status" placeholder="状态(可选)" size="small" style="width:130px" clearable>
                    <el-option label="处理中" value="processing" />
                    <el-option label="已解决" value="resolved" />
                    <el-option label="重新打开" value="open" />
                  </el-select>
                  <el-button size="small" type="primary" @click="addHandling(dp)">记录处理</el-button>
                </div>
              </el-collapse-item>
            </el-collapse>
          </el-card>
        </el-col>

        <el-col :span="10">
          <!-- 结算与调整 -->
          <el-card class="page-card">
            <div class="section-title">结算与运费调整</div>
            <template v-if="d.settlement">
              <div class="settle-box">
                <div>
                  <div class="muted">原结算金额(不可修改)</div>
                  <div style="font-size:22px;font-weight:700">¥{{ d.settlement.amount.toFixed(2) }}</div>
                </div>
                <el-tag :type="d.settlement.status === 'paid' ? 'success' : 'info'" effect="dark">
                  {{ d.settlement.status === 'paid' ? '已付款' : '未付款' }}
                </el-tag>
              </div>
              <div v-for="p in d.payments" :key="p.id" class="muted" style="margin:4px 0">
                💳 {{ fmtTime(p.paid_at) }} {{ p.operator?.name }} 支付 ¥{{ p.amount.toFixed(2) }}(模拟交易号 {{ p.txn_no }})
              </div>
              <el-button v-if="canPay && d.settlement.status !== 'paid'" type="primary" size="small"
                style="margin-top:8px" @click="pay">模拟支付(本地接口)</el-button>
            </template>

            <el-divider content-position="left">调整单(追加记录,不覆盖原结算)</el-divider>
            <div class="muted" style="margin-bottom:8px" v-if="d.applicable_rate">
              本运单业务时间 {{ fmtDate(d.waybill.business_date) }} 适用费率版本:
              <el-tag size="small" type="warning">{{ d.applicable_rate.version }}</el-tag>
              单价 ¥{{ d.applicable_rate.unit_price }}/件,破损扣 ¥{{ d.applicable_rate.damage_deduction_per_piece }}/件
            </div>
            <el-empty v-if="!d.adjustments.length" description="暂无调整单" :image-size="60" />
            <div v-for="a in d.adjustments" :key="a.id" class="adj-item">
              <div>
                <b>{{ a.adjust_no }}</b>
                <el-tag size="small" style="margin-left:6px">{{ a.rate_version_name }}</el-tag>
                <div :style="{ color: a.diff_amount >= 0 ? '#67c23a' : '#f56c6c', fontWeight: 700, fontSize: '16px' }">
                  {{ a.diff_amount >= 0 ? '+' : '' }}{{ a.diff_amount.toFixed(2) }} 元
                </div>
              </div>
              <div class="muted">
                实收 {{ a.received_pieces }} 件 × ¥{{ a.unit_price }} − 破损 {{ a.damaged_pieces }} 件 × ¥{{ a.damage_deduction }}
                = 应付 ¥{{ a.payable_amount.toFixed(2) }};已结算 ¥{{ a.already_settled.toFixed(2) }}
              </div>
              <el-popover placement="left" width="420" trigger="click">
                <template #reference><el-button size="small" text type="primary">计算依据</el-button></template>
                <p style="font-size:13px;line-height:1.7">{{ a.calc_detail }}</p>
                <p class="muted">{{ a.creator?.name }} · {{ fmtTime(a.created_at) }} · {{ a.note }}</p>
              </el-popover>
            </div>
            <el-button v-if="canAdjust" type="warning" plain size="small" style="margin-top:10px" :disabled="!approvedReceipt"
              @click="openAdjust">按费率版本生成调整单</el-button>
            <div v-if="canAdjust && !approvedReceipt" class="muted">需回单审核通过后才能生成调整单</div>
          </el-card>

          <!-- 审计日志 -->
          <el-card class="page-card">
            <div class="section-title">操作留痕</div>
            <el-timeline>
              <el-timeline-item v-for="log in d.audits" :key="log.id" :timestamp="fmtTime(log.created_at)" placement="top">
                <b>{{ log.actor_name }}</b> {{ log.action }}<br />
                <span class="muted">{{ log.detail }}</span>
              </el-timeline-item>
            </el-timeline>
          </el-card>
        </el-col>
      </el-row>
    </template>

    <!-- 发起争议对话框 -->
    <el-dialog v-model="disputeDialog" title="发起争议" width="480px">
      <el-form label-width="80px">
        <el-form-item label="争议类型">
          <el-radio-group v-model="disputeForm.type">
            <el-radio-button value="pieces">件数</el-radio-button>
            <el-radio-button value="damage">破损</el-radio-button>
            <el-radio-button value="rate">费率</el-radio-button>
            <el-radio-button value="other">其他</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="标题"><el-input v-model="disputeForm.title" /></el-form-item>
        <el-form-item label="详细说明"><el-input v-model="disputeForm.description" type="textarea" :rows="3" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="disputeDialog = false">取消</el-button>
        <el-button type="primary" @click="createDispute">提交</el-button>
      </template>
    </el-dialog>

    <!-- 调整单预览对话框 -->
    <el-dialog v-model="adjustDialog" title="生成运费调整单" width="560px">
      <template v-if="adjustPreview">
        <el-alert type="warning" :closable="false" style="margin-bottom:12px"
          title="按原业务发生时的费率版本计算,计算依据将随调整单永久保存;原结算记录不会被覆盖。" />
        <el-form inline size="small" style="margin-bottom:8px">
          <el-form-item label="核实签收件数">
            <el-input-number v-model="adjustForm.received" :min="0" :max="d.waybill.planned_pieces" />
          </el-form-item>
          <el-form-item label="核实破损件数">
            <el-input-number v-model="adjustForm.damaged" :min="0" :max="adjustForm.received" />
          </el-form-item>
        </el-form>
        <div class="muted" style="margin-bottom:8px">默认取回单记录;已付款后发现差额等场景,按争议核实结果修改并填写备注</div>
        <el-descriptions :column="1" border size="small">
          <el-descriptions-item label="费率版本">
            {{ d.applicable_rate.version }}(自 {{ fmtDate(d.applicable_rate.effective_from) }} 生效,业务发生 {{ fmtDate(d.waybill.business_date) }})
          </el-descriptions-item>
          <el-descriptions-item label="应付运费">
            {{ adjustForm.received }} 件 × ¥{{ d.applicable_rate.unit_price }}
            − {{ adjustForm.damaged }} 件 × ¥{{ d.applicable_rate.damage_deduction_per_piece }}
            = <b>¥{{ adjustPreview.payable.toFixed(2) }}</b>
          </el-descriptions-item>
          <el-descriptions-item label="已结算">¥{{ adjustPreview.already.toFixed(2) }}(原结算 + 历史调整)</el-descriptions-item>
          <el-descriptions-item label="本次调整">
            <b :style="{ color: adjustPreview.diff >= 0 ? '#67c23a' : '#f56c6c' }">
              {{ adjustPreview.diff >= 0 ? '+' : '' }}{{ adjustPreview.diff.toFixed(2) }} 元
            </b>
          </el-descriptions-item>
        </el-descriptions>
        <el-input v-model="adjustNote" placeholder="备注(修改核实件数时必填,如对应争议单号)" style="margin-top:12px" />
      </template>
      <template #footer>
        <el-button @click="adjustDialog = false">取消</el-button>
        <el-button type="warning" :loading="adjusting" @click="createAdjustment">确认生成</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import api, { showError } from '../api'
import { store } from '../store'

const route = useRoute()
const id = route.params.id
const loading = ref(false)
const d = ref({ receipts: [], payments: [], adjustments: [], disputes: [], audits: [] })

const form = reactive({ received: 0, damaged: 0, damageNote: '', declaredAt: '', refusedSeal: false, files: [] })
const idemKey = crypto.randomUUID()
const submitting = ref(false)
const scanning = ref(false)
const disputeDialog = ref(false)
const disputeForm = reactive({ type: 'pieces', title: '', description: '' })
const handlingForms = reactive({})
const adjustDialog = ref(false)
const adjustNote = ref('')
const adjusting = ref(false)
const adjustForm = reactive({ received: 0, damaged: 0 })

const me = computed(() => store.user)
const approvedReceipt = computed(() => d.value.receipts.find(r => r.status === 'approved'))
const hasPendingReceipt = computed(() => d.value.receipts.some(r => r.status === 'pending'))
const isMyWaybill = computed(() => d.value.waybill && me.value && d.value.waybill.driver_id === me.value.id)
const canSubmitReceipt = computed(() =>
  d.value.waybill && (me.value?.role === 'admin' || (me.value?.role === 'driver' && isMyWaybill.value)) &&
  !approvedReceipt.value && !hasPendingReceipt.value)
const canPay = computed(() => ['finance', 'admin'].includes(me.value?.role))
const canAdjust = computed(() => ['finance', 'admin'].includes(me.value?.role))
const canHandle = computed(() => ['ops', 'finance', 'admin'].includes(me.value?.role))

const damagedCount = computed(() => approvedReceipt.value?.damaged_pieces ?? d.value.receipts[0]?.damaged_pieces ?? 0)
const damageNote = computed(() => approvedReceipt.value?.damage_note || d.value.receipts[0]?.damage_note || '')
const signedPiecesText = computed(() => {
  const r = approvedReceipt.value || d.value.receipts[0]
  return r ? String(r.received_pieces) : '—'
})
const signedColor = computed(() => {
  const r = approvedReceipt.value || d.value.receipts[0]
  if (!r) return '#909399'
  return r.received_pieces < d.value.waybill.planned_pieces ? '#f56c6c' : '#67c23a'
})
const signedSub = computed(() => {
  const r = approvedReceipt.value
  if (!r) return d.value.receipts.length ? '回单待审核' : '暂无回单'
  const diff = d.value.waybill.planned_pieces - r.received_pieces
  return diff > 0 ? `部分签收,少 ${diff} 件` : '全部签收'
})

const adjustPreview = computed(() => {
  if (!d.value.applicable_rate || !approvedReceipt.value) return null
  const rv = d.value.applicable_rate
  const payable = round2(adjustForm.received * rv.unit_price - adjustForm.damaged * rv.damage_deduction_per_piece)
  const prevSum = d.value.adjustments.reduce((s, a) => s + a.diff_amount, 0)
  const already = round2(d.value.settlement.amount + prevSum)
  return { payable, already, diff: round2(payable - already) }
})

function round2(f) { return Math.round(f * 100) / 100 }
function fmtDate(s) { return s ? s.slice(0, 10) : '' }
function fmtTime(s) { return s ? s.slice(0, 16).replace('T', ' ') : '—' }
function statusTag(s) {
  return { pending_receipt: 'info', receipt_pending: 'warning', signed: 'success', partial_signed: 'danger' }[s] || 'info'
}
function receiptTag(s) { return { pending: 'warning', approved: 'success', rejected: 'danger' }[s] }
function receiptStatusName(s) { return { pending: '待审核', approved: '已审核', rejected: '已驳回' }[s] }
function disputeTypeTag(t) { return { pieces: 'warning', damage: 'danger', rate: 'primary', other: 'info' }[t] }
function disputeTypeName(t) { return { pieces: '件数', damage: '破损', rate: '费率', other: '其他' }[t] }
function disputeStatusTag(s) { return { open: 'danger', processing: 'warning', resolved: 'success' }[s] }
function disputeStatusName(s) { return { open: '待处理', processing: '处理中', resolved: '已解决' }[s] }
function parseAtt(r) { try { return JSON.parse(r.attachments || '[]') } catch { return [] } }
function lagHours(r) {
  return Math.round((new Date(r.recorded_at) - new Date(r.declared_signed_at)) / 3600000)
}
function disableFuture(date) { return date.getTime() > Date.now() }

async function load() {
  loading.value = true
  try {
    const { data } = await api.get('/waybills/' + id)
    d.value = data
    for (const dp of data.disputes) {
      if (!handlingForms[dp.id]) handlingForms[dp.id] = { action: '', note: '', status: '' }
    }
  } catch (e) {
    showError(e)
  } finally {
    loading.value = false
  }
}

async function scanMock() {
  scanning.value = true
  try {
    const { data } = await api.post('/scan/mock', { waybill_no: d.value.waybill.waybill_no })
    form.received = data.received_pieces
    form.damaged = data.damaged_pieces
    form.declaredAt = data.declared_signed_at
    ElMessage.success(data.message)
  } catch (e) {
    showError(e)
  } finally {
    scanning.value = false
  }
}

function onFiles(e) { form.files = Array.from(e.target.files) }

async function submitReceipt() {
  if (!form.declaredAt) return ElMessage.warning('请选择申报签收时间')
  submitting.value = true
  try {
    const fd = new FormData()
    fd.append('received_pieces', form.received)
    fd.append('damaged_pieces', form.damaged)
    fd.append('damage_note', form.damageNote)
    fd.append('customer_refused_seal', form.refusedSeal)
    fd.append('declared_signed_at', form.declaredAt)
    fd.append('idempotency_key', idemKey)
    for (const f of form.files) fd.append('files', f)
    const { data } = await api.post(`/waybills/${id}/receipts`, fd)
    ElMessage.success(data.message + `;系统补录时间 ${fmtTime(data.receipt.recorded_at)}`)
    load()
  } catch (e) {
    showError(e) // 409:重复回单拦截会在这里提示
  } finally {
    submitting.value = false
  }
}

async function review(r, approve) {
  try {
    const { value } = await ElMessageBox.prompt('审核意见', approve ? '审核通过' : '驳回回单', {
      confirmButtonText: '确定', cancelButtonText: '取消', inputPlaceholder: '选填'
    })
    try {
      const { data } = await api.post(`/receipts/${r.id}/review`, { approve, note: value || '' })
      ElMessage.success(data.message)
      load()
    } catch (e) {
      showError(e) // 403:司机审核自己补录会被拦截
    }
  } catch { /* 取消 */ }
}

async function pay() {
  try {
    await ElMessageBox.confirm(`确认通过本地模拟支付接口付款 ¥${d.value.settlement.amount.toFixed(2)}?`, '模拟支付')
    const { data } = await api.post(`/settlements/${d.value.settlement.id}/pay`)
    ElMessage.success(data.message + ',交易号 ' + data.payment.txn_no)
    load()
  } catch (e) {
    if (e !== 'cancel') showError(e)
  }
}

function openAdjust() {
  adjustForm.received = approvedReceipt.value.received_pieces
  adjustForm.damaged = approvedReceipt.value.damaged_pieces
  adjustDialog.value = true
}

async function createAdjustment() {
  adjusting.value = true
  try {
    const r = approvedReceipt.value
    const body = { note: adjustNote.value, expected_payable: adjustPreview.value.payable }
    if (adjustForm.received !== r.received_pieces || adjustForm.damaged !== r.damaged_pieces) {
      body.override_received_pieces = adjustForm.received
      body.override_damaged_pieces = adjustForm.damaged
    }
    const { data } = await api.post(`/waybills/${id}/adjustments`, body)
    ElMessage.success(data.message + ':' + data.adjustment.adjust_no)
    adjustDialog.value = false
    load()
  } catch (e) {
    showError(e)
  } finally {
    adjusting.value = false
  }
}

async function createDispute() {
  if (!disputeForm.title) return ElMessage.warning('请填写标题')
  try {
    const { data } = await api.post(`/waybills/${id}/disputes`, disputeForm)
    ElMessage.success(data.message)
    disputeDialog.value = false
    disputeForm.title = ''
    disputeForm.description = ''
    load()
  } catch (e) {
    showError(e)
  }
}

async function addHandling(dp) {
  const f = handlingForms[dp.id]
  if (!f.action) return ElMessage.warning('请填写处理动作')
  try {
    const { data } = await api.post(`/disputes/${dp.id}/handlings`, f)
    ElMessage.success(data.message)
    f.action = ''
    f.note = ''
    f.status = ''
    load()
  } catch (e) {
    showError(e)
  }
}

onMounted(load)
</script>

<style scoped>
.receipt-item { border: 1px solid #ebeef5; border-radius: 6px; padding: 12px; margin-bottom: 12px; }
.receipt-head { display: flex; align-items: center; gap: 10px; }
.review-bar { display: flex; align-items: center; gap: 8px; }
.settle-box { display: flex; justify-content: space-between; align-items: center; background: #fafafa; border-radius: 6px; padding: 10px 14px; }
.adj-item { display: flex; justify-content: space-between; align-items: center; border-bottom: 1px dashed #ebeef5; padding: 8px 0; }
.handling-form { display: flex; gap: 8px; margin-top: 10px; flex-wrap: wrap; }
</style>
