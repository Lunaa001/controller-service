package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisCache struct {
	client *redis.Client
}

// New crea una nueva instancia de Redis cache
func New(redisURL, password string) (*RedisCache, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}

	if password != "" {
		opts.Password = password
	}

	client := redis.NewClient(opts)

	// Verificar conexión
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &RedisCache{client: client}, nil
}

// Set guarda un valor en Redis con TTL
func (rc *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return rc.client.Set(ctx, key, string(data), ttl).Err()
}

// Get obtiene un valor de Redis
func (rc *RedisCache) Get(ctx context.Context, key string, dest interface{}) error {
	val, err := rc.client.Get(ctx, key).Result()
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(val), dest)
}

// Delete elimina un valor de Redis
func (rc *RedisCache) Delete(ctx context.Context, keys ...string) error {
	return rc.client.Del(ctx, keys...).Err()
}

// Exists verifica si una clave existe
func (rc *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	count, err := rc.client.Exists(ctx, key).Result()
	return count > 0, err
}

// SetString guarda un string simple (sin JSON)
func (rc *RedisCache) SetString(ctx context.Context, key, value string, ttl time.Duration) error {
	return rc.client.Set(ctx, key, value, ttl).Err()
}

// GetString obtiene un string simple
func (rc *RedisCache) GetString(ctx context.Context, key string) (string, error) {
	return rc.client.Get(ctx, key).Result()
}

// Close cierra la conexión a Redis
func (rc *RedisCache) Close() error {
	return rc.client.Close()
}

// IsKeyNotFound verifica si es un error de clave no encontrada
func IsKeyNotFound(err error) bool {
	return err == redis.Nil
}
