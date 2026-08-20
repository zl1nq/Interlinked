package handlers

import (
	"feed/config"
	"feed/utils"
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// UploadHandler 负责媒体文件上传（图片/视频）。
// 说明：
// - 仅处理基础校验与落盘；
// - 业务侧只保存返回的 URL，不直接感知存储细节。
type UploadHandler struct{}

// NewUploadHandler 构建 UploadHandler。
func NewUploadHandler() *UploadHandler {
	return &UploadHandler{}
}

// UploadImage 上传图片
// POST /api/upload/image
func (h *UploadHandler) UploadImage(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		utils.Error(c, 400, "请选择要上传的文件")
		return
	}

	// 检查文件大小
	maxSize := int64(config.AppConfig.Upload.ImageMaxSize) * 1024 * 1024
	if file.Size > maxSize {
		utils.Error(c, 400, fmt.Sprintf("图片大小不能超过%dMB", config.AppConfig.Upload.ImageMaxSize))
		return
	}

	// 检查文件类型
	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowed := false
	for _, e := range config.AppConfig.Upload.ImageExts {
		if ext == e {
			allowed = true
			break
		}
	}
	if !allowed {
		utils.Error(c, 400, "不支持的图片格式，支持: "+strings.Join(config.AppConfig.Upload.ImageExts, ", "))
		return
	}
	if !hasAllowedContentType(file, []string{"image/jpeg", "image/png", "image/gif", "image/webp"}) {
		utils.Error(c, 400, "图片文件内容与格式不匹配")
		return
	}

	// 生成存储路径
	datePath := time.Now().Format("2006/01/02")
	uploadDir := filepath.Join(config.AppConfig.Upload.Path, "images", datePath)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		utils.Error(c, 500, "创建目录失败")
		return
	}

	// 生成唯一文件名
	filename := uuid.New().String() + ext
	savePath := filepath.Join(uploadDir, filename)

	if err := c.SaveUploadedFile(file, savePath); err != nil {
		utils.Error(c, 500, "保存文件失败")
		return
	}

	// 返回访问URL
	url := "/uploads/images/" + datePath + "/" + filename

	utils.Success(c, gin.H{
		"url":      url,
		"filename": file.Filename,
		"size":     file.Size,
		"type":     "image",
	})
}

func hasAllowedContentType(fileHeader interface {
	Open() (multipart.File, error)
}, allowedTypes []string) bool {
	file, err := fileHeader.Open()
	if err != nil {
		return false
	}
	defer file.Close()

	buf := make([]byte, 512)
	n, _ := file.Read(buf)
	contentType := http.DetectContentType(buf[:n])
	for _, allowed := range allowedTypes {
		if contentType == allowed {
			return true
		}
	}
	return false
}

// UploadVideo 上传视频
// POST /api/upload/video
func (h *UploadHandler) UploadVideo(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		utils.Error(c, 400, "请选择要上传的文件")
		return
	}

	// 检查文件大小
	maxSize := int64(config.AppConfig.Upload.VideoMaxSize) * 1024 * 1024
	if file.Size > maxSize {
		utils.Error(c, 400, fmt.Sprintf("视频大小不能超过%dMB", config.AppConfig.Upload.VideoMaxSize))
		return
	}

	// 检查文件类型
	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowed := false
	for _, e := range config.AppConfig.Upload.VideoExts {
		if ext == e {
			allowed = true
			break
		}
	}
	if !allowed {
		utils.Error(c, 400, "不支持的视频格式，支持: "+strings.Join(config.AppConfig.Upload.VideoExts, ", "))
		return
	}
	if !hasAllowedContentType(file, []string{"video/mp4", "video/quicktime", "video/x-msvideo", "video/webm"}) {
		utils.Error(c, 400, "视频文件内容与格式不匹配")
		return
	}

	// 生成存储路径
	datePath := time.Now().Format("2006/01/02")
	uploadDir := filepath.Join(config.AppConfig.Upload.Path, "videos", datePath)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		utils.Error(c, 500, "创建目录失败")
		return
	}

	// 生成唯一文件名
	filename := uuid.New().String() + ext
	savePath := filepath.Join(uploadDir, filename)

	if err := c.SaveUploadedFile(file, savePath); err != nil {
		utils.Error(c, 500, "保存文件失败")
		return
	}

	// 返回访问URL
	url := "/uploads/videos/" + datePath + "/" + filename

	utils.Success(c, gin.H{
		"url":      url,
		"filename": file.Filename,
		"size":     file.Size,
		"type":     "video",
	})
}
