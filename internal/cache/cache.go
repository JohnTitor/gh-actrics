package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Cache provides a simple file-based cache with TTL semantics.
type Cache struct {
	dir string
	ttl time.Duration

	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

// New creates a new Cache rooted at dir. The directory will be created if it
// does not exist. The provided ttl controls entry expiration; an entry older
// than ttl will be treated as a cache miss.
func New(dir string, ttl time.Duration) (*Cache, error) {
	if ttl <= 0 {
		return nil, errors.New("ttl must be positive")
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	return &Cache{
		dir:   dir,
		ttl:   ttl,
		locks: make(map[string]*sync.Mutex),
	}, nil
}

// Get returns the cached value for key if it exists and has not expired.
func (c *Cache) Get(key string) ([]byte, bool, error) {
	path, lock := c.pathFor(key)
	lock.Lock()
	defer lock.Unlock()

	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}

	if time.Since(info.ModTime()) > c.ttl {
		_ = os.Remove(path)
		return nil, false, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

// Set stores data for key in the cache.
func (c *Cache) Set(key string, data []byte) error {
	path, lock := c.pathFor(key)
	lock.Lock()
	defer lock.Unlock()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func (c *Cache) pathFor(key string) (string, *sync.Mutex) {
	hash := sha256.Sum256([]byte(key))
	hexed := hex.EncodeToString(hash[:])
	prefix := hexed[:2]
	path := filepath.Join(c.dir, prefix, hexed)

	c.mu.Lock()
	lock, ok := c.locks[path]
	if !ok {
		lock = &sync.Mutex{}
		c.locks[path] = lock
	}
	c.mu.Unlock()

	return path, lock
}
