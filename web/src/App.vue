<template>
  <div>
    <header v-if="user" class="topbar">
      <div class="brand">🚚 回单补录与运费争议系统</div>
      <nav>
        <router-link to="/waybills">运单</router-link>
        <router-link to="/adjustments">调整单</router-link>
        <router-link to="/disputes">争议</router-link>
        <router-link to="/rate-versions">费率版本</router-link>
      </nav>
      <div class="userbox">
        <span class="role-tag" :class="user.role">{{ roleName }}</span>
        <span>{{ user.name }}</span>
        <a href="#" @click.prevent="logout">退出</a>
      </div>
    </header>
    <main :class="{ 'with-nav': user }">
      <router-view />
    </main>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { currentUser } from './api'

const route = useRoute()
const router = useRouter()
const user = ref(currentUser())
watch(() => route.path, () => { user.value = currentUser() })

const roleName = computed(() => ({ driver: '司机', ops: '运营', finance: '财务' }[user.value?.role] || ''))

function logout() {
  localStorage.removeItem('token')
  localStorage.removeItem('user')
  router.push('/login')
}
</script>

<style>
* { box-sizing: border-box; margin: 0; padding: 0; }
body { font-family: -apple-system, 'PingFang SC', 'Microsoft YaHei', sans-serif; background: #f0f3f8; color: #2b3445; }
a { color: #2563eb; text-decoration: none; }

.topbar { display: flex; align-items: center; gap: 28px; background: #1e3a5f; color: #fff; padding: 0 24px; height: 56px; position: sticky; top: 0; z-index: 10; }
.brand { font-weight: 700; font-size: 16px; }
.topbar nav { display: flex; gap: 4px; flex: 1; }
.topbar nav a { color: #c8d6e8; padding: 6px 14px; border-radius: 6px; font-size: 14px; }
.topbar nav a.router-link-active { background: rgba(255,255,255,.15); color: #fff; }
.userbox { display: flex; align-items: center; gap: 10px; font-size: 14px; }
.userbox a { color: #9fb8d4; font-size: 13px; }
.role-tag { font-size: 12px; padding: 2px 8px; border-radius: 10px; background: #64748b; }
.role-tag.driver { background: #0e9f6e; }
.role-tag.ops { background: #2563eb; }
.role-tag.finance { background: #b45309; }

main.with-nav { max-width: 1180px; margin: 24px auto; padding: 0 20px 40px; }

.card { background: #fff; border-radius: 10px; padding: 20px 24px; margin-bottom: 18px; box-shadow: 0 1px 3px rgba(20,40,80,.08); }
.card h3 { font-size: 15px; margin-bottom: 14px; color: #1e3a5f; display: flex; align-items: center; gap: 8px; }
.card h3::before { content: ''; width: 4px; height: 14px; background: #2563eb; border-radius: 2px; }

table.grid { width: 100%; border-collapse: collapse; font-size: 14px; }
table.grid th { text-align: left; padding: 10px 12px; background: #f6f8fb; color: #5a6b85; font-weight: 600; border-bottom: 2px solid #e4e9f2; white-space: nowrap; }
table.grid td { padding: 10px 12px; border-bottom: 1px solid #eef1f6; }
table.grid tr:hover td { background: #f8faff; }

.btn { display: inline-block; border: none; border-radius: 6px; padding: 7px 16px; font-size: 13px; cursor: pointer; background: #2563eb; color: #fff; }
.btn:hover { background: #1d4ed8; }
.btn.ghost { background: #eef2f9; color: #33415e; }
.btn.ghost:hover { background: #dfe7f5; }
.btn.danger { background: #dc2626; }
.btn.sm { padding: 4px 10px; font-size: 12px; }
.btn:disabled { opacity: .5; cursor: not-allowed; }

.tag { display: inline-block; font-size: 12px; padding: 2px 10px; border-radius: 10px; white-space: nowrap; }
.tag.blue { background: #dbeafe; color: #1d4ed8; }
.tag.green { background: #d1fae5; color: #047857; }
.tag.orange { background: #ffedd5; color: #c2410c; }
.tag.red { background: #fee2e2; color: #b91c1c; }
.tag.gray { background: #e2e8f0; color: #475569; }

.form-row { display: flex; gap: 14px; flex-wrap: wrap; margin-bottom: 12px; }
.form-item { display: flex; flex-direction: column; gap: 5px; }
.form-item label { font-size: 12px; color: #5a6b85; }
input, select, textarea { border: 1px solid #d5dcea; border-radius: 6px; padding: 7px 10px; font-size: 13px; font-family: inherit; }
input:focus, select:focus, textarea:focus { outline: 2px solid #bfdbfe; border-color: #2563eb; }

.alert { padding: 10px 14px; border-radius: 6px; font-size: 13px; margin-bottom: 12px; }
.alert.error { background: #fef2f2; color: #b91c1c; border: 1px solid #fecaca; }
.alert.ok { background: #f0fdf4; color: #15803d; border: 1px solid #bbf7d0; }
.alert.info { background: #eff6ff; color: #1d4ed8; border: 1px solid #bfdbfe; }

.stat-row { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 12px; }
.stat { background: #f6f8fb; border-radius: 8px; padding: 12px 16px; }
.stat .label { font-size: 12px; color: #5a6b85; margin-bottom: 4px; }
.stat .value { font-size: 20px; font-weight: 700; color: #1e3a5f; }
.stat .value.warn { color: #c2410c; }
.stat .value.money { color: #b45309; }

.muted { color: #8a97ab; font-size: 12px; }
.mono { font-family: ui-monospace, monospace; font-size: 12px; }
</style>
