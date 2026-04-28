package store

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/prasenjit/go-virtual/internal/config"
	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/redis/go-redis/v9"
)

const sessionTimeLayout = time.RFC3339Nano

// RedisSessionManager stores session state in Redis so multiple instances can
// share the same session namespace.
type RedisSessionManager struct {
	client *redis.Client
	global GlobalStoreBackend
	cfg    config.SessionConfig
	prefix string
}

var _ SessionRegistry = (*RedisSessionManager)(nil)

type redisSession struct {
	manager    *RedisSessionManager
	id         string
	createdAt  time.Time
	lastActive time.Time

	mu    sync.Mutex
	store map[string]any
}

var _ SessionState = (*redisSession)(nil)

func NewRedisSessionManager(ctx context.Context, global GlobalStoreBackend, cfg config.SessionConfig) (*RedisSessionManager, error) {
	cfg.Normalize()

	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Username: cfg.Redis.Username,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis session backend ping failed: %w", err)
	}

	return &RedisSessionManager{
		client: client,
		global: global,
		cfg:    cfg,
		prefix: strings.TrimSuffix(cfg.Redis.KeyPrefix, ":"),
	}, nil
}

func (m *RedisSessionManager) GetOrCreate(rawID string) (SessionState, bool, error) {
	ctx := context.Background()

	if rawID != "" {
		sess, ok, err := m.loadSession(ctx, rawID)
		if err != nil {
			return nil, false, err
		}
		if ok {
			if err := m.touchSession(ctx, sess); err != nil {
				return nil, false, err
			}
			return sess, false, nil
		}
	}

	if err := m.evictOldestIfNeeded(ctx); err != nil {
		return nil, false, err
	}

	id := rawID
	if id == "" {
		id = uuid.New().String()
	}
	var snapshot map[string]any
	if m.global != nil {
		snapshot = m.global.Snapshot()
	}
	now := time.Now().UTC()
	sess := &redisSession{
		manager:    m,
		id:         id,
		createdAt:  now,
		lastActive: now,
		store:      copySessionMap(snapshot),
	}
	if err := m.writeFullSession(ctx, sess); err != nil {
		return nil, false, err
	}
	return sess, true, nil
}

func (m *RedisSessionManager) Get(sessionID string) (SessionState, bool, error) {
	return m.loadSession(context.Background(), sessionID)
}

func (m *RedisSessionManager) Invalidate(sessionID string) error {
	ctx := context.Background()
	pipe := m.client.TxPipeline()
	pipe.Del(ctx, m.metaKey(sessionID), m.dataKey(sessionID))
	pipe.ZRem(ctx, m.indexKey(), sessionID)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("redis invalidate session %q: %w", sessionID, err)
	}
	return nil
}

func (m *RedisSessionManager) InvalidateAll() error {
	ctx := context.Background()
	keys, err := m.scanKeys(ctx, m.prefix+":meta:*")
	if err != nil {
		return err
	}
	dataKeys, err := m.scanKeys(ctx, m.prefix+":data:*")
	if err != nil {
		return err
	}
	keys = append(keys, dataKeys...)
	keys = append(keys, m.indexKey())
	if len(keys) == 0 {
		return nil
	}
	if err := m.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("redis invalidate all sessions: %w", err)
	}
	return nil
}

func (m *RedisSessionManager) ActiveSessions() ([]models.SessionInfo, error) {
	ctx := context.Background()
	ids, err := m.client.ZRevRange(ctx, m.indexKey(), 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("redis list sessions: %w", err)
	}

	infos := make([]models.SessionInfo, 0, len(ids))
	stale := make([]any, 0)
	for _, id := range ids {
		sess, ok, err := m.loadSession(ctx, id)
		if err != nil {
			return nil, err
		}
		if !ok {
			stale = append(stale, id)
			continue
		}
		infos = append(infos, sess.Info(false))
	}
	if len(stale) > 0 {
		if err := m.client.ZRem(ctx, m.indexKey(), stale...).Err(); err != nil {
			return nil, fmt.Errorf("redis prune stale sessions: %w", err)
		}
	}
	return infos, nil
}

func (m *RedisSessionManager) Count() (int, error) {
	infos, err := m.ActiveSessions()
	if err != nil {
		return 0, err
	}
	return len(infos), nil
}

