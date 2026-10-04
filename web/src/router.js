import { createRouter, createWebHistory } from 'vue-router'
import Login from './views/Login.vue'
import WaybillList from './views/WaybillList.vue'
import WaybillDetail from './views/WaybillDetail.vue'
import RateVersions from './views/RateVersions.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: Login },
    { path: '/', component: WaybillList },
    { path: '/waybills/:id', component: WaybillDetail },
    { path: '/rates', component: RateVersions }
  ]
})

router.beforeEach(to => {
  if (to.path !== '/login' && !localStorage.getItem('token')) return '/login'
})

export default router
