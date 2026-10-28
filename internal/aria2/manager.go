package aria2

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/incurl/bitmaher/internal/console"
)

// Manager owns the lifecycle of the Aria2-Next sidecar.
//
// # LICENSING BOUNDARY
//
// The sidecar is a SEPARATE PROCESS. The manager starts it, watches it,
// talks to it over JSON-RPC and stops it again. It never links against
// aria2, never imports its sources and never embeds them.
//
// The manager is what keeps the boundary intact in practice:
//
//   - the binary is executed as its own process, so it is a separate
//     program with its own address space;
//   - the only channel is a loopback HTTP socket carrying JSON-RPC;
//   - the secret token means that only this process can drive it.
//
// The manager does not implement any part of the download protocol.
// Everything about HTTP, BitTorrent and Metalink is the engine's work.
type Manager struct {
	// mu guards the fields below.
	mu sync.Mutex

	// binary is the path of the executable that is started.
	binary string
	// args are the arguments the executable is started with.
	args []string
	// dataDir is where the engine keeps its own files: the session
	// file, the logs and the temporary downloads.
	dataDir string

	// cmd is the running sidecar, if there is one.
	cmd *exec.Cmd
	// waitDone is closed when the sidecar process has exited.
	//
	// Exactly one goroutine waits for the process, so that the exit
	// status is never read twice.
	waitDone chan struct{}
	// exitCode is the code the sidecar exited with, once it has.
	exitCode int
	// port is the port the RPC endpoint listens on.
	port int
	// secret is the token the engine authenticates the caller with.
	secret string
	// client talks to the engine.
	client *Client

	// ready reports whether the engine answered a version call.
	ready bool
	// version is the identity of the running engine.
	version Version

	// log collects the engine's output.
	log *console.Log

	// cancel stops the watchdog.
	cancel context.CancelFunc
	// wg waits for the watchdog.
	wg sync.WaitGroup

	// restart is whether a crashed sidecar is started again.
	restart bool
	// autoRestart is the user-facing switch for the watchdog.
	autoRestart bool

	// stderr collects the engine's standard error.
	stderrPath string
}

// ManagerOption configures a Manager.
type ManagerOption func(*Manager)

// WithBinary sets the executable that is started.
//
// It is either the bundled binary or the external one the user chose
// in the settings.
func WithBinary(path string) ManagerOption {
	return func(m *Manager) { m.binary = path }
}

// WithDataDir sets the directory the engine keeps its files in.
func WithDataDir(dir string) ManagerOption {
	return func(m *Manager) { m.dataDir = dir }
}

// WithSecret sets the token the engine is started with.
//
// The token is generated once, on the first run, and stored encrypted
// by the caller. The same token is used again on every later run, so
// that the session file stays usable.
func WithSecret(secret string) ManagerOption {
	return func(m *Manager) { m.secret = secret }
}

// WithAutoRestart sets whether a crashed sidecar is started again.
func WithAutoRestart(enabled bool) ManagerOption {
	return func(m *Manager) { m.autoRestart = enabled }
}

// WithLog attaches the log the engine's output is written to.
func WithLog(log *console.Log) ManagerOption {
	return func(m *Manager) { m.log = log }
}

// NewManager creates a manager for the sidecar.
func NewManager(opts ...ManagerOption) *Manager {
	m := &Manager{
		autoRestart: true,
		restart:     true,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(m)
		}
	}
	return m
}

// GenerateSecret returns a new random token for the RPC endpoint.
//
// The token is 32 random bytes, hex encoded. It is the engine's
// authentication: a request without it is refused, so it keeps other
// processes on the machine from driving the sidecar.
func GenerateSecret() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("aria2: generate the RPC secret: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}

// Port returns the port the RPC endpoint is on.
func (m *Manager) Port() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.port
}

// Endpoint returns the URL of the RPC endpoint, or an empty string when
// the sidecar is not running.
func (m *Manager) Endpoint() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.port == 0 {
		return ""
	}
	return fmt.Sprintf("http://127.0.0.1:%d/jsonrpc", m.port)
}

// Secret returns the token the sidecar is started with.
//
// It is never sent to the frontend: the Go process is the only
// caller of the engine.
func (m *Manager) Secret() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.secret
}

