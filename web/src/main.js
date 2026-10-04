import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'
import Login from './views/Login.vue'
import WaybillList from './views/WaybillList.vue'
import WaybillDetail from './views/WaybillDetail.vue'
import RateVersions from './views/RateVersions.vue'
import Adjustments from './views/Adjustments.vue'
import Disputes from './views/Disputes.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: Login },
    { path: '/', redirect: '/waybills' },
    { path: '/waybills', component: WaybillList },
    { path: '/waybills/:id', component: WaybillDetail },
    { path: '/rate-versions', component: RateVersions },
    { path: '/adjustments', component: Adjustments },
    { path: '/disputes', component: Disputes }
  ]
})

router.beforeEach((to) => {
  if (to.path !== '/login' && !localStorage.getItem('token')) return '/login'
})

createApp(App).use(router).mount('#app')
