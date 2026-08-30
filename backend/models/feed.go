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
	LikeCount    int64          `gorm:"default:0;index:idx_feed_like_count" json:"like_count"`                                 // 点赞数（发现页热门动态排行索引）
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
	ID              uint          `json:"id"`
	UserID          uint          `json:"user_id"`
	Content         string        `json:"content"`
	Images          string        `json:"images"`
	Videos          string        `json:"videos"`
	FeedType        int           `json:"feed_type"`
	OriginalID      *uint         `json:"original_id"`
	OriginalFeed    *FeedResponse `json:"original_feed,omitempty"` // 转发的原始Feed
	OriginalDeleted bool          `json:"original_deleted"`        // 原帖已被删除（OriginalFeed 为空且此标记为 true）
	LikeCount       int64         `json:"like_count"`
	CommentCount    int64         `json:"comment_count"`
	ShareCount      int64         `json:"share_count"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
	Author          UserResponse  `json:"author"`
	IsLiked         bool          `json:"is_liked"`
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

// Comment 评论模型（两段式楼中楼）：
//   - 根评论：root_id=0；
//   - 楼内回复：root_id=所属根评论 ID，reply_to 三元组指向被回复的那条评论
//     （直接回复根评论时 reply_to_comment_id = root_id）。
type Comment struct {
	ID        uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint           `gorm:"index;not null" json:"user_id"`
	FeedID    uint           `gorm:"index:idx_comment_feed_created,priority:1;not null" json:"feed_id"`
	Content   string         `gorm:"type:varchar(500);not null" json:"content"`
	RootID    uint           `gorm:"default:0;index:idx_comment_root_created,priority:1" json:"root_id"` // 0=根评论
	CreatedAt time.Time      `gorm:"index:idx_comment_feed_created,priority:2;index:idx_comment_root_created,priority:2" json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	ReplyToCommentID uint   `gorm:"default:0" json:"reply_to_comment_id"`                  // 被回复的评论 ID；直接回复根评论时等于 root_id
	ReplyToUserID    uint   `gorm:"default:0" json:"reply_to_user_id"`                     // 被回复的用户 ID
	ReplyToNickname  string `gorm:"type:varchar(100);default:''" json:"reply_to_nickname"` // 被回复者昵称快照
}

func (Comment) TableName() string {
	return "comments"
}

// CommentResponse 评论/楼内回复的展示模型。
type CommentResponse struct {
	ID               uint      `json:"id"`
	UserID           uint      `json:"user_id"`
	FeedID           uint      `json:"feed_id"`
	Content          string    `json:"content"`
	Username         string    `json:"username"`
	Nickname         string    `json:"nickname"`
	CreatedAt        time.Time `json:"created_at"`
	ReplyToCommentID uint      `json:"reply_to_comment_id"`
	ReplyToUserID    uint      `json:"reply_to_user_id"`
	ReplyToNickname  string    `json:"reply_to_nickname"` // 为空表示直接回复根评论
}

// CommentThreadResponse 评论列表元素：根评论楼层（含首屏回复）。
type CommentThreadResponse struct {
	ID         uint              `json:"id"`
	UserID     uint              `json:"user_id"`
	FeedID     uint              `json:"feed_id"`
	Content    string            `json:"content"`
	Username   string            `json:"username"`
	Nickname   string            `json:"nickname"`
	CreatedAt  time.Time         `json:"created_at"`
	ReplyCount int64             `json:"reply_count"` // 楼内回复总数（不含根评论自身）
	Replies    []CommentResponse `json:"replies"`     // 首屏回复，时间正序前 3 条
}
