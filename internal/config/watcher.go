package config

import (
	"log/slog"
	"os"
	"time"
)

// DefaultWatchInterval is the polling interval used by Watcher when no
// explicit interval is provided. Two seconds balances responsiveness
// against negligible CPU cost.
const DefaultWatchInterval = 2 * time.Second

// Watcher polls a config file for modification-time changes and emits
// a freshly loaded *Config on the returned channel whenever the file
// changes.
//
// Polling is used instead of fsnotify because config files often live
// on network or cloud-synced filesystems (e.g. YandexDisk) where
// kernel-level file notifications are unreliable or unavailable.
type Watcher struct {
	path     string
	interval time.Duration
	stop     chan struct{}
}

// NewWatcher creates a polling watcher for the config file at path.
// interval controls how often the file's modification time is checked;
// if zero, DefaultWatchInterval is used.
func NewWatcher(path string, interval time.Duration) *Watcher {
	if interval <= 0 {
		interval = DefaultWatchInterval
	}
	return &Watcher{
		path:     path,
		interval: interval,
		stop:     make(chan struct{}),
	}
}

// Start begins polling in a background goroutine and returns a
// receive-only channel. A new *Config is sent each time the file's
// modification time changes and the file reloads successfully. The
// channel is closed when Stop is called.
//
// If a reload fails (parse error, validation error, etc.) the previous
// configuration is kept and the error is logged; no value is sent.
func (w *Watcher) Start() <-chan *Config {
	ch := make(chan *Config, 1)
	go w.loop(ch)
	return ch
}

// Stop terminates the watcher. The channel returned by Start will be
// closed. Stop is safe to call at most once.
func (w *Watcher) Stop() {
	close(w.stop)
}

func (w *Watcher) loop(ch chan<- *Config) {
	defer close(ch)

	var lastMod time.Time
	if fi, err := os.Stat(w.path); err == nil {
		lastMod = fi.ModTime()
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stop:
			return
		case <-ticker.C:
			fi, err := os.Stat(w.path)
			if err != nil {
				slog.Warn("config watch: stat failed, will retry",
					"path", w.path, "error", err)
				continue
			}
			if !fi.ModTime().After(lastMod) {
				continue
			}
			lastMod = fi.ModTime()

			cfg, err := Load(w.path)
			if err != nil {
				slog.Error("config reload failed, keeping previous config",
					"path", w.path, "error", err)
				continue
			}

			slog.Info("config file changed, reloading", "path", w.path)
			select {
			case ch <- cfg:
			case <-w.stop:
				return
			}
		}
	}
}