func (m *RedisSessionManager) loadSession(ctx context.Context, id string) (*redisSession, bool, error) {
	meta, err := m.client.HGetAll(ctx, m.metaKey(id)).Result()
	if err != nil {
		return nil, false, fmt.Errorf("redis load session metadata %q: %w", id, err)
	}
	if len(meta) == 0 {
		if err := m.client.ZRem(ctx, m.indexKey(), id).Err(); err != nil {
			return nil, false, fmt.Errorf("redis prune missing session %q: %w", id, err)
		}
		return nil, false, nil
	}

	createdAt, err := time.Parse(sessionTimeLayout, meta["createdAt"])
	if err != nil {
		return nil, false, fmt.Errorf("redis parse session createdAt %q: %w", id, err)
	}
	lastActive, err := time.Parse(sessionTimeLayout, meta["lastActive"])
	if err != nil {
		return nil, false, fmt.Errorf("redis parse session lastActive %q: %w", id, err)
	}

	storeValues, err := m.client.HGetAll(ctx, m.dataKey(id)).Result()
	if err != nil {
		return nil, false, fmt.Errorf("redis load session data %q: %w", id, err)
	}
	sessionStore := make(map[string]any, len(storeValues))
	for key, raw := range storeValues {
		var value any
		if err := json.Unmarshal([]byte(raw), &value); err != nil {
			return nil, false, fmt.Errorf("redis decode session value %q[%q]: %w", id, key, err)
		}
		sessionStore[key] = value
	}

	return &redisSession{
		manager:    m,
		id:         id,
		createdAt:  createdAt.UTC(),
		lastActive: lastActive.UTC(),
		store:      sessionStore,
	}, true, nil
}

func (m *RedisSessionManager) touchSession(ctx context.Context, sess *redisSession) error {
	sess.mu.Lock()
	sess.lastActive = time.Now().UTC()
	createdAt := sess.createdAt
	lastActive := sess.lastActive
	sessionID := sess.id
	hasStore := len(sess.store) > 0
	sess.mu.Unlock()

	pipe := m.client.TxPipeline()
	pipe.HSet(ctx, m.metaKey(sessionID), map[string]any{
		"id":         sessionID,
		"createdAt":  createdAt.Format(sessionTimeLayout),
		"lastActive": lastActive.Format(sessionTimeLayout),
	})
	pipe.ZAdd(ctx, m.indexKey(), redis.Z{Score: float64(lastActive.UnixMilli()), Member: sessionID})
	pipe.Expire(ctx, m.metaKey(sessionID), m.cfg.InactivityTimeout)
	if hasStore {
		pipe.Expire(ctx, m.dataKey(sessionID), m.cfg.InactivityTimeout)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("redis touch session %q: %w", sessionID, err)
	}
	return nil
}

func (m *RedisSessionManager) writeFullSession(ctx context.Context, sess *redisSession) error {
	sess.mu.Lock()
	defer sess.mu.Unlock()

	pipe := m.client.TxPipeline()
	pipe.Del(ctx, m.metaKey(sess.id), m.dataKey(sess.id))
	pipe.HSet(ctx, m.metaKey(sess.id), map[string]any{
		"id":         sess.id,
		"createdAt":  sess.createdAt.Format(sessionTimeLayout),
		"lastActive": sess.lastActive.Format(sessionTimeLayout),
	})
	pipe.ZAdd(ctx, m.indexKey(), redis.Z{Score: float64(sess.lastActive.UnixMilli()), Member: sess.id})
	pipe.Expire(ctx, m.metaKey(sess.id), m.cfg.InactivityTimeout)
	if len(sess.store) > 0 {
		fields := make(map[string]any, len(sess.store))
		for key, value := range sess.store {
			encoded, err := json.Marshal(value)
			if err != nil {
				return fmt.Errorf("redis encode session value %q[%q]: %w", sess.id, key, err)
			}
			fields[key] = string(encoded)
		}
		pipe.HSet(ctx, m.dataKey(sess.id), fields)
		pipe.Expire(ctx, m.dataKey(sess.id), m.cfg.InactivityTimeout)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("redis write session %q: %w", sess.id, err)
	}
	return nil
}

