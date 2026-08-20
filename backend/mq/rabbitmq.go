package mq

import (
	"encoding/json"
	"feed/cache"
	"feed/config"
	"feed/models"
	"fmt"
	"log"
	"maps"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	headerRetryCount   = "x-retry-count"
	headerMessageKey   = "x-message-key"
	headersContentType = "application/json"
)

// FeedMessage 推送消息
type FeedMessage struct {
	FeedID    uint    `json:"feed_id"`
	AuthorID  uint    `json:"author_id"`
	Timestamp float64 `json:"timestamp"`
}

var (
	publisherConn  *amqp.Connection
	publisherChan  *amqp.Channel
	publisherQueue amqp.Queue
	publisherMu    sync.Mutex

	mqCircuitOpenUntil int64 //熔断开启时间 unix秒数，大于当前时间表示熔断开启

	mqCircuitOpenCount   uint64 //熔断开启次数
	mqCircuitCloseCount  uint64 //熔断关闭次数
	mqDegradeCount       uint64 //降级次数
	mqPublishFailCount   uint64 //发布失败次数
	mqRetryCount         uint64 //重试次数
	mqDLQCount           uint64 //死信次数
	mqDispatchErrorCount uint64 //分发错误次数
)

// InitMQ 初始化 RabbitMQ，并启动消费者。
func InitMQ() error {
	if err := initPublisher(); err != nil {
		return err
	}

	consumerCount := config.AppConfig.RabbitMQ.Consumers
	if consumerCount <= 0 {
		consumerCount = 10
	}

	for i := 0; i < consumerCount; i++ {
		go consumerLoop(i)
	}

	log.Printf("RabbitMQ initialized with %d consumers", consumerCount)
	return nil
}

// initPublisher 初始化发布端连接与通道，并声明完整队列拓扑。
// 失败时由调用方决定重试策略。
func initPublisher() error {
	cfg := config.AppConfig.RabbitMQ

	conn, err := amqp.Dial(cfg.URL())
	if err != nil {
		return err
	}

	ch, err := conn.Channel() //创建通道
	if err != nil {
		_ = conn.Close()
		return err
	}

	q, err := declareTopology(ch) //声明队列拓扑
	if err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return err
	}

	publisherConn = conn
	publisherChan = ch
	publisherQueue = q
	return nil
}

// declareTopology 声明 MQ 拓扑：主队列、重试队列、死信队列。
// 说明：
// - 主队列消费失败后可进入重试或死信流程；
// - 重试队列通过 TTL 到期后回流主队列；
// - 死信队列承接超过重试上限的消息，便于排查与回放。
func declareTopology(ch *amqp.Channel) (amqp.Queue, error) {
	q, err := declareMainQueue(ch)
	if err != nil {
		return amqp.Queue{}, err
	}
	if _, err := declareRetryQueue(ch); err != nil {
		return amqp.Queue{}, err
	}
	if _, err := declareDeadLetterQueue(ch); err != nil {
		return amqp.Queue{}, err
	}
	return q, nil
}

// declareMainQueue 声明主消费队列。
// 配置 dead-letter-routing-key 指向 DLQ，便于异常兜底。
func declareMainQueue(ch *amqp.Channel) (amqp.Queue, error) {
	cfg := config.AppConfig.RabbitMQ
	args := amqp.Table{
		"x-dead-letter-exchange":    "",
		"x-dead-letter-routing-key": cfg.DeadLetterQueueName(),
	}
	return ch.QueueDeclare(cfg.QueueName(), true, false, false, false, args)
}

// declareRetryQueue 声明重试队列。
// 消息在该队列等待 retry_delay_ms 后自动回流主队列，实现延迟重试。
func declareRetryQueue(ch *amqp.Channel) (amqp.Queue, error) {
	cfg := config.AppConfig.RabbitMQ
	delay := cfg.RetryDelayMS //重试延迟时间
	if delay <= 0 {
		delay = 5000 //如果重试延迟时间小于0，则设置为5000毫秒
	}
	args := amqp.Table{
		"x-message-ttl":             int32(delay),
		"x-dead-letter-exchange":    "",
		"x-dead-letter-routing-key": cfg.QueueName(),
	}
	return ch.QueueDeclare(cfg.RetryQueueName(), true, false, false, false, args) //声明重试队列 自动回流主队列
}

