// Package main 为应用启动入口。
// 启动顺序：配置 -> 数据库 -> Redis -> RabbitMQ -> 路由服务。
package main

import (
	"context"
	"errors"
	"feed/cache"
	"feed/config"
	"feed/models"
	"feed/mq"
	"feed/router"
	"feed/services"
	"feed/utils"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	//	0. 初始化颜色打印样式
	utils.InitColorPrint()

	// 1. 初始化配置
	if err := config.InitConfig(); err != nil {
		log.Fatalf("Init config failed: %v", err)
	}
	utils.LogInfo("✅ Config loaded successfully")

	// 2. 初始化数据库
	if err := models.InitDB(); err != nil {
		log.Fatalf("Init database failed: %v", err)
	}
	utils.LogInfo("✅ Database initialized successfully")

	// 3. 初始化Redis
	if err := cache.InitRedis(); err != nil {
		utils.LogError(fmt.Sprintf("Init redis failed: %v", err))
	}
	utils.LogInfo("✅ Redis initialized successfully")

	// 4. 初始化消息队列
	if err := mq.InitMQ(); err != nil {
		utils.LogError(fmt.Sprintf("Init rabbitmq failed: %v", err))
	}
	utils.LogInfo("✅ Message queue initialized successfully")

	// 5. 初始化布隆过滤器（防缓存穿透）
	if err := cache.InitBloomFilters(); err != nil {
		log.Printf("⚠️ Init bloom filters failed, fallback without bloom: %v", err)
	} else {
		utils.LogInfo("✅ Bloom filters initialized successfully")
	}

	// 6. 启动可靠事件投递 worker
	workerCtx, stopWorkers := context.WithCancel(context.Background())
	defer stopWorkers()
	go services.NewOutboxService().Start(workerCtx)

	// 7. 设置路由并启动服务器
	r := router.SetupRouter()
	port := config.AppConfig.Server.Port
	addr := fmt.Sprintf(":%d", port)

	log.Printf("🚀 Feed System Server starting on %s", addr)
	log.Printf("📋 API Documentation: http://localhost:%d/api", port)
	log.Printf("📊 推拉混合策略:")
	log.Printf("   - 大V阈值: %d 粉丝", config.AppConfig.Feed.BigVThreshold)
	log.Printf("   - 推模式上限: %d 粉丝", config.AppConfig.Feed.PushFanLimit)
	log.Printf("   - 收件箱大小: %d", config.AppConfig.Feed.InboxMaxSize)
	log.Printf("   - 发件箱大小: %d", config.AppConfig.Feed.OutboxMaxSize)

	srv := &http.Server{
		Addr:    addr,
		Handler: r,
		// 慢连接（Slowloris、慢 body）会无限占用 goroutine 和 fd，四个超时缺一不可。
		// ReadTimeout 须覆盖最大上传（video_max_size: 60MB）在弱网下的传输时间；
		// WriteTimeout 起点是"请求头读完"，须 >= ReadTimeout，否则慢上传会出现
		// "body 传完但响应被掐"。
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       5 * time.Minute,
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       120 * time.Second, // 回收空闲 keep-alive 连接
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server start failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		utils.LogError(fmt.Sprintf("Server forced to shutdown: %v", err))
	}
	utils.LogInfo("Server exited")
}