// Ready reports whether the sidecar is running and has answered.
func (m *Manager) Ready() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ready
}

// Version returns the identity of the running engine.
func (m *Manager) Version() Version {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.version
}

// Client returns the RPC client, or nil when the sidecar is not
// running.
func (m *Manager) Client() *Client {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.client
}

// PID returns the process id of the sidecar, or zero when it is not
// running.
//
// It is used by the tests and by the checks, which need to reach the
// child process.
func (m *Manager) PID() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cmd == nil || m.cmd.Process == nil {
		return 0
	}
	return m.cmd.Process.Pid
}

// freePort picks a free port on the loopback interface.
//
// The port is chosen by binding it, letting the operating system hand
// out an unused one and closing it again. There is a small window
// between the close and the sidecar binding the port, which is the
// usual race of this approach: the manager handles it by retrying
// with another port when the sidecar cannot bind.
func freePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("aria2: find a free port: %w", err)
	}
	defer listener.Close()

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("aria2: the listener has an unexpected address")
	}
	return addr.Port, nil
}

// Start starts the sidecar and waits until it answers.
//
// The engine is verified before it is marked as ready: the version
// call must answer and the engine must identify itself as aria2-next.
// A binary that is not aria2-next is refused, because BitMaher is
// built against the aria2-next RPC surface.
//
// The token is generated when none was supplied, and is returned so
// that the caller can store it.
func (m *Manager) Start(ctx context.Context) (string, error) {
	secret, err := m.start(ctx)
	if err != nil {
		return secret, err
	}

	// The watchdog watches the sidecar that was just started.
	m.startWatchdog()
	return secret, nil
}

// start starts the sidecar without starting a watchdog.
//
// The watchdog calls it when it restarts a sidecar: the watchdog is
// already running, and a second one would wait for the first at the
// shutdown.
func (m *Manager) start(ctx context.Context) (string, error) {
	m.mu.Lock()
	if m.cmd != nil {
		m.mu.Unlock()
		return m.secret, fmt.Errorf("aria2: the sidecar is already running")
	}
	if m.binary == "" {
		m.mu.Unlock()
		return "", fmt.Errorf("aria2: no aria2-next binary was configured")
	}
	if m.secret == "" {
		secret, err := GenerateSecret()
		if err != nil {
			m.mu.Unlock()
			return "", err
		}
		m.secret = secret
	}
	secret := m.secret
	binary := m.binary
	m.mu.Unlock()

	// The engine keeps its session file and its logs in the data
	// directory. The session file is what lets a download survive a
	// restart of the application: it holds the gid of every download
	// that was still running.
	if err := os.MkdirAll(m.dataDir, 0o700); err != nil {
		return "", fmt.Errorf("aria2: create the sidecar data dir: %w", err)
	}

	// The port is picked per start, so that two instances of the
	// application never fight over the same one.
	const portAttempts = 5
	var lastErr error
	for attempt := 0; attempt < portAttempts; attempt++ {
		port, err := freePort()
		if err != nil {
			return "", err
		}

		if err := m.launch(binary, port, secret); err != nil {
			lastErr = err
			// A port that is taken is the one failure worth
			// retrying: the next port is likely free.
			if strings.Contains(err.Error(), "Address already in use") {
				continue
			}
			return "", err
		}

		if err := m.waitReady(ctx, port, secret); err != nil {
			m.stopProcess()
			lastErr = err
			if strings.Contains(err.Error(), "Address already in use") {
				continue
			}
			return "", err
		}

		m.mu.Lock()
		m.port = port
		m.client = NewClient(fmt.Sprintf("http://127.0.0.1:%d/jsonrpc", port), secret)
		m.ready = true
		m.mu.Unlock()

		return secret, nil
	}

	return "", fmt.Errorf("aria2: the sidecar did not start: %w", lastErr)
}