// declareDeadLetterQueue 声明死信队列。
func declareDeadLetterQueue(ch *amqp.Channel) (amqp.Queue, error) {
	cfg := config.AppConfig.RabbitMQ
	return ch.QueueDeclare(cfg.DeadLetterQueueName(), true, false, false, false, nil) //声明死信队列
}

// PublishFeed 发布 Feed 推送任务。
// PublishFeed 发布任务到 MQ；当熔断开启时降级为仅写 outbox，保障发布主链路可用。
func PublishFeed(feedID, authorID uint) error {
	msg := FeedMessage{
		FeedID:    feedID,
		AuthorID:  authorID,
		Timestamp: float64(time.Now().UnixMilli()),
	}

	if isCircuitOpen() {
		degradeToOutbox(msg) //降级为发件箱
		return nil
	}

	body, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[MQ] Marshal message failed: %v", err)
		return err
	}

	publisherMu.Lock()
	defer publisherMu.Unlock()

	if publisherChan == nil {
		if err := initPublisher(); err != nil {
			log.Printf("[MQ] Re-init publisher failed: %v", err)
			degradeToOutbox(msg)
			return err
		}
	}
	//消息幂等键
	msgKey := messageKey(feedID, authorID)
	//发布消息到队列
	err = publishToQueue(publisherQueue.Name, body, amqp.Table{
		headerRetryCount: int32(0),
		headerMessageKey: msgKey,
	})
	if err != nil { //发布消息失败，则重置发布者 重试发布消息
		//重置发布者
		_ = resetPublisher()
		//如果发布者不为空，则重新发布消息
		if publisherChan != nil {
			err = publishToQueue(publisherQueue.Name, body, amqp.Table{
				headerRetryCount: int32(0),
				headerMessageKey: msgKey,
			})
		}
	}
	//发布失败 打开熔断器 降级为发件箱
	if err != nil {
		atomic.AddUint64(&mqPublishFailCount, 1)
		log.Printf("[MQ] Publish feed %d failed: %v", feedID, err)
		openCircuit(15 * time.Second) //打开熔断器
		degradeToOutbox(msg)          //降级为发件箱
		return err
	}

	closeCircuit() //关闭熔断器

	log.Printf("[MQ] Feed %d published to queue", feedID)
	return nil
}

// publishToQueue 统一发布函数。
// 所有重试次数、幂等键等元信息通过 headers 透传。
func publishToQueue(queueName string, body []byte, headers amqp.Table) error {
	return publisherChan.Publish(
		"",
		queueName,
		false,
		false,
		amqp.Publishing{
			Headers:      headers,
			ContentType:  headersContentType,
			Body:         body,
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
		},
	)
}

// 重置发布者
func resetPublisher() error {
	if publisherChan != nil {
		_ = publisherChan.Close()
		publisherChan = nil
	}
	if publisherConn != nil {
		_ = publisherConn.Close()
		publisherConn = nil
	}
	return initPublisher()
}

// 消费循环
func consumerLoop(workerID int) {
	for {
		if err := runConsumer(workerID); err != nil {
			log.Printf("[MQ Consumer %d] disconnected: %v", workerID, err)
			time.Sleep(3 * time.Second) //睡眠3秒
		}
	}
}

