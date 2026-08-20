import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { authApi, userApi } from '../api'

export const useUserStore = defineStore('user', () => {
    const token = ref(sessionStorage.getItem('token') || '')
    const userInfo = ref(JSON.parse(sessionStorage.getItem('user') || 'null'))

    const isLoggedIn = computed(() => !!token.value)

    async function login(username, password) {
        const res = await authApi.login({ username, password })
        token.value = res.data.token
        userInfo.value = res.data.user
        sessionStorage.setItem('token', res.data.token)
        sessionStorage.setItem('user', JSON.stringify(res.data.user))
        return res
    }

    async function register(username, password, nickname) {
        const res = await authApi.register({ username, password, nickname })
        token.value = res.data.token
        userInfo.value = res.data.user
        sessionStorage.setItem('token', res.data.token)
        sessionStorage.setItem('user', JSON.stringify(res.data.user))
        return res
    }

    function logout() {
        token.value = ''
        userInfo.value = null
        sessionStorage.removeItem('token')
        sessionStorage.removeItem('user')
    }

    async function fetchCurrentUser() {
        try {
            const res = await userApi.getCurrentUser()
            userInfo.value = res.data
            sessionStorage.setItem('user', JSON.stringify(res.data))
        } catch (e) {
            logout()
        }
    }

    return {
        token,
        userInfo,
        isLoggedIn,
        login,
        register,
        logout,
        fetchCurrentUser,
    }
})