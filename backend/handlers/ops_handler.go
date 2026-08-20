package handlers

import (
	"feed/cache"
	"feed/mq"
	"feed/utils"

	"github.com/gin-gonic/gin"
)

// OpsHandler 提供运维观测接口（只读）。
type OpsHandler struct{}

func NewOpsHandler() *OpsHandler { return &OpsHandler{} }

// MQMetrics 查看 MQ 熔断/降级指标快照。
// GET /api/ops/mq/metrics
func (h *OpsHandler) MQMetrics(c *gin.Context) {
	utils.Success(c, mq.SnapshotMetrics())
}

// CacheMetrics 查看缓存命中/失效指标快照。
// GET /api/ops/cache/metrics
func (h *OpsHandler) CacheMetrics(c *gin.Context) {
	utils.Success(c, cache.SnapshotCacheMetrics())
}
