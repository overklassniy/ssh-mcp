package ssh

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
)

// ForwardDirection specifies whether a forward is local or remote.
type ForwardDirection string

const (
	ForwardLocal  ForwardDirection = "local"
	ForwardRemote ForwardDirection = "remote"
)

// ForwardAction specifies what to do with a forward.
type ForwardAction string

const (
	ForwardOpen  ForwardAction = "open"
	ForwardClose ForwardAction = "close"
	ForwardList  ForwardAction = "list"
)

// ForwardEntry represents an active port forward.
type ForwardEntry struct {
	ID         string           `json:"id"`
	Direction  ForwardDirection `json:"direction"`
	LocalAddr  string           `json:"localAddr"`
	RemoteAddr string           `json:"remoteAddr"`
	cancel     context.CancelFunc
}

// ForwardManager manages SSH port forwards for a single SSH connection.
type ForwardManager struct {
	mu       sync.Mutex
	forwards map[string]*ForwardEntry
	nextID   atomic.Int64
}

// NewForwardManager creates a new ForwardManager.
func NewForwardManager() *ForwardManager {
	return &ForwardManager{
		forwards: make(map[string]*ForwardEntry),
	}
}

// OpenLocalForward opens a local port forward: local -> remote via SSH.
// The forward persists until explicitly closed via CloseForward or CloseAll,
// independent of the request context.
func (fm *ForwardManager) OpenLocalForward(ctx context.Context, client SSHClient, localAddr, remoteAddr string) (*ForwardEntry, error) {
	listener, err := net.Listen("tcp", localAddr)
	if err != nil {
		return nil, NewToolError(CodeSSHConnectionFailed,
			fmt.Sprintf("listen on %s: %v", localAddr, err), false)
	}

	id := fmt.Sprintf("lf-%d", fm.nextID.Add(1))
	// Use context.Background() so the forward outlives the request.
	// The forward is cancelled explicitly via CloseForward/CloseAll.
	fwdCtx, cancel := context.WithCancel(context.Background())

	entry := &ForwardEntry{
		ID:        id,
		Direction: ForwardLocal,
		LocalAddr: listener.Addr().String(),
		RemoteAddr: remoteAddr,
		cancel:    cancel,
	}

	fm.mu.Lock()
	fm.forwards[id] = entry
	fm.mu.Unlock()

	go func() {
		<-fwdCtx.Done()
		listener.Close()
		fm.mu.Lock()
		delete(fm.forwards, id)
		fm.mu.Unlock()
	}()

	go fm.localForwardLoop(fwdCtx, listener, client, remoteAddr, id)

	return entry, nil
}

// localForwardLoop accepts connections and tunnels them to the remote address.
func (fm *ForwardManager) localForwardLoop(ctx context.Context, listener net.Listener, client SSHClient, remoteAddr, id string) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Warn("local forward accept failed", "id", id, "error", err)
			return
		}

		go func(localConn net.Conn) {
			defer localConn.Close()

			remoteConn, err := client.Client().Dial("tcp", remoteAddr)
			if err != nil {
				slog.Warn("local forward dial failed", "id", id, "error", err)
				return
			}
			defer remoteConn.Close()

			// Bidirectional copy
			done := make(chan struct{}, 2)
			go func() {
				io.Copy(remoteConn, localConn)
				done <- struct{}{}
			}()
			go func() {
				io.Copy(localConn, remoteConn)
				done <- struct{}{}
			}()
			<-done
		}(conn)
	}
}

// OpenRemoteForward opens a remote port forward: remote -> local via SSH.
// The forward persists until explicitly closed via CloseForward or CloseAll,
// independent of the request context.
func (fm *ForwardManager) OpenRemoteForward(ctx context.Context, client SSHClient, localAddr, remoteAddr string) (*ForwardEntry, error) {
	listener, err := client.Client().Listen("tcp", remoteAddr)
	if err != nil {
		return nil, NewToolError(CodeSSHConnectionFailed,
			fmt.Sprintf("listen on remote %s: %v", remoteAddr, err), false)
	}

	id := fmt.Sprintf("rf-%d", fm.nextID.Add(1))
	// Use context.Background() so the forward outlives the request.
	fwdCtx, cancel := context.WithCancel(context.Background())

	entry := &ForwardEntry{
		ID:        id,
		Direction: ForwardRemote,
		LocalAddr: localAddr,
		RemoteAddr: listener.Addr().String(),
		cancel:    cancel,
	}

	fm.mu.Lock()
	fm.forwards[id] = entry
	fm.mu.Unlock()

	go func() {
		<-fwdCtx.Done()
		listener.Close()
		fm.mu.Lock()
		delete(fm.forwards, id)
		fm.mu.Unlock()
	}()

	go fm.remoteForwardLoop(fwdCtx, listener, localAddr, id)

	return entry, nil
}

// remoteForwardLoop accepts connections from the remote side and forwards
// them to the local address.
func (fm *ForwardManager) remoteForwardLoop(ctx context.Context, listener net.Listener, localAddr, id string) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Warn("remote forward accept failed", "id", id, "error", err)
			return
		}

		go func(remoteConn net.Conn) {
			defer remoteConn.Close()

			localConn, err := net.Dial("tcp", localAddr)
			if err != nil {
				slog.Warn("remote forward dial local failed", "id", id, "error", err)
				return
			}
			defer localConn.Close()

			done := make(chan struct{}, 2)
			go func() {
				io.Copy(localConn, remoteConn)
				done <- struct{}{}
			}()
			go func() {
				io.Copy(remoteConn, localConn)
				done <- struct{}{}
			}()
			<-done
		}(conn)
	}
}

// CloseForward closes a port forward by ID.
func (fm *ForwardManager) CloseForward(id string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	entry, ok := fm.forwards[id]
	if !ok {
		return NewToolError(CodeUnknownError, fmt.Sprintf("forward %q not found", id), false)
	}
	entry.cancel()
	delete(fm.forwards, id)
	return nil
}

// ListForwards returns all active port forwards.
func (fm *ForwardManager) ListForwards() []*ForwardEntry {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	entries := make([]*ForwardEntry, 0, len(fm.forwards))
	for _, e := range fm.forwards {
		entries = append(entries, e)
	}
	return entries
}

// CloseAll closes all active port forwards.
func (fm *ForwardManager) CloseAll() {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	for _, e := range fm.forwards {
		e.cancel()
	}
	fm.forwards = make(map[string]*ForwardEntry)
}
