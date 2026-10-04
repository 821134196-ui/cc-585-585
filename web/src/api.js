const BASE = '/api'

export function currentUser() {
  const s = localStorage.getItem('user')
  return s ? JSON.parse(s) : null
}

export async function api(path, opts = {}) {
  const headers = opts.headers || {}
  const token = localStorage.getItem('token')
  if (token) headers['Authorization'] = 'Bearer ' + token
  if (opts.json !== undefined) {
    headers['Content-Type'] = 'application/json'
    opts.body = JSON.stringify(opts.json)
  }
  const res = await fetch(BASE + path, { ...opts, headers })
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    const err = new Error(data.error || ('请求失败 ' + res.status))
    err.status = res.status
    throw err
  }
  return data
}

export function fmtTime(s) {
  if (!s) return '—'
  const d = new Date(s)
  const p = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}

export function fmtDate(s) {
  if (!s) return '—'
  return fmtTime(s).slice(0, 10)
}

export function fmtMoney(n) {
  if (n === null || n === undefined) return '—'
  return '¥' + Number(n).toFixed(2)
}

export const WAYBILL_STATUS = {
  in_transit: '运输中',
  receipt_pending: '回单待审核',
  signed: '已签收',
  adjusted: '已调整'
}

export const RECEIPT_STATUS = {
  pending: '待审核',
  approved: '已通过',
  rejected: '已驳回'
}

export const DISPUTE_TYPE = {
  pieces: '件数争议',
  damage: '破损争议',
  rate: '费率争议'
}

export const DISPUTE_STATUS = {
  open: '待处理',
  processing: '处理中',
  resolved: '已办结'
}
