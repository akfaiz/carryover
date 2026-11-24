package kit

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
)

// ImageCache keeps decoded images as MyGo bitmaps by key (file path and
// version, or URL). Asking for an image never blocks: it starts the load and
// returns nil until the bitmap is in, when the windows are redrawn.
type ImageCache struct {
	mu     sync.Mutex
	items  map[string]*ui.Bitmap
	seen   map[string]int // times a bitmap was handed to a view, up to 2
	order  []string
	loads  map[string]bool // loads on their way
	failed map[string]bool
	client *http.Client
}

const imageCacheMax = 300

// Images is the application's cover cache.
var Images = &ImageCache{
	items:  map[string]*ui.Bitmap{},
	seen:   map[string]int{},
	loads:  map[string]bool{},
	failed: map[string]bool{},
	client: &http.Client{Timeout: 30 * time.Second},
}

// File returns the bitmap of an image file, scaled so its longest side is at
// most maxSide pixels (0: as is), or nil while it loads or when it can't be
// read. Version tells a changed file from the one cached under its path.
func (c *ImageCache) File(path string, version int64, maxSide int) *ui.Bitmap {
	return c.get(fmt.Sprintf("%s?%d@%d", path, version, maxSide), maxSide, func() ([]byte, error) { return os.ReadFile(path) })
}

// URL returns the bitmap of an image on the web, as File does.
func (c *ImageCache) URL(url string, maxSide int) *ui.Bitmap {
	return c.get(fmt.Sprintf("%s@%d", url, maxSide), maxSide, func() ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		resp, err := c.client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("%s: %s", url, resp.Status)
		}
		return io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	})
}

func (c *ImageCache) get(key string, maxSide int, fetch func() ([]byte, error)) *ui.Bitmap {
	c.mu.Lock()
	defer c.mu.Unlock()
	if bm, ok := c.items[key]; ok {
		// A bitmap keeps its pixels on the CPU as well as on the GPU. The
		// second time a view asks for it, the first frame has drawn it, so
		// the CPU copy can go (MyGo makes it again if something needs it).
		// Views run on the UI goroutine, as ReleasePixels requires.
		switch c.seen[key] {
		case 0:
			c.seen[key] = 1
		case 1:
			c.seen[key] = 2
			bm.ReleasePixels()
		}
		return bm
	}
	if c.failed[key] || c.loads[key] {
		return nil
	}
	c.loads[key] = true
	go func() {
		var bm *ui.Bitmap
		data, err := fetch()
		if err == nil {
			if maxSide > 0 {
				bm, err = ui.DecodeBitmapScaled(data, maxSide)
			} else {
				bm, err = ui.DecodeBitmap(data)
			}
		}
		c.mu.Lock()
		delete(c.loads, key)
		if err != nil {
			c.failed[key] = true
		} else {
			c.items[key] = bm
			c.order = append(c.order, key)
			for len(c.order) > imageCacheMax {
				delete(c.items, c.order[0])
				delete(c.seen, c.order[0])
				c.order = c.order[1:]
			}
		}
		c.mu.Unlock()
		if err == nil {
			Redraw()
		}
	}()
	return nil
}
