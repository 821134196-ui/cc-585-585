<template>
  <router-view v-if="isLoginPage" />
  <el-container v-else class="layout">
    <el-header class="header">
      <div class="brand" @click="$router.push('/')">
        <span class="logo">🚚</span>
        <span class="title">物流回单补录与运费争议系统</span>
      </div>
      <el-menu mode="horizontal" :default-active="$route.path" router class="nav" background-color="transparent">
        <el-menu-item index="/">运单管理</el-menu-item>
        <el-menu-item index="/rates">费率版本</el-menu-item>
      </el-menu>
      <div class="userbox" v-if="store.user">
        <el-tag :type="roleTag" effect="dark" size="small">{{ roleName }}</el-tag>
        <span class="uname">{{ store.user.name }}</span>
        <el-button size="small" text @click="logout">退出</el-button>
      </div>
    </el-header>
    <el-main class="main">
      <router-view />
    </el-main>
  </el-container>
</template>

<script setup>
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { store, ROLE_NAMES } from './store'

const route = useRoute()
const router = useRouter()
const isLoginPage = computed(() => route.path === '/login')
const roleName = computed(() => ROLE_NAMES[store.user?.role] || '')
const roleTag = computed(() => ({ driver: 'success', ops: 'warning', finance: 'danger', admin: 'info' }[store.user?.role] || 'info'))

function logout() {
  store.logout()
  router.push('/login')
}
</script>

<style>
* { box-sizing: border-box; }
body { margin: 0; font-family: 'Helvetica Neue', Helvetica, 'PingFang SC', 'Microsoft YaHei', Arial, sans-serif; background: #f0f2f5; }
.layout { min-height: 100vh; }
.header { display: flex; align-items: center; background: #001529; color: #fff; padding: 0 20px; height: 56px; }
.brand { display: flex; align-items: center; cursor: pointer; margin-right: 40px; }
.brand .logo { font-size: 24px; margin-right: 8px; }
.brand .title { font-size: 17px; font-weight: 600; letter-spacing: 1px; }
.nav { flex: 1; border-bottom: none !important; }
.nav .el-menu-item { color: rgba(255,255,255,.75); }
.nav .el-menu-item.is-active, .nav .el-menu-item:hover { color: #fff; background: rgba(255,255,255,.08) !important; }
.userbox { display: flex; align-items: center; gap: 10px; color: #fff; }
.userbox .uname { font-size: 14px; }
.userbox .el-button { color: rgba(255,255,255,.8); }
.main { padding: 20px; max-width: 1280px; margin: 0 auto; width: 100%; }
.page-card { margin-bottom: 16px; }
.stat-row { margin-bottom: 16px; }
.stat-card { text-align: center; border-radius: 8px; }
.stat-card .label { color: #909399; font-size: 13px; margin-bottom: 6px; }
.stat-card .value { font-size: 28px; font-weight: 700; }
.stat-card .sub { font-size: 12px; color: #909399; margin-top: 4px; }
.section-title { font-size: 15px; font-weight: 600; margin: 0 0 12px; display: flex; align-items: center; gap: 8px; }
.muted { color: #909399; font-size: 12px; }
</style>
