package internal

import (
	"os"
	"path/filepath"
)

var (
	hostname string
	appname  string
)

func init() {
	hostname, _ = os.Hostname()
	appname = filepath.Base(os.Args[0])
}

type buildInfo struct{}

var BuildInfo = &buildInfo{}

func (b *buildInfo) AppName() string {
	return appname
}

func (b *buildInfo) Hostname() string {
	return hostname
}
