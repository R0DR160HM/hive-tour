package hive

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

func init() {
	if runtime.GOOS != "android" {
		return
	}
	net.DefaultResolver = &net.Resolver{PreferGo: true, Dial: resolverDial}
}

func resolverDial(ctx context.Context, network string, _ string) (net.Conn, error) {
	servers := resolvers()
	if len(servers) == 0 {
		return nil, errors.New("no resolver: nothing has written " +
			resolverFile() + ", so a name cannot be looked up here")
	}
	var dialer net.Dialer
	var last error
	for _, server := range servers {
		conn, err := dialer.DialContext(ctx, network, server)
		if err == nil {
			return conn, nil
		}
		last = err
	}
	return nil, last
}

var resolverMu sync.Mutex
var resolverStamp time.Time
var resolverCache []string

func resolvers() []string {
	path := resolverFile()
	if path == "" {
		return nil
	}
	resolverMu.Lock()
	defer resolverMu.Unlock()
	at, err := os.Stat(path)
	if err != nil {
		resolverCache = nil
		resolverStamp = time.Time{}
		return nil
	}
	if at.ModTime().Equal(resolverStamp) {
		return resolverCache
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return resolverCache
	}
	resolverStamp = at.ModTime()
	resolverCache = resolverAddresses(string(body))
	return resolverCache
}

func resolverFile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".hive", "resolvers")
}

func resolverAddresses(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if _, _, err := net.SplitHostPort(line); err != nil {
			line = net.JoinHostPort(line, "53")
		}
		out = append(out, line)
	}
	return out
}
