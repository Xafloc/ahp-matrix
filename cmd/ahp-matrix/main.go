// Command ahp-matrix starts a local web app for AHP decision making.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/Xafloc/ahp-matrix/internal/server"
	"github.com/Xafloc/ahp-matrix/internal/store"
	"github.com/Xafloc/ahp-matrix/web"
)

// version is set at build time with -ldflags "-X main.version=…".
var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println(version)
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, os.Args[1:], os.Stdout)
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "ahp-matrix:", err)
		os.Exit(1)
	}
}

// run starts the server and blocks until ctx is cancelled.
func run(ctx context.Context, args []string, out io.Writer) error {
	flags := flag.NewFlagSet("ahp-matrix", flag.ContinueOnError)
	flags.SetOutput(out)
	host := flags.String("host", "127.0.0.1", "address to listen on; there is no login, so keep it local")
	port := flags.Int("port", 7777, "port to listen on; a free port is used if it is busy")
	dataDir := flags.String("data-dir", "", "folder for project files (default: your user data folder)")
	noOpen := flags.Bool("no-open", false, "don't open a browser")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	dir := *dataDir
	if dir == "" {
		d, err := defaultDataDir()
		if err != nil {
			return err
		}
		dir = d
	}
	st, err := store.NewFileStore(dir)
	if err != nil {
		return err
	}
	ln, err := listen(*host, *port)
	if err != nil {
		return err
	}
	url := "http://" + net.JoinHostPort(browseHost(*host), strconv.Itoa(ln.Addr().(*net.TCPAddr).Port))

	srv := &http.Server{
		Handler:           server.New(server.Config{Store: st, UI: web.FS(), Version: version, Host: *host}),
		ReadHeaderTimeout: 10 * time.Second,
	}
	fmt.Fprintf(out, "AHP Matrix %s\n  projects: %s\n  open:     %s\n", version, dir, url)
	if ip := net.ParseIP(*host); *host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		fmt.Fprintf(out, "  warning:  listening on %s, and there is no login; anyone who can reach this address can edit your projects\n", *host)
	}
	if !*noOpen {
		_ = openBrowser(url)
	}

	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdown)
	}
}

// listen opens host:port, falling back to any free port if that one is taken.
func listen(host string, port int) (net.Listener, error) {
	ln, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err == nil || port == 0 {
		return ln, err
	}
	return net.Listen("tcp", net.JoinHostPort(host, "0"))
}

// browseHost is the host to put in the printed URL.
func browseHost(host string) string {
	if host == "" || host == "0.0.0.0" || host == "::" {
		return "127.0.0.1"
	}
	return host
}

// defaultDataDir is ~/Library/Application Support/ahp-matrix on macOS,
// %AppData%\ahp-matrix on Windows, and $XDG_DATA_HOME/ahp-matrix (default
// ~/.local/share/ahp-matrix) elsewhere.
func defaultDataDir() (string, error) {
	switch runtime.GOOS {
	case "darwin", "windows":
		d, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(d, "ahp-matrix"), nil
	}
	if x := os.Getenv("XDG_DATA_HOME"); x != "" {
		return filepath.Join(x, "ahp-matrix"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "ahp-matrix"), nil
}
