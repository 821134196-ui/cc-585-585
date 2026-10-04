import axios from 'axios'
import { ElMessage } from 'element-plus'

const api = axios.create({ baseURL: '/api', timeout: 15000 })

api.interceptors.request.use(cfg => {
  const t = localStorage.getItem('token')
  if (t) cfg.headers.Authorization = 'Bearer ' + t
  return cfg
})

api.interceptors.response.use(
  r => r,
  err => {
    if (err.response && err.response.status === 401) {
      localStorage.removeItem('token')
      localStorage.removeItem('user')
      if (location.pathname !== '/login') location.href = '/login'
    }
    return Promise.reject(err)
  }
)

// 统一错误提示,返回 false 便于调用方判断
export function showError(err) {
  const msg = (err.response && err.response.data && err.response.data.error) || err.message || '请求失败'
  ElMessage({ type: 'error', message: msg, duration: 4000, showClose: true })
  return false
}

export default api