// launch starts the process.
//
// The engine is told to keep its session, so that the downloads it is
// working on survive a restart of the application.
func (m *Manager) launch(binary string, port int, secret string) error {
	sessionPath := filepath.Join(m.dataDir, "session.txt")
	logPath := filepath.Join(m.dataDir, "aria2-next.log")

	args := []string{
		"--enable-rpc=true",
		fmt.Sprintf("--rpc-listen-port=%d", port),
		fmt.Sprintf("--rpc-secret=%s", secret),
		// The engine must not listen on anything but the loopback
		// interface: the RPC endpoint carries a secret, but a
		// download manager on the network has no business driving
		// this one.
		"--rpc-listen-all=false",
		fmt.Sprintf("--dir=%s", m.dataDir),
		fmt.Sprintf("--save-session=%s", sessionPath),
		"--save-session-interval=5",
		fmt.Sprintf("--log=%s", logPath),
		"--log-level=notice",
		"--console-log-level=warn",
	}

	// The session is read back on the next start, so that a download
	// that was running is picked up again with the same gid.
	//
	// The engine refuses to start when the file does not exist, so it
	// is only passed once there is one.
	if _, err := os.Stat(sessionPath); err == nil {
		args = append(args, fmt.Sprintf("--input-file=%s", sessionPath))
	}

	cmd := exec.Command(binary, args...)
	// The engine must not outlive the application, so it gets its
	// own process group: the shutdown can then signal it directly.
	cmd.SysProcAttr = processGroupAttributes()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("aria2: open the sidecar output: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("aria2: open the sidecar error output: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("aria2: start the sidecar %q: %w", binary, err)
	}

	// The process is reaped by exactly one goroutine, so that the
	// exit status is read once and the watchdog and the shutdown can
	// both watch for it without racing.
	waitDone := make(chan struct{})

	m.mu.Lock()
	m.cmd = cmd
	m.waitDone = waitDone
	m.exitCode = 0
	m.mu.Unlock()

	go func() {
		err := cmd.Wait()

		m.mu.Lock()
		m.exitCode = exitCodeOf(err)
		m.mu.Unlock()

		close(waitDone)
	}()

	// The output is read in the background and written to the log, so
	// that a crash is visible in the debug console.
	go m.pipe("stdout", stdout)
	go m.pipe("stderr", stderr)

	return nil
}

// pipe reads the engine's output and writes it to the log.
func (m *Manager) pipe(name string, reader io.Reader) {
	buffer := make([]byte, 4096)
	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			m.log.Append(console.LevelRaw, "aria2", "the sidecar wrote to "+name, string(buffer[:n]))
		}
		if err != nil {
			return
		}
	}
}

// waitReady waits until the engine answers the version call.
//
// The engine needs a moment to bind its port, so the call is retried
// for a short while before the start is given up on.
func (m *Manager) waitReady(ctx context.Context, port int, secret string) error {
	endpoint := fmt.Sprintf("http://127.0.0.1:%d/jsonrpc", port)
	client := NewClient(endpoint, secret)

	const (
		attempts = 40
		delay    = 100 * time.Millisecond
	)

	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}

		callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		version, err := client.Version(callCtx)
		cancel()
		if err != nil {
			lastErr = err
			continue
		}

		// The binary has to be aria2-next. The upstream aria2 has
		// no product name and a different RPC surface, and it is
		// not what BitMaher is built against.
		if !version.IsAria2Next() {
			return fmt.Errorf("aria2: the sidecar is %q, not aria2-next", version.Product)
		}

		m.mu.Lock()
		m.version = version
		m.mu.Unlock()

		m.log.Info("aria2", fmt.Sprintf("the sidecar is ready: %s %s", version.Product, version.Version))
		return nil
	}

	return fmt.Errorf("aria2: the sidecar did not answer on port %d: %w", port, lastErr)
}

// Stop shuts the sidecar down.
//
// The engine is asked to stop over RPC first, so that it finishes what
// it is doing and writes its session file. If it does not answer, or
// it does not stop in time, the process is signalled.
//
// The watchdog is stopped first, so that a sidecar that is on its way
// out is not started again.
func (m *Manager) Stop(ctx context.Context) error {
	m.stopWatchdog()

	m.mu.Lock()
	client := m.client
	waitDone := m.waitDone
	m.ready = false
	m.client = nil
	m.mu.Unlock()

	if client != nil {
		// The engine finishes the downloads it is working on, which
		// can take a moment, so the call gets its own budget.
		shutdownCtx, cancel := context.WithTimeout(ctx, shutdownGrace)
		defer cancel()

		if err := client.Shutdown(shutdownCtx); err != nil {
			m.log.Warn("aria2", "the sidecar did not stop politely: "+err.Error())
		}
	}

	if waitDone == nil {
		return nil
	}

	// The engine is given a moment to leave on its own.
	select {
	case <-waitDone:
		m.log.Info("aria2", "the sidecar stopped")
		return nil
	case <-time.After(shutdownGrace):
	}

	// It is still there, so it is signalled.
	m.log.Warn("aria2", "the sidecar did not stop in time, it is being signalled")
	return m.stopProcess()
}

