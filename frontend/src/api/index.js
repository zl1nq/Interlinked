// API 分层说明：
// - 仅在此处维护接口路径与参数映射；
// - 页面组件通过 api 模块调用，避免散落 request 细节；
// - 后续扩展（重试、埋点、统一错误）可在 request.js 与本文件集中演进。
import request from './request'

// ==================== 认证相关 ====================
export const authApi = {
    register(data) {
        return request.post('/auth/register', data)
    },
    login(data) {
        return request.post('/auth/login', data)
    },
}

// ==================== 用户相关 ====================
export const userApi = {
    getCurrentUser() {
        return request.get('/users/me')
    },
    updateProfile(data) {
        return request.put('/users/me', data)
    },
    getUserProfile(id) {
        return request.get(`/users/${id}`)
    },
    searchUsers(keyword, page = 1, pageSize = 20) {
        return request.get('/users/search', { params: { keyword, page, page_size: pageSize } })
    },
    getRecentVisits(page = 1, pageSize = 20) {
        return request.get('/users/me/visits', { params: { page, page_size: pageSize } })
    },
}

// ==================== 上传相关 ====================
export const uploadApi = {
    uploadImage(file) {
        const formData = new FormData()
        formData.append('file', file)
        return request.post('/upload/image', formData, {
            headers: { 'Content-Type': 'multipart/form-data' },
        })
    },
    uploadVideo(file) {
        const formData = new FormData()
        formData.append('file', file)
        return request.post('/upload/video', formData, {
            headers: { 'Content-Type': 'multipart/form-data' },
        })
    },
}

// ==================== 关注相关 ====================
export const followApi = {
    follow(userId) {
        return request.post(`/follow/${userId}`)
    },
    unfollow(userId) {
        return request.delete(`/follow/${userId}`)
    },
    getFollowers(userId, page = 1, pageSize = 20) {
        return request.get(`/users/${userId}/followers`, { params: { page, page_size: pageSize } })
    },
    getFollowing(userId, page = 1, pageSize = 20) {
        return request.get(`/users/${userId}/following`, { params: { page, page_size: pageSize } })
    },
}

// ==================== Feed相关 ====================
export const feedApi = {
    publish(data) {
        return request.post('/feeds', data)
    },
    repost(data) {
        return request.post('/feeds/repost', data)
    },
    deleteFeed(id) {
        return request.delete(`/feeds/${id}`)
    },
    getFeed(id) {
        return request.get(`/feeds/${id}`)
    },
    getUserFeeds(userId, page = 1, pageSize = 20) {
        return request.get(`/users/${userId}/feeds`, { params: { page, page_size: pageSize } })
    },
    searchFeeds(keyword, page = 1, pageSize = 20) {
        return request.get('/feeds/search', { params: { keyword, page, page_size: pageSize } })
    },
    // 游标分页：cursor 为下一页起始游标（首次传 0）
    getTimeline(cursor = 0, pageSize = 20) {
        return request.get('/timeline', { params: { cursor, page_size: pageSize } })
    },
    like(feedId) {
        return request.post(`/feeds/${feedId}/like`)
    },
    unlike(feedId) {
        return request.delete(`/feeds/${feedId}/like`)
    },
    getLikers(feedId, page = 1, pageSize = 20) {
        return request.get(`/feeds/${feedId}/likes`, { params: { page, page_size: pageSize } })
    },
    getComments(feedId, page = 1, pageSize = 20) {
        return request.get(`/feeds/${feedId}/comments`, { params: { page, page_size: pageSize } })
    },
    addComment(feedId, content) {
        return request.post(`/feeds/${feedId}/comments`, { content })
    },
    deleteComment(feedId, commentId) {
        return request.delete(`/feeds/${feedId}/comments/${commentId}`)
    },
}

// ==================== 消息相关 ====================
export const opsApi = {
    getMQMetrics() {
        return request.get('/ops/mq/metrics')
    },
    getCacheMetrics() {
        return request.get('/ops/cache/metrics')
    },
}

export const notificationApi = {
    getNotifications(page = 1, pageSize = 20) {
        return request.get('/notifications', { params: { page, page_size: pageSize } })
    },
    markAllRead() {
        return request.post('/notifications/read-all')
    },
}

export const messageApi = {
    connectMessageWS(token) {
        const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
        const base = import.meta.env.VITE_WS_BASE_URL || `${wsProtocol}//${window.location.host}`
        return new WebSocket(`${base}/api/ws/messages?token=${encodeURIComponent(token)}`)
    },
    getConversations(page = 1, pageSize = 50) {
        return request.get('/messages/conversations', { params: { page, page_size: pageSize } })
    },
    getHistory(targetUserId, page = 1, pageSize = 100) {
        return request.get(`/messages/history/${targetUserId}`, { params: { page, page_size: pageSize } })
    },
}
