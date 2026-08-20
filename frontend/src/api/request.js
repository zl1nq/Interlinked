import axios from 'axios'
import { ElMessage } from 'element-plus'
import router from '../router'

const request = axios.create({
    baseURL: '/api',
    timeout: 10000,
})

// 请求拦截器
request.interceptors.request.use(
    (config) => {
        const token = sessionStorage.getItem('token')
        if (token) {
            config.headers.Authorization = `Bearer ${token}`
        }
        return config
    },
    (error) => {
        return Promise.reject(error)
    }
)

// 响应拦截器
request.interceptors.response.use(
    (response) => {
        const res = response.data
        if (res.code !== 0) {
            ElMessage.error(res.message || '请求失败')
            if (res.code === 401) {
                sessionStorage.removeItem('token')
                sessionStorage.removeItem('user')
                router.push('/login')
            }
            return Promise.reject(new Error(res.message))
        }
        return res
    },
    (error) => {
        if (error.response) {
            if (error.response.status === 401) {
                sessionStorage.removeItem('token')
                sessionStorage.removeItem('user')
                router.push('/login')
                ElMessage.error('登录已过期，请重新登录')
            } else {
                ElMessage.error(error.response.data?.message || '网络错误')
            }
        } else {
            ElMessage.error('网络连接失败')
        }
        return Promise.reject(error)
    }
)

export default request