// runConsumer 启动单个消费者：
// - 声明队列拓扑
// - 设置 Qos
// - 订阅主队列并逐条处理
func runConsumer(workerID int) error {
	cfg := config.AppConfig.RabbitMQ

	conn, err := amqp.Dial(cfg.URL())
	if err != nil {
		return err
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()
	//声明队列拓扑 主队列 重试队列 死信队列
	if _, err := declareTopology(ch); err != nil {
		return err
	}

	//设置Qos 预取消息数量
	prefetch := cfg.Prefetch
	if prefetch <= 0 {
		prefetch = 50
	}
	if err := ch.Qos(prefetch, 0, false); err != nil {
		return err
	}

	//消费消息
	msgs, err := ch.Consume(
		cfg.QueueName(),
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	//消费消息
	for d := range msgs {
		var msg FeedMessage
		if err := json.Unmarshal(d.Body, &msg); err != nil {
			log.Printf("[MQ Consumer %d] Invalid message: %v", workerID, err)
			_ = d.Ack(false)
			continue
		}

		processed, err := processDispatch(workerID, msg, d.Headers) //处理消息
		if err != nil {
			handleRetryOrDeadLetter(workerID, d, err) //处理失败消息
			continue
		}
		if processed { //确认消息
			_ = d.Ack(false)
		}
	}

	return amqp.ErrClosed
}

// processDispatch 执行推拉混合分发核心逻辑，并做幂等保护。
func processDispatch(workerID int, msg FeedMessage, headers amqp.Table) (bool, error) {
	defer func() { //延迟函数，用于恢复panic
		if r := recover(); r != nil {
			log.Printf("[MQ Worker %d] Panic recovered: %v", workerID, r)
		}
	}()
	//提取消息幂等键
	messageKey := extractMessageKey(msg, headers)
	if messageKey == "" {
		messageKey = messageKeyFromMessage(msg)
	}

	ok, err := tryMarkMessageProcessing(messageKey) //尝试设置幂等标记
	if err != nil {
		return false, fmt.Errorf("idempotent check failed: %w", err)
	}
	if !ok { //如果幂等标记失败，则忽略消息
		log.Printf("[MQ Worker %d] Duplicate message ignored: %s", workerID, messageKey)
		return true, nil
	}
	defer func() {
		if err != nil {
			_ = releaseMessageProcessing(messageKey) //释放幂等标记 防止幂等标记永久占用
		}
	}()
	//判断是否为大V
	isBigV, err := cache.IsBigV(msg.AuthorID)
	if err != nil {
		var user models.User
		if dbErr := models.DB.Select("is_big_v").Where("id = ?", msg.AuthorID).First(&user).Error; dbErr == nil {
			isBigV = user.IsBigV
		}
	}
	//将消息添加到发件箱
	if err := cache.AddToOutbox(msg.AuthorID, msg.FeedID, msg.Timestamp); err != nil {
		return false, fmt.Errorf("add to outbox failed: %w", err)
	}

	if isBigV {
		log.Printf("[MQ Worker %d] User %d is BigV, feed %d stored in outbox only", workerID, msg.AuthorID, msg.FeedID)
		return true, nil
	}
	//不是大V 将消息添加到收件箱
	if err := dispatchToFollowers(workerID, msg); err != nil {
		atomic.AddUint64(&mqDispatchErrorCount, 1)
		return false, err
	}
	return true, nil
}

// 将消息添加到收件箱
func dispatchToFollowers(workerID int, msg FeedMessage) error {
	threshold := config.AppConfig.Feed.PushFanLimit //普通用户推送上限

	var author models.User
	if err := models.DB.Select("follower_count").Where("id = ?", msg.AuthorID).First(&author).Error; err != nil {
		return fmt.Errorf("query author follower count failed: %w", err)
	}
	if author.FollowerCount > int64(threshold) {
		log.Printf("[MQ Worker %d] User %d has %d followers, exceeds threshold, keep outbox-only read path", workerID, msg.AuthorID, author.FollowerCount)
		return nil
	}

	var followers []models.Follow //粉丝列表
	if err := models.DB.Where("followed_id = ?", msg.AuthorID).Select("user_id").Find(&followers).Error; err != nil {
		return fmt.Errorf("query followers failed: %w", err)
	}

	followerIDs := make([]uint, 0, len(followers))
	timelines := make([]models.Timeline, 0, len(followers))
	now := time.Now()
	for _, follower := range followers {
		followerIDs = append(followerIDs, follower.UserID)
		timelines = append(timelines, models.Timeline{
			UserID:    follower.UserID,
			FeedID:    msg.FeedID,
			AuthorID:  msg.AuthorID,
			CreatedAt: now,
		})
	}

	if err := cache.AddToInboxes(followerIDs, msg.FeedID, msg.Timestamp); err != nil {
		return fmt.Errorf("batch push to inbox failed: %w", err)
	}
	if len(timelines) > 0 {
		if err := models.DB.CreateInBatches(&timelines, 500).Error; err != nil {
			log.Printf("[MQ Worker %d] Create timeline records failed (degraded): %v", workerID, err)
		}
	}
	log.Printf("[MQ Worker %d] Feed %d pushed to %d followers' inbox", workerID, msg.FeedID, len(followers))
	return nil
}

// handleRetryOrDeadLetter 处理消费失败的消息：
// - 未超过最大重试：投递到重试队列（延迟后回流主队列）
// - 超过最大重试：投递到死信队列（DLQ）
func handleRetryOrDeadLetter(workerID int, d amqp.Delivery, processErr error) {
	//获取配置
	cfg := config.AppConfig.RabbitMQ
	maxRetries := cfg.MaxRetries //最大重试次数
	if maxRetries <= 0 {
		maxRetries = 3
	}

	retryCount := extractRetryCount(d.Headers) //提取重试次数
	retryCount++                               //重试次数加1

	if publisherChan == nil { //如果发布者为空，则重新初始化发布者
		if err := initPublisher(); err != nil { //重新初始化发布者失败，则忽略消息
			log.Printf("[MQ Worker %d] re-init publisher failed when retry: %v", workerID, err)
			_ = d.Nack(false, true)
			return
		}
	}

	headers := copyHeaders(d.Headers)             //复制消息头
	headers[headerRetryCount] = int32(retryCount) //重试次数写入消息头

	var routeQueue string
	if retryCount > maxRetries { //如果重试次数超过最大重试次数，则投递到死信队列
		routeQueue = cfg.DeadLetterQueueName()
		atomic.AddUint64(&mqDLQCount, 1)
		log.Printf("[MQ Worker %d] move message to DLQ after %d retries, err=%v", workerID, retryCount-1, processErr)
	} else {
		routeQueue = cfg.RetryQueueName() //如果重试次数不超过最大重试次数，则投递到重试队列
		atomic.AddUint64(&mqRetryCount, 1)
		log.Printf("[MQ Worker %d] retry message #%d, err=%v", workerID, retryCount, processErr)
	}

	if err := publishToQueue(routeQueue, d.Body, headers); err != nil { //发布消息到重试队列或死信队列
		log.Printf("[MQ Worker %d] publish retry/dlq failed: %v", workerID, err)
		_ = d.Nack(false, true)
		return
	}

	_ = d.Ack(false) //确认消息
}

// tryMarkMessageProcessing 尝试设置幂等标记。
// 返回 true 表示“首次处理”，false 表示“重复消息”。
func tryMarkMessageProcessing(messageKey string) (bool, error) {
	ttlMin := config.AppConfig.RabbitMQ.IdempotentTTLMin //幂等时间
	if ttlMin <= 0 {
		ttlMin = 60
	}
	ttl := time.Duration(ttlMin) * time.Minute //幂等时间
	idKey := "mq:idempotent:" + messageKey
	ok, err := cache.RedisClient.SetNX(cache.Ctx, idKey, "1", ttl).Result() //尝试设置幂等标记
	return ok, err
}

// 释放幂等标记
func releaseMessageProcessing(messageKey string) error {
	idKey := "mq:idempotent:" + messageKey
	return cache.RedisClient.Del(cache.Ctx, idKey).Err()
}

// 提取重试次数
func extractRetryCount(headers amqp.Table) int {
	if headers == nil {
		return 0
	}
	raw, ok := headers[headerRetryCount]
	if !ok {
		return 0
	}
	return parseHeaderInt(raw)
}

// 提取消息幂等键
func extractMessageKey(msg FeedMessage, headers amqp.Table) string {
	if headers == nil {
		return messageKeyFromMessage(msg)
	}
	if raw, ok := headers[headerMessageKey]; ok {
		if s, ok := raw.(string); ok && s != "" {
			return s
		}
	}
	return messageKeyFromMessage(msg)
}

// 提取消息幂等键
func messageKeyFromMessage(msg FeedMessage) string {
	return messageKey(msg.FeedID, msg.AuthorID)
}

// 生成消息幂等键
func messageKey(feedID, authorID uint) string {
	return fmt.Sprintf("feed:%d:author:%d", feedID, authorID)
}

// 复制消息头
func copyHeaders(src amqp.Table) amqp.Table {
	dst := amqp.Table{}
	maps.Copy(dst, src)
	return dst
}

// 熔断器
func isCircuitOpen() bool {
	until := atomic.LoadInt64(&mqCircuitOpenUntil) //熔断开启时间
	if until == 0 {
		return false
	}
	return time.Now().Unix() < until //当前时间小于熔断开启时间，表示熔断开启
}

// 打开熔断器
func openCircuit(duration time.Duration) {
	until := time.Now().Add(duration).Unix()     //熔断开启时间
	old := atomic.LoadInt64(&mqCircuitOpenUntil) //旧的熔断开启时间
	atomic.StoreInt64(&mqCircuitOpenUntil, until)
	if old == 0 || time.Now().Unix() >= old {
		atomic.AddUint64(&mqCircuitOpenCount, 1) //熔断开启次数加1
		log.Printf("[MQ][Circuit] opened for %s", duration)
	}
}

// 关闭熔断器
func closeCircuit() {
	old := atomic.LoadInt64(&mqCircuitOpenUntil)
	if old != 0 {
		atomic.AddUint64(&mqCircuitCloseCount, 1)
		log.Printf("[MQ][Circuit] closed")
	}
	atomic.StoreInt64(&mqCircuitOpenUntil, 0)
}

// degradeToOutbox 降级路径：MQ 不可用时仅写作者 outbox，避免发布链路被阻塞。
func degradeToOutbox(msg FeedMessage) {
	atomic.AddUint64(&mqDegradeCount, 1)
	if err := cache.AddToOutbox(msg.AuthorID, msg.FeedID, msg.Timestamp); err != nil {
		log.Printf("[MQ][Degrade] add to outbox failed, author=%d feed=%d err=%v", msg.AuthorID, msg.FeedID, err)
		return
	}
	log.Printf("[MQ][Degrade] circuit open, fallback to outbox only, author=%d feed=%d", msg.AuthorID, msg.FeedID)
}

// SnapshotMetrics 返回 MQ 熔断/降级关键指标快照，便于接入监控上报。
func SnapshotMetrics() map[string]uint64 {
	openUntil := atomic.LoadInt64(&mqCircuitOpenUntil)
	isOpen := uint64(0)
	if openUntil > time.Now().Unix() {
		isOpen = 1
	}
	return map[string]uint64{
		"circuit_open":         isOpen,                                  //熔断开启状态
		"circuit_open_count":   atomic.LoadUint64(&mqCircuitOpenCount),  //熔断开启次数
		"circuit_close_count":  atomic.LoadUint64(&mqCircuitCloseCount), //熔断关闭次数
		"degrade_count":        atomic.LoadUint64(&mqDegradeCount),      //降级次数
		"publish_fail_count":   atomic.LoadUint64(&mqPublishFailCount),
		"retry_count":          atomic.LoadUint64(&mqRetryCount),         //重试次数
		"dlq_count":            atomic.LoadUint64(&mqDLQCount),           //死信次数
		"dispatch_error_count": atomic.LoadUint64(&mqDispatchErrorCount), //分发错误次数
	}
}

// 解析消息头中的整数值
func parseHeaderInt(v any) int {
	switch val := v.(type) {
	case int:
		return val
	case int8:
		return int(val)
	case int16:
		return int(val)
	case int32:
		return int(val)
	case int64:
		return int(val)
	case uint:
		return int(val)
	case uint8:
		return int(val)
	case uint16:
		return int(val)
	case uint32:
		return int(val)
	case uint64:
		return int(val)
	case string:
		n, _ := strconv.Atoi(val)
		return n
	default:
		return 0
	}
}
