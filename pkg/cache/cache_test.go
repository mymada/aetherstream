package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLRUCache(t *testing.T) {
	c := NewLRUCache(10)
	require.NotNil(t, c)
	assert.Equal(t, 10, c.maxSize)
}

func TestLRUCache_GetSet(t *testing.T) {
	c := NewLRUCache(2)

	c.Set("a", "value-a", 1*time.Hour)
	c.Set("b", "value-b", 1*time.Hour)

	v, ok := c.Get("a")
	require.True(t, ok)
	assert.Equal(t, "value-a", v)

	v, ok = c.Get("b")
	require.True(t, ok)
	assert.Equal(t, "value-b", v)

	_, ok = c.Get("c")
	assert.False(t, ok)
}

func TestLRUCache_TTLEviction(t *testing.T) {
	c := NewLRUCache(10)
	c.Set("a", "value-a", 1*time.Millisecond)
	time.Sleep(2 * time.Millisecond)

	_, ok := c.Get("a")
	assert.False(t, ok, "expired entry should be evicted")
}

func TestLRUCache_LRU(t *testing.T) {
	c := NewLRUCache(2)
	c.Set("a", "value-a", 1*time.Hour)
	c.Set("b", "value-b", 1*time.Hour)
	c.Set("c", "value-c", 1*time.Hour)

	_, ok := c.Get("a")
	assert.False(t, ok, "oldest entry should be evicted")

	_, ok = c.Get("b")
	assert.True(t, ok)

	_, ok = c.Get("c")
	assert.True(t, ok)
}

func TestLRUCache_Delete(t *testing.T) {
	c := NewLRUCache(10)
	c.Set("a", "value-a", 1*time.Hour)
	c.Delete("a")

	_, ok := c.Get("a")
	assert.False(t, ok)
}

func TestLRUCache_Clear(t *testing.T) {
	c := NewLRUCache(10)
	c.Set("a", "value-a", 1*time.Hour)
	c.Set("b", "value-b", 1*time.Hour)
	c.Clear()

	_, ok := c.Get("a")
	assert.False(t, ok)
	_, ok = c.Get("b")
	assert.False(t, ok)
	assert.Equal(t, 0, c.order.Len())
}

func TestLRUCache_UpdateExisting(t *testing.T) {
	c := NewLRUCache(10)
	c.Set("a", "old", 1*time.Hour)
	c.Set("a", "new", 1*time.Hour)

	v, ok := c.Get("a")
	require.True(t, ok)
	assert.Equal(t, "new", v)
}

func TestLRUCache_DefaultSize(t *testing.T) {
	c := NewLRUCache(0)
	assert.Equal(t, 1000, c.maxSize)
}

func TestKeyHelpers(t *testing.T) {
	assert.Equal(t, "metadata:movie:123", MetadataKey("movie", "123"))
	assert.Equal(t, "poster:abc", PosterKey("abc"))
	assert.Equal(t, "epg:ch1", EPGKey("ch1"))
}
