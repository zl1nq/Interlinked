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
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// 1. 初始化配置
	if err := config.InitConfig(); err != nil {
		log.Fatalf("Init config failed: %v", err)
	}
	log.Println("✅ Config loaded successfully")

	// 2. 初始化数据库
	if err := models.InitDB(); err != nil {
		log.Fatalf("Init database failed: %v", err)
	}
	log.Println("✅ Database initialized successfully")

	// 3. 初始化Redis
	if err := cache.InitRedis(); err != nil {
		log.Fatalf("Init redis failed: %v", err)
	}
	log.Println("✅ Redis initialized successfully")

	// 4. 初始化消息队列
	if err := mq.InitMQ(); err != nil {
		log.Fatalf("Init rabbitmq failed: %v", err)
	}
	log.Println("✅ Message queue initialized successfully")

	// 5. 初始化布隆过滤器（防缓存穿透）
	if err := cache.InitBloomFilters(); err != nil {
		log.Printf("⚠️ Init bloom filters failed, fallback without bloom: %v", err)
	} else {
		log.Println("✅ Bloom filters initialized successfully")
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
		log.Printf("Server forced to shutdown: %v", err)
	}
	log.Println("Server exited")
}