// stopProcess signals the sidecar and waits for it to go.
//
// The whole process group is signalled, because the engine may have
// children of its own.
func (m *Manager) stopProcess() error {
	m.mu.Lock()
	cmd := m.cmd
	waitDone := m.waitDone
	m.cmd = nil
	m.ready = false
	m.client = nil
	m.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return nil
	}

	if err := signalProcessGroup(cmd.Process); err != nil {
		return fmt.Errorf("aria2: stop the sidecar: %w", err)
	}

	if waitDone == nil {
		return nil
	}

	select {
	case <-waitDone:
		return nil
	case <-time.After(killGrace):
		// The engine ignored the polite request, so it is killed.
		_ = killProcessGroup(cmd.Process)
		<-waitDone
		return nil
	}
}

// startWatchdog starts the goroutine that restarts a crashed sidecar.
//
// Only one watchdog runs at a time: the goroutine that watches a
// sidecar which was started again would otherwise keep running, and
// the shutdown would wait for all of them.
func (m *Manager) startWatchdog() {
	m.mu.Lock()

	// A watchdog that is already running is stopped first.
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.mu.Unlock()

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.watch(ctx)
	}()
}

// stopWatchdog stops the watchdog and waits for it.
func (m *Manager) stopWatchdog() {
	m.mu.Lock()
	cancel := m.cancel
	m.cancel = nil
	m.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	m.wg.Wait()
}

// watch restarts the sidecar when it dies unexpectedly.
//
// The sidecar is expected to be alive for as long as the application
// is. When it is not, and the user asked for the watchdog, it is
// started again on a new port.
//
// The restart is not immediate: the engine is given a moment to leave,
// so that a deliberate stop is not mistaken for a crash.
func (m *Manager) watch(ctx context.Context) {
	for {
		m.mu.Lock()
		waitDone := m.waitDone
		enabled := m.autoRestart
		m.mu.Unlock()

		if waitDone == nil {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-waitDone:
			// The sidecar has left. The watchdog decides whether
			// that was a crash or a deliberate stop.
		case <-time.After(watchdogInterval):
			continue
		}

		m.mu.Lock()
		stillOurs := m.waitDone == waitDone
		m.mu.Unlock()

		// The shutdown already took the sidecar down on purpose, so
		// there is nothing to restart.
		if !stillOurs {
			return
		}

		if !enabled {
			m.log.Error("aria2", "the sidecar stopped and the watchdog is off")
			m.mu.Lock()
			m.ready = false
			m.client = nil
			m.waitDone = nil
			m.mu.Unlock()
			return
		}

		m.log.Warn("aria2", "the sidecar stopped, it is being started again")
		m.mu.Lock()
		m.cmd = nil
		m.waitDone = nil
		m.mu.Unlock()

		// The sidecar is started without a new watchdog: this one
		// keeps watching.
		if _, err := m.start(ctx); err != nil {
			m.log.Error("aria2", "the sidecar could not be started again: "+err.Error())
			return
		}
	}
}

// SetAutoRestart turns the watchdog on or off.
func (m *Manager) SetAutoRestart(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.autoRestart = enabled
}

// AutoRestart reports whether the watchdog is on.
func (m *Manager) AutoRestart() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.autoRestart
}

// The timings of the sidecar's lifecycle.
const (
	// shutdownGrace is how long the engine may take to stop on its
	// own after it has been asked to.
	shutdownGrace = 5 * time.Second
	// killGrace is how long the engine may take after it has been
	// signalled, before it is killed.
	killGrace = 3 * time.Second
	// watchdogInterval is how often the watchdog checks that the
	// sidecar is still alive.
	watchdogInterval = 2 * time.Second
)

// ErrNotRunning is returned when the sidecar is asked to do something
// while it is not running.
var ErrNotRunning = errors.New("aria2: the sidecar is not running")
