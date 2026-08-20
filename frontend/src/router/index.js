import { createRouter, createWebHistory } from 'vue-router'

const routes = [
    {
        path: '/login',
        name: 'Login',
        component: () => import('../views/Login.vue'),
        meta: { requiresAuth: false },
    },
    {
        path: '/register',
        name: 'Register',
        component: () => import('../views/Register.vue'),
        meta: { requiresAuth: false },
    },
    {
        path: '/',
        name: 'Layout',
        component: () => import('../views/Layout.vue'),
        meta: { requiresAuth: true },
        children: [
            {
                path: '',
                redirect: '/discover',
            },
            {
                path: 'discover',
                name: 'Discover',
                component: () => import('../views/Discover.vue'),
            },
            {
                path: 'timeline',
                name: 'Timeline',
                component: () => import('../views/Timeline.vue'),
            },
            {
                path: 'publish',
                name: 'Publish',
                component: () => import('../views/Publish.vue'),
            },
            {
                path: 'profile/:id',
                name: 'Profile',
                component: () => import('../views/Profile.vue'),
            },
            {
                path: 'messages',
                name: 'Messages',
                component: () => import('../views/Messages.vue'),
            },
            {
                path: 'notifications',
                name: 'Notifications',
                component: () => import('../views/Notifications.vue'),
            },
            {
                path: 'search_result',
                name: 'SearchResult',
                component: () => import('../views/SearchResult.vue'),
            },
            {
                path: 'feed/:id',
                name: 'FeedDetail',
                component: () => import('../views/FeedDetail.vue'),
            },
            {
                path: 'ops',
                name: 'OpsPanel',
                component: () => import('../views/OpsPanel.vue'),
            },
        ],
    },
]

const router = createRouter({
    history: createWebHistory(),
    routes,
})

// 路由守卫
router.beforeEach((to, from, next) => {
    const token = sessionStorage.getItem('token')

    if (to.meta.requiresAuth !== false && !token) {
        next('/login')
    } else if ((to.path === '/login' || to.path === '/register') && token) {
        next('/')
    } else {
        next()
    }
})

export default router