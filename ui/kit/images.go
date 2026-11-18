package kit

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/jpeg" // cover formats
	_ "image/png"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	_ "golang.org/x/image/webp"
)

// ImageCache keeps decoded images by key (file path + version, or URL).
type ImageCache struct {
	mu    sync.Mutex
	items map[string]image.Image
	order []string
	// pending dedupes concurrent loads; failed remembers failures.
	pending map[string][]func(image.Image)
	failed  map[string]bool
	client  *http.Client
}

const imageCacheMax = 300

var Images = &ImageCache{
	items:   map[string]image.Image{},
	pending: map[string][]func(image.Image){},
	failed:  map[string]bool{},
	client:  &http.Client{Timeout: 30 * time.Second},
}

func (c *ImageCache) put(key string, img image.Image) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.items[key]; !ok {
		c.order = append(c.order, key)
	}
	c.items[key] = img
	for len(c.order) > imageCacheMax {
		delete(c.items, c.order[0])
		c.order = c.order[1:]
	}
}

// load calls done (on the UI thread) with the image, or nil on failure.
func (c *ImageCache) load(key string, fetch func() ([]byte, error), done func(image.Image)) {
	c.mu.Lock()
	if img, ok := c.items[key]; ok {
		c.mu.Unlock()
		done(img)
		return
	}
	if c.failed[key] {
		c.mu.Unlock()
		done(nil)
		return
	}
	if waiters, ok := c.pending[key]; ok {
		c.pending[key] = append(waiters, done)
		c.mu.Unlock()
		return
	}
	c.pending[key] = []func(image.Image){done}
	c.mu.Unlock()

	go func() {
		var img image.Image
		data, err := fetch()
		if err == nil {
			img, _, err = image.Decode(bytes.NewReader(data))
		}
		c.mu.Lock()
		waiters := c.pending[key]
		delete(c.pending, key)
		if err != nil {
			c.failed[key] = true
		}
		c.mu.Unlock()
		if err == nil {
			c.put(key, img)
		}
		OnUI(func() {
			for _, w := range waiters {
				w(img)
			}
		})
	}()
}

func (c *ImageCache) LoadFile(path string, version int64, done func(image.Image)) {
	c.load(fmt.Sprintf("%s?%d", path, version), func() ([]byte, error) { return os.ReadFile(path) }, done)
}

func (c *ImageCache) LoadURL(url string, done func(image.Image)) {
	c.load(url, func() ([]byte, error) {
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
	}, done)
}

// CoverSource says where a cover comes from (first non-empty wins).
type CoverSource struct {
	Path    string
	Version int64
	URL     string
	Name    string // placeholder initial
	Contain bool   // show the whole image instead of filling (logos, icons)
}

// CoverView shows a cover image (2:3) with a letter placeholder.
func CoverView(src CoverSource, size fyne.Size) fyne.CanvasObject {
	bg := canvas.NewRectangle(ColBg3)
	bg.CornerRadius = 8
	bg.StrokeColor = ColLine
	bg.StrokeWidth = 1
	letter := canvas.NewText(initial(src.Name), ColMuted)
	letter.TextSize = size.Height / 5
	letter.TextStyle.Bold = true
	letter.Alignment = fyne.TextAlignCenter
	if src.Name == "" {
		letter.Text = ""
	}
	img := canvas.NewImageFromImage(nil)
	img.FillMode = canvas.ImageFillCover
	if src.Contain {
		img.FillMode = canvas.ImageFillContain
	}
	img.CornerRadius = 8
	img.Hide()
	set := func(i image.Image) {
		if i == nil {
			return
		}
		img.Image = i
		img.Show()
		letter.Hide()
		img.Refresh()
	}
	switch {
	case src.Path != "":
		Images.LoadFile(src.Path, src.Version, set)
	case src.URL != "":
		Images.LoadURL(src.URL, set)
	}
	return FixedSize(size, container.NewStack(bg, container.NewCenter(letter), img))
}
