<template>
  <div class="login-wrap">
    <div class="login-card">
      <h1>🚚 回单补录与运费争议系统</h1>
      <p class="sub">司机补录 · 运营核验 · 财务调整 · 全程可查</p>
      <div class="users">
        <button v-for="u in demoUsers" :key="u.username" class="user-btn" @click="login(u.username)">
          <span class="avatar" :class="u.role">{{ u.name[0] }}</span>
          <span class="uname">{{ u.name }}</span>
          <span class="urole">{{ u.roleName }} · {{ u.username }}</span>
        </button>
      </div>
      <div v-if="error" class="alert error">{{ error }}</div>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api'

const router = useRouter()
const error = ref('')
const demoUsers = [
  { username: 'driver1', name: '张师傅', role: 'driver', roleName: '司机' },
  { username: 'driver2', name: '李师傅', role: 'driver', roleName: '司机' },
  { username: 'ops1', name: '王运营', role: 'ops', roleName: '运营' },
  { username: 'finance1', name: '赵财务', role: 'finance', roleName: '财务' }
]

async function login(username) {
  error.value = ''
  try {
    const data = await api('/login', { method: 'POST', json: { username } })
    localStorage.setItem('token', data.token)
    localStorage.setItem('user', JSON.stringify(data.user))
    router.push('/waybills')
  } catch (e) {
    error.value = e.message
  }
}
</script>

<style scoped>
.login-wrap { min-height: 100vh; display: flex; align-items: center; justify-content: center; background: linear-gradient(135deg, #1e3a5f, #2563eb); }
.login-card { background: #fff; border-radius: 14px; padding: 40px 44px; width: 420px; box-shadow: 0 12px 40px rgba(0,0,0,.25); }
h1 { font-size: 20px; color: #1e3a5f; }
.sub { color: #8a97ab; font-size: 13px; margin: 8px 0 24px; }
.users { display: flex; flex-direction: column; gap: 10px; }
.user-btn { display: flex; align-items: center; gap: 12px; padding: 12px 16px; border: 1px solid #e4e9f2; border-radius: 10px; background: #fff; cursor: pointer; text-align: left; }
.user-btn:hover { border-color: #2563eb; background: #f8faff; }
.avatar { width: 38px; height: 38px; border-radius: 50%; color: #fff; display: flex; align-items: center; justify-content: center; font-weight: 700; background: #64748b; }
.avatar.driver { background: #0e9f6e; }
.avatar.ops { background: #2563eb; }
.avatar.finance { background: #b45309; }
.uname { font-weight: 600; }
.urole { margin-left: auto; font-size: 12px; color: #8a97ab; }
</style>
