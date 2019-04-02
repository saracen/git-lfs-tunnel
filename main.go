package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/saracen/lfscache/server"
	"golang.org/x/net/http2"
)

type StdinStdoutConn struct {
	io.ReadCloser
	io.WriteCloser
}

func (conn *StdinStdoutConn) Close() error {
	conn.ReadCloser.Close()
	conn.WriteCloser.Close()
	return nil
}

func (*StdinStdoutConn) LocalAddr() net.Addr                { return addr{} }
func (*StdinStdoutConn) RemoteAddr() net.Addr               { return addr{} }
func (*StdinStdoutConn) SetDeadline(t time.Time) error      { return fmt.Errorf("unsupported") }
func (*StdinStdoutConn) SetReadDeadline(t time.Time) error  { return fmt.Errorf("unsupported") }
func (*StdinStdoutConn) SetWriteDeadline(t time.Time) error { return fmt.Errorf("unsupported") }

type addr struct{}

func (addr) Network() string { return "stdinstdoutconn" }
func (addr) String() string  { return "stdinstdoutconn" }

var ErrUnsupportedUpstreamServer = errors.New("unsupported upstream server")

func main() {
	var logger log.Logger
	{
		logger = log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
		logger = level.NewFilter(logger, level.AllowInfo())
		logger = log.With(logger, "ts", log.DefaultTimestampUTC)
	}

	unsupported := func() {
		level.Error(logger).Log("err", ErrUnsupportedUpstreamServer)
		os.Exit(1)
	}

	if len(os.Args) != 2 {
		unsupported()
	}

	addr, err := url.Parse(os.Args[1])
	if err != nil || (addr.Scheme != "http" && addr.Scheme != "https") {
		unsupported()
	}

	handler, err := server.NewNoCache(logger, addr.String())
	if err != nil {
		panic(err)
	}

	handler.ObjectBatchActionURLRewriter = func(href *url.URL) *url.URL {
		href.Scheme = "https"
		href.Path = path.Join(addr.Path, href.Path)
		return href
	}

	s := http2.Server{}
	s.ServeConn(&StdinStdoutConn{os.Stdin, os.Stdout}, &http2.ServeConnOpts{Handler: http.StripPrefix(addr.Path, handler.Handle())})
}
