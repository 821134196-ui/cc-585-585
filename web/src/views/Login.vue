<template>
  <div class="login-wrap">
    <el-card class="login-card">
      <div class="login-head">
        <span style="font-size:36px">🚚</span>
        <h2>物流回单补录与运费争议系统</h2>
        <p class="muted">司机补录 · 运营核验 · 财务调整 · 全程留痕</p>
      </div>
      <el-form @submit.prevent="doLogin">
        <el-form-item>
          <el-input v-model="username" placeholder="用户名" size="large" />
        </el-form-item>
        <el-form-item>
          <el-input v-model="password" type="password" placeholder="密码" size="large" @keyup.enter="doLogin" />
        </el-form-item>
        <el-button type="primary" size="large" style="width:100%" :loading="loading" @click="doLogin">登 录</el-button>
      </el-form>
      <el-divider content-position="left">演示账号(点击填充)</el-divider>
      <div class="acc-list">
        <el-tag v-for="a in accounts" :key="a.username" class="acc" :type="tagType(a.role)" effect="plain" @click="fill(a)">
          {{ a.name }} {{ a.username }}
        </el-tag>
      </div>
      <el-alert type="info" :closable="false" class="demo-hint">
        <p><b>三大演示场景</b></p>
        <p>① 运单 WB20250310001:部分签收 95/100 件 + 破损 3 件 → 财务按 V2025H1 费率生成调整单</p>
        <p>② 运单 WB20250805002:已有待审回单 → 重复补录被拦截;司机张师傅审核自己被拦截</p>
        <p>③ 运单 WB20240615003:已付款后发现件数差额 → 争议留痕 + 补调整单,原结算不覆盖</p>
      </el-alert>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import api, { showError } from '../api'
import { store } from '../store'

const router = useRouter()
const username = ref('')
const password = ref('')
const loading = ref(false)
const accounts = ref([])

onMounted(async () => {
  try {
    const { data } = await api.get('/accounts')
    accounts.value = data
  } catch (e) { /* 忽略,登录时再看错误 */ }
})

function tagType(role) {
  return { driver: 'success', ops: 'warning', finance: 'danger', admin: 'info' }[role] || 'info'
}
function fill(a) {
  username.value = a.username
  password.value = a.password
}
async function doLogin() {
  if (!username.value || !password.value) return ElMessage.warning('请输入用户名和密码')
  loading.value = true
  try {
    const { data } = await api.post('/login', { username: username.value, password: password.value })
    store.login(data.token, data.user)
    ElMessage.success('欢迎,' + data.user.name)
    router.push('/')
  } catch (e) {
    showError(e)
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-wrap { min-height: 100vh; display: flex; align-items: center; justify-content: center; background: linear-gradient(135deg, #001529 0%, #003a70 100%); }
.login-card { width: 460px; border-radius: 10px; }
.login-head { text-align: center; margin-bottom: 18px; }
.login-head h2 { margin: 8px 0 4px; font-size: 20px; }
.acc-list { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 14px; }
.acc { cursor: pointer; }
.demo-hint { font-size: 12px; }
.demo-hint p { margin: 2px 0; }
</style>
