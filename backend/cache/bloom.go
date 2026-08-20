package cache

import (
	"feed/models"
	"fmt"
	"hash/fnv"
	"sync/atomic"

	"github.com/go-redis/redis/v8"
)

const (
	bloomUserKey = "bf:user:id"
	bloomFeedKey = "bf:feed:id"
	bloomM       = uint(1 << 24) // 16,777,216 bits (~2MB)
	bloomK       = 6
)

var bloomReady atomic.Bool

// 初始化布隆过滤器
func InitBloomFilters() error {
	if RedisClient == nil {
		return fmt.Errorf("redis not initialized")
	}
	//获取所有用户ID 添加到布隆过滤器
	var users []models.User
	if err := models.DB.Select("id").Find(&users).Error; err != nil {
		return err
	}
	for _, u := range users {
		_ = bloomAdd(bloomUserKey, u.ID)
	}
	//获取所有动态ID 添加到布隆过滤器
	var feeds []models.Feed
	if err := models.DB.Select("id").Find(&feeds).Error; err != nil {
		return err
	}
	for _, f := range feeds {
		_ = bloomAdd(bloomFeedKey, f.ID)
	}
	//布隆过滤器准备完成
	bloomReady.Store(true)
	return nil
}

// 添加用户ID到布隆过滤器
func AddUserID(id uint) {
	if id == 0 || RedisClient == nil {
		return
	}
	_ = bloomAdd(bloomUserKey, id)
}

// 添加动态ID到布隆过滤器
func AddFeedID(id uint) {
	if id == 0 || RedisClient == nil {
		return
	}
	_ = bloomAdd(bloomFeedKey, id)
}

// 判断用户ID是否可能存在
func MightUserExist(id uint) bool {
	if id == 0 {
		return false
	}
	if !bloomReady.Load() { //如果布隆过滤器未准备好 则认为存在
		return true
	}
	ok, err := bloomMightContain(bloomUserKey, id)
	if err != nil {
		return true
	}
	return ok
}

// 判断动态ID是否可能存在
func MightFeedExist(id uint) bool {
	if id == 0 {
		return false
	}
	if !bloomReady.Load() { //如果布隆过滤器未准备好 则认为存在
		return true
	}
	ok, err := bloomMightContain(bloomFeedKey, id)
	if err != nil {
		return true
	}
	return ok
}

// 添加ID到布隆过滤器
func bloomAdd(key string, id uint) error {
	offsets := bloomOffsets(id)
	pipe := RedisClient.Pipeline()
	for _, off := range offsets {
		pipe.SetBit(Ctx, key, int64(off), 1) //设置位图的值
	}
	_, err := pipe.Exec(Ctx)
	return err
}

// 判断ID是否可能存在
func bloomMightContain(key string, id uint) (bool, error) {
	offsets := bloomOffsets(id)    //计算ID的偏移量
	pipe := RedisClient.Pipeline() //创建管道
	cmds := make([]*redis.IntCmd, 0, len(offsets))
	for _, off := range offsets {
		cmds = append(cmds, pipe.GetBit(Ctx, key, int64(off))) //获取位图的值
	}
	if _, err := pipe.Exec(Ctx); err != nil { //执行管道
		return false, err
	}
	for _, cmd := range cmds {
		v, err := cmd.Result()
		if err != nil {
			return false, err
		}
		if v == 0 { //如果值为0 则不存在
			return false, nil
		}
	}
	return true, nil
}

// 计算ID的偏移量
func bloomOffsets(id uint) []uint {
	offsets := make([]uint, 0, bloomK)
	v := fmt.Sprintf("%d", id)
	// 每次用 i+1 + id 做一次哈希计算，得到一个偏移量
	for i := range bloomK {
		h := fnv.New64a()
		_, _ = h.Write([]byte{byte(i + 1)})
		_, _ = h.Write([]byte(v))
		offsets = append(offsets, uint(h.Sum64()%uint64(bloomM)))
	}
	return offsets
}
