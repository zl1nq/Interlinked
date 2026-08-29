import { defineStore } from 'pinia'
import { ref } from 'vue'

// 通知未读数共享状态：Layout 角标与通知页的删除/已读操作联动，
// 删除单条后直接用响应的 unread_count 覆盖，无需再发请求
export const useNotificationStore = defineStore('notification', () => {
    const unreadCount = ref(0)

    function setUnreadCount(value) {
        unreadCount.value = Number(value) || 0
    }

    return {
        unreadCount,
        setUnreadCount,
    }
})
