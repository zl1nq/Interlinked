package router

import (
	"feed/config"
	"feed/handlers"
	"feed/middleware"
	"feed/services"

	"github.com/gin-gonic/gin"
)

// SetupRouter 组装路由、服务依赖与中间件。
// 约定：
// - 先初始化 service，再注入 handler；
// - 受保护路由统一挂载 AuthMiddleware。
func SetupRouter() *gin.Engine {
	if config.AppConfig.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()

	// 全局中间件
	r.Use(middleware.CorsMiddleware())

	// 上传静态文件访问
	r.Static("/uploads", config.AppConfig.Upload.Path)

	// 初始化Service
	userService := services.NewUserService()
	followService := services.NewFollowService()
	feedService := services.NewFeedService()
	messageService := services.NewMessageService()
	notificationService := services.NewNotificationService()
	discoverService := services.NewDiscoverService()

	// 初始化Handler
	userHandler := handlers.NewUserHandler(userService)
	followHandler := handlers.NewFollowHandler(followService)
	feedHandler := handlers.NewFeedHandler(feedService)
	uploadHandler := handlers.NewUploadHandler()
	notificationHandler := handlers.NewNotificationHandler(notificationService)
	wsHandler := handlers.NewWSHandler(messageService)
	messageHandler := handlers.NewMessageHandler(messageService)
	opsHandler := handlers.NewOpsHandler()
	discoverHandler := handlers.NewDiscoverHandler(discoverService)

	// API路由组
	api := r.Group("/api")
	{
		rl := config.AppConfig.RateLimit

		// ==================== 认证相关（无需登录） ====================
		auth := api.Group("/auth")
		{
			auth.POST("/register", middleware.RateLimitByIP("register", rl.RegisterIP.Rate, rl.RegisterIP.Burst), userHandler.Register)         //令牌桶限流
			auth.POST("/register/confirm", userHandler.RegisterConfirm)                                                                         //注册第二步：验证码确认建号
			auth.POST("/login", middleware.RateLimitByIP("login", rl.LoginIP.Rate, rl.LoginIP.Burst), userHandler.Login)                        //令牌桶限流
			auth.POST("/email/code", middleware.RateLimitByIP("send_code", rl.SendCodeIP.Rate, rl.SendCodeIP.Burst), userHandler.SendEmailCode) //验证码发送（注册/找回密码）令牌桶限流
			auth.POST("/password/reset", userHandler.ResetPassword)                                                                             //忘记密码重置
		}

		// ==================== 需要登录的路由 ====================
		authenticated := api.Group("")
		authenticated.Use(middleware.AuthMiddleware())
		{
			// 用户相关
			authenticated.GET("/users/me", userHandler.GetCurrentUser)
			authenticated.PUT("/users/me", userHandler.UpdateProfile)
			authenticated.GET("/users/me/visits", userHandler.GetRecentVisits)
			authenticated.POST("/users/me/password/code", userHandler.SendChangePasswordCode)  //修改密码验证码
			authenticated.PUT("/users/me/password", userHandler.ChangePassword)                //修改密码
			authenticated.POST("/users/me/email/change/code", userHandler.SendChangeEmailCode) //绑定/更换邮箱验证码
			authenticated.PUT("/users/me/email", userHandler.ChangeEmail)                      //绑定/更换邮箱
			authenticated.GET("/users/search", userHandler.SearchUsers)
			authenticated.GET("/users/:id", userHandler.GetProfile)

			// 上传相关
			authenticated.POST("/upload/image", uploadHandler.UploadImage)
			authenticated.POST("/upload/video", uploadHandler.UploadVideo)

			// 关注相关
			authenticated.POST("/follow/:id", followHandler.Follow)
			authenticated.DELETE("/follow/:id", followHandler.Unfollow)
			authenticated.GET("/users/:id/followers", followHandler.GetFollowers)
			authenticated.GET("/users/:id/following", followHandler.GetFollowing)

			// Feed相关
			authenticated.POST("/feeds", middleware.RateLimitByUser("publish_feed", rl.PublishFeed.Rate, rl.PublishFeed.Burst), feedHandler.PublishFeed)    //令牌桶限流
			authenticated.POST("/feeds/repost", middleware.RateLimitByUser("repost_feed", rl.RepostFeed.Rate, rl.RepostFeed.Burst), feedHandler.RepostFeed) //令牌桶限流
			authenticated.DELETE("/feeds/:id", feedHandler.DeleteFeed)
			authenticated.GET("/feeds/search", feedHandler.SearchFeeds)
			authenticated.GET("/feeds/:id", feedHandler.GetFeed)
			authenticated.GET("/users/:id/feeds", feedHandler.GetUserFeeds)

			// 时间线
			authenticated.GET("/timeline", feedHandler.GetTimeline)

			// 点赞
			authenticated.POST("/feeds/:id/like", middleware.RateLimitByFeedFromContext("like", rl.LikeFeed.Rate, rl.LikeFeed.Burst), feedHandler.LikeFeed)       //令牌桶限流
			authenticated.DELETE("/feeds/:id/like", middleware.RateLimitByFeedFromContext("unlike", rl.LikeFeed.Rate, rl.LikeFeed.Burst), feedHandler.UnlikeFeed) //令牌桶限流
			authenticated.GET("/feeds/:id/likes", feedHandler.GetFeedLikers)

			// 评论
			authenticated.POST("/feeds/:id/comments", middleware.RateLimitByFeedFromContext("comment", rl.CommentFeed.Rate, rl.CommentFeed.Burst), feedHandler.CommentFeed)                        //令牌桶限流
			authenticated.DELETE("/feeds/:id/comments/:comment_id", middleware.RateLimitByFeedFromContext("delete_comment", rl.CommentFeed.Rate, rl.CommentFeed.Burst), feedHandler.DeleteComment) //令牌桶限流
			authenticated.GET("/feeds/:id/comments", feedHandler.GetComments)
			authenticated.GET("/feeds/:id/comments/:comment_id/replies", feedHandler.GetCommentReplies) //楼内回复列表

			// 私信：WebSocket 负责实时发送/接收，HTTP 提供会话与历史兜底读取。
			authenticated.GET("/ws/messages", wsHandler.MessageWS)
			authenticated.GET("/messages/conversations", messageHandler.ListConversations)
			authenticated.GET("/messages/history/:target_id", messageHandler.GetHistory)

			// 通知中心
			authenticated.GET("/notifications", notificationHandler.ListNotifications)
			authenticated.POST("/notifications/read-all", notificationHandler.MarkAllRead)
			authenticated.DELETE("/notifications/:id", notificationHandler.DeleteNotification)          //删除单条通知
			authenticated.POST("/notifications/clear-read", notificationHandler.ClearReadNotifications) //一键清空已读

			// 发现页
			authenticated.GET("/discover/popular-users", discoverHandler.PopularUsers) //热门用户
			authenticated.GET("/discover/popular-feeds", discoverHandler.PopularFeeds) //热门动态

			// 运维观测
			authenticated.GET("/ops/mq/metrics", opsHandler.MQMetrics)
			authenticated.GET("/ops/cache/metrics", opsHandler.CacheMetrics)
		}
	}

	return r
}