func (m *RedisSessionManager) evictOldestIfNeeded(ctx context.Context) error {
	if m.cfg.MaxSessions <= 0 {
		return nil
	}
	infos, err := m.ActiveSessions()
	if err != nil {
		return err
	}
	if len(infos) < m.cfg.MaxSessions {
		return nil
	}
	oldest := infos[len(infos)-1]
	if err := m.Invalidate(oldest.ID); err != nil {
		return err
	}
	return nil
}

func (m *RedisSessionManager) scanKeys(ctx context.Context, pattern string) ([]string, error) {
	var (
		cursor uint64
		keys   []string
	)
	for {
		batch, next, err := m.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, fmt.Errorf("redis scan %q: %w", pattern, err)
		}
		keys = append(keys, batch...)
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return keys, nil
}

func (m *RedisSessionManager) metaKey(id string) string {
	return m.prefix + ":meta:" + id
}

func (m *RedisSessionManager) dataKey(id string) string {
	return m.prefix + ":data:" + id
}

func (m *RedisSessionManager) indexKey() string {
	return m.prefix + ":index"
}

func (s *redisSession) Get(key string) (any, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.store[key]
	return v, ok
}

func (s *redisSession) Set(key string, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode session value %q: %w", key, err)
	}

	s.mu.Lock()
	s.store[key] = value
	s.lastActive = time.Now().UTC()
	lastActive := s.lastActive
	createdAt := s.createdAt
	sessionID := s.id
	s.mu.Unlock()

	ctx := context.Background()
	pipe := s.manager.client.TxPipeline()
	pipe.HSet(ctx, s.manager.dataKey(sessionID), key, string(encoded))
	pipe.HSet(ctx, s.manager.metaKey(sessionID), map[string]any{
		"id":         sessionID,
		"createdAt":  createdAt.Format(sessionTimeLayout),
		"lastActive": lastActive.Format(sessionTimeLayout),
	})
	pipe.ZAdd(ctx, s.manager.indexKey(), redis.Z{Score: float64(lastActive.UnixMilli()), Member: sessionID})
	pipe.Expire(ctx, s.manager.metaKey(sessionID), s.manager.cfg.InactivityTimeout)
	pipe.Expire(ctx, s.manager.dataKey(sessionID), s.manager.cfg.InactivityTimeout)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("persist session value %q[%q]: %w", sessionID, key, err)
	}
	return nil
}

func (s *redisSession) Has(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.store[key]
	return ok
}

func (s *redisSession) Delete(key string) error {
	s.mu.Lock()
	delete(s.store, key)
	s.lastActive = time.Now().UTC()
	lastActive := s.lastActive
	createdAt := s.createdAt
	sessionID := s.id
	hasStore := len(s.store) > 0
	s.mu.Unlock()

	ctx := context.Background()
	pipe := s.manager.client.TxPipeline()
	pipe.HDel(ctx, s.manager.dataKey(sessionID), key)
	pipe.HSet(ctx, s.manager.metaKey(sessionID), map[string]any{
		"id":         sessionID,
		"createdAt":  createdAt.Format(sessionTimeLayout),
		"lastActive": lastActive.Format(sessionTimeLayout),
	})
	pipe.ZAdd(ctx, s.manager.indexKey(), redis.Z{Score: float64(lastActive.UnixMilli()), Member: sessionID})
	pipe.Expire(ctx, s.manager.metaKey(sessionID), s.manager.cfg.InactivityTimeout)
	if hasStore {
		pipe.Expire(ctx, s.manager.dataKey(sessionID), s.manager.cfg.InactivityTimeout)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("delete session value %q[%q]: %w", sessionID, key, err)
	}
	return nil
}

func (s *redisSession) Keys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	keys := make([]string, 0, len(s.store))
	for key := range s.store {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (s *redisSession) Snapshot() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return copySessionMap(s.store)
}

func (s *redisSession) Info(includeSnapshot bool) models.SessionInfo {
	s.mu.Lock()
	defer s.mu.Unlock()

	info := models.SessionInfo{
		ID:         s.id,
		CreatedAt:  s.createdAt,
		LastActive: s.lastActive,
		EntryCount: len(s.store),
	}
	if includeSnapshot {
		info.StoreSnapshot = copySessionMap(s.store)
	}
	return info
}

func copySessionMap(src map[string]any) map[string]any {
	if src == nil {
		return make(map[string]any)
	}
	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}
