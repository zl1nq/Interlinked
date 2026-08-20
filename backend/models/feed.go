package models

import (
	"time"

	"gorm.io/gorm"
)

// FeedType 常量定义。
// 约定：仅通过常量判断动态类型，避免魔法值。
const (
	FeedTypeOriginal = 0 // 原创
	FeedTypeRepost   = 1 // 转发
)

// Feed 动态/帖子模型（发件箱 - 存储用户发布的内容）
type Feed struct {
	ID           uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID       uint           `gorm:"index:idx_user_created;index:idx_feed_created_user,priority:2;not null" json:"user_id"` // 发布者ID
	Content      string         `gorm:"type:text" json:"content"`                                                              // 文案内容（可为空，纯图片/视频）
	Images       string         `gorm:"type:varchar(2000);default:''" json:"images"`                                           // 图片URL列表，JSON数组
	Videos       string         `gorm:"type:varchar(2000);default:''" json:"videos"`                                           // 视频URL列表，JSON数组
	FeedType     int            `gorm:"default:0" json:"feed_type"`                                                            // 0-原创 1-转发
	OriginalID   *uint          `gorm:"index" json:"original_id"`                                                              // 转发的原始Feed ID
	LikeCount    int64          `gorm:"default:0" json:"like_count"`                                                           // 点赞数
	CommentCount int64          `gorm:"default:0" json:"comment_count"`                                                        // 评论数
	ShareCount   int64          `gorm:"default:0" json:"share_count"`                                                          // 转发数
	CreatedAt    time.Time      `gorm:"index:idx_user_created;index:idx_feed_created_user,priority:1;index:idx_feed_created" json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Feed) TableName() string {
	return "feeds"
}

// FeedResponse 为对外返回的动态视图模型。
// 说明：
// - 与数据库实体 Feed 分离，便于扩展展示字段；
// - OriginalFeed 用于转发场景展示引用内容。
type FeedResponse struct {
	ID           uint          `json:"id"`
	UserID       uint          `json:"user_id"`
	Content      string        `json:"content"`
	Images       string        `json:"images"`
	Videos       string        `json:"videos"`
	FeedType     int           `json:"feed_type"`
	OriginalID   *uint         `json:"original_id"`
	OriginalFeed *FeedResponse `json:"original_feed,omitempty"` // 转发的原始Feed
	LikeCount    int64         `json:"like_count"`
	CommentCount int64         `json:"comment_count"`
	ShareCount   int64         `json:"share_count"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
	Author       UserResponse  `json:"author"`
	IsLiked      bool          `json:"is_liked"`
}

// Timeline 收件箱模型（推模式下，将feed推送到粉丝的收件箱）
type Timeline struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint      `gorm:"index:idx_timeline_user;not null" json:"user_id"` // 收件箱所有者
	FeedID    uint      `gorm:"not null" json:"feed_id"`                         // Feed ID
	AuthorID  uint      `gorm:"not null" json:"author_id"`                       // 原作者ID
	CreatedAt time.Time `gorm:"index:idx_timeline_user" json:"created_at"`       // 按时间排序
}

func (Timeline) TableName() string {
	return "timelines"
}

// Like 点赞模型
type Like struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint      `gorm:"index:idx_like_user_feed,unique;index:idx_like_feed_user,priority:2;not null" json:"user_id"`
	FeedID    uint      `gorm:"index:idx_like_user_feed,unique;index:idx_like_feed_user,priority:1;not null" json:"feed_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (Like) TableName() string {
	return "likes"
}

// LikeResponse 点赞列表项响应
type LikeResponse struct {
	ID       uint   `json:"id"`
	UserID   uint   `json:"user_id"`
	FeedID   uint   `json:"feed_id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
}

// Comment 评论模型
type Comment struct {
	ID        uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint           `gorm:"index;not null" json:"user_id"`
	FeedID    uint           `gorm:"index:idx_comment_feed_created,priority:1;not null" json:"feed_id"`
	Content   string         `gorm:"type:varchar(500);not null" json:"content"`
	CreatedAt time.Time      `gorm:"index:idx_comment_feed_created,priority:2" json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Comment) TableName() string {
	return "comments"
}

// CommentResponse 评论列表项响应
type CommentResponse struct {
	ID       uint   `json:"id"`
	UserID   uint   `json:"user_id"`
	FeedID   uint   `json:"feed_id"`
	Content  string `json:"content"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
}
