package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore is a shared Store backed by Redis: pending actions + windowed
// history in keys, activity fan-out over pub/sub. Key layout:
//   erp:pending:{tenant}:{conv}  (JSON PendingAction, TTL 30m)
//   erp:hist:{tenant}:{conv}     (list of JSON HistoryMsg, trimmed to 20)
//   erp:activity:{tenant}        (pub/sub channel of JSON ActivityEvent)
type RedisStore struct {
	rdb      *redis.Client
	histSize int64
	pending  time.Duration
}

func NewRedisStore(addr string) (*RedisStore, error) {
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping %s failed: %w", addr, err)
	}
	return &RedisStore{rdb: rdb, histSize: 20, pending: 30 * time.Minute}, nil
}

func redisPendingKey(tenantID, conversationID string) string {
	return "erp:pending:" + tenantID + ":" + conversationID
}

func redisHistKey(tenantID, conversationID string) string {
	return "erp:hist:" + tenantID + ":" + conversationID
}

func redisActivityChannel(tenantID string) string {
	return "erp:activity:" + tenantID
}

func (s *RedisStore) SavePending(ctx context.Context, p PendingAction) error {
	if p.CreatedUnix == 0 {
		p.CreatedUnix = time.Now().Unix()
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, redisPendingKey(p.TenantID, p.ConversationID), raw, s.pending).Err()
}

func (s *RedisStore) TakePending(ctx context.Context, tenantID, conversationID string) (*PendingAction, error) {
	k := redisPendingKey(tenantID, conversationID)
	raw, err := s.rdb.GetDel(ctx, k).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var p PendingAction
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *RedisStore) DropPending(ctx context.Context, tenantID, conversationID string) error {
	return s.rdb.Del(ctx, redisPendingKey(tenantID, conversationID)).Err()
}

func (s *RedisStore) AppendHistory(ctx context.Context, tenantID, conversationID, role, text string) error {
	raw, err := json.Marshal(HistoryMsg{Role: role, Text: text, Timestamp: time.Now().Unix()})
	if err != nil {
		return err
	}
	k := redisHistKey(tenantID, conversationID)
	if err := s.rdb.LPush(ctx, k, raw).Err(); err != nil {
		return err
	}
	return s.rdb.LTrim(ctx, k, 0, s.histSize-1).Err()
}

func (s *RedisStore) History(ctx context.Context, tenantID, conversationID string, n int) ([]HistoryMsg, error) {
	count := int64(n)
	if count <= 0 || count > s.histSize {
		count = s.histSize
	}
	raws, err := s.rdb.LRange(ctx, redisHistKey(tenantID, conversationID), 0, count-1).Result()
	if err != nil {
		return nil, err
	}
	out := make([]HistoryMsg, 0, len(raws))
	for i := len(raws) - 1; i >= 0; i-- {
		var h HistoryMsg
		if err := json.Unmarshal([]byte(raws[i]), &h); err != nil {
			continue
		}
		out = append(out, h)
	}
	return out, nil
}

func (s *RedisStore) PublishActivity(ctx context.Context, event ActivityEvent) error {
	if event.Timestamp == 0 {
		event.Timestamp = time.Now().Unix()
	}
	raw, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return s.rdb.Publish(ctx, redisActivityChannel(event.TenantID), raw).Err()
}

func (s *RedisStore) SubscribeActivity(ctx context.Context, tenantID string) (<-chan ActivityEvent, func(), error) {
	sub := s.rdb.Subscribe(ctx, redisActivityChannel(tenantID))
	if _, err := sub.Receive(ctx); err != nil {
		_ = sub.Close()
		return nil, nil, err
	}
	ch := make(chan ActivityEvent, 32)
	done := make(chan struct{})
	go func() {
		defer close(ch)
		msgCh := sub.Channel()
		for {
			select {
			case <-done:
				_ = sub.Close()
				return
			case msg, ok := <-msgCh:
				if !ok {
					return
				}
				var ev ActivityEvent
				if err := json.Unmarshal([]byte(msg.Payload), &ev); err != nil {
					continue
				}
				select {
				case ch <- ev:
				case <-done:
					_ = sub.Close()
					return
				}
			}
		}
	}()
	unsub := func() {
		select {
		case <-done:
		default:
			close(done)
		}
	}
	return ch, unsub, nil
}
