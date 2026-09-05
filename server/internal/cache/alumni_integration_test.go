package cache

import (
	"context"
	"os"
	"testing"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
	"github.com/redis/go-redis/v9"
)

func TestExportCacheIsolatesAuthorizationAndInvalidates(t *testing.T) {
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("TEST_REDIS_ADDR is not configured")
	}
	client := redis.NewClient(&redis.Options{Addr: addr, Password: os.Getenv("TEST_REDIS_PASSWORD"), DB: 15})
	t.Cleanup(func() { _ = client.Close() })
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping test redis: %v", err)
	}
	previousVersion, versionErr := client.Get(ctx, alumniExportVersion).Result()
	if versionErr != nil && versionErr != redis.Nil {
		t.Fatalf("read previous export cache version: %v", versionErr)
	}
	t.Cleanup(func() {
		if versionErr == redis.Nil {
			_ = client.Del(ctx, alumniExportVersion).Err()
			return
		}
		_ = client.Set(ctx, alumniExportVersion, previousVersion, 0).Err()
	})

	cache := NewExportCache(client)
	byDomain := do.AlumniListQuery{Keyword: "cache-integration-isolation", DataDomainIDs: []uint64{2, 1}}.Normalize()
	equivalentDomainOrder := do.AlumniListQuery{Keyword: "cache-integration-isolation", DataDomainIDs: []uint64{1, 2}}.Normalize()
	withSensitivePermission := byDomain
	withSensitivePermission.CanReadSensitive = true
	otherDomain := do.AlumniListQuery{Keyword: "cache-integration-isolation", DataDomainIDs: []uint64{3}}.Normalize()

	key, err := cache.buildKey(ctx, byDomain)
	if err != nil {
		t.Fatalf("build scoped cache key: %v", err)
	}
	equivalentKey, err := cache.buildKey(ctx, equivalentDomainOrder)
	if err != nil {
		t.Fatalf("build equivalent cache key: %v", err)
	}
	permissionKey, err := cache.buildKey(ctx, withSensitivePermission)
	if err != nil {
		t.Fatalf("build permission cache key: %v", err)
	}
	otherDomainKey, err := cache.buildKey(ctx, otherDomain)
	if err != nil {
		t.Fatalf("build other-domain cache key: %v", err)
	}
	if key != equivalentKey || key == permissionKey || key == otherDomainKey {
		t.Fatalf("unexpected authorization cache keys: scoped=%s equivalent=%s permission=%s other_domain=%s", key, equivalentKey, permissionKey, otherDomainKey)
	}
	t.Cleanup(func() { _ = client.Del(ctx, key, permissionKey, otherDomainKey).Err() })

	if err := cache.Set(ctx, byDomain, []byte(`[{"name":"scoped"}]`)); err != nil {
		t.Fatalf("cache scoped export: %v", err)
	}
	if err := cache.Set(ctx, withSensitivePermission, []byte(`[{"name":"sensitive"}]`)); err != nil {
		t.Fatalf("cache sensitive export: %v", err)
	}
	if got, err := cache.Get(ctx, byDomain); err != nil || string(got) != `[{"name":"scoped"}]` {
		t.Fatalf("scoped cache result = %q, %v", got, err)
	}
	if got, err := cache.Get(ctx, withSensitivePermission); err != nil || string(got) != `[{"name":"sensitive"}]` {
		t.Fatalf("sensitive cache result = %q, %v", got, err)
	}
	if err := cache.Invalidate(ctx); err != nil {
		t.Fatalf("invalidate export cache: %v", err)
	}
	if _, err := cache.Get(ctx, byDomain); err != redis.Nil {
		t.Fatalf("invalidated cache error = %v, want redis.Nil", err)
	}
}
