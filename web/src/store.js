import { reactive } from 'vue'

export const store = reactive({
  user: JSON.parse(localStorage.getItem('user') || 'null'),
  login(token, user) {
    localStorage.setItem('token', token)
    localStorage.setItem('user', JSON.stringify(user))
    this.user = user
  },
  logout() {
    localStorage.removeItem('token')
    localStorage.removeItem('user')
    this.user = null
  }
})

export const ROLE_NAMES = { driver: '司机', ops: '运营', finance: '财务', admin: '管理员' }
