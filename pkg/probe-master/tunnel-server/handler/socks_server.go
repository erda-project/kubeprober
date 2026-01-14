// Copyright (c) 2021 Terminus, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package handler

import (
	"context"
	"fmt"
	"log"
	"net"
	"strconv"
	"sync"

	socks5 "github.com/armon/go-socks5"
	"github.com/sirupsen/logrus"
	xcontext "golang.org/x/net/context"

	"github.com/erda-project/kubeprober/pkg/probe-master/tunnel-client/clusterdialer"
)

type socksServer struct {
	clusterKey string
	dialerKey  string
	port       int
	listener   net.Listener
	server     *socks5.Server
}

func newSocksServer(clusterKey, dialerKey string, port int, listener net.Listener) (*socksServer, error) {
	server, err := socks5.New(&socks5.Config{
		Logger: log.New(logrus.StandardLogger().Out, fmt.Sprintf("[socks5 %s] ", clusterKey), log.LstdFlags),
		Dial: func(ctx xcontext.Context, network, addr string) (net.Conn, error) {
			stdCtx, ok := ctx.(context.Context)
			if !ok {
				stdCtx = context.Background()
			}
			conn, err := clusterdialer.DialContext(dialerKey)(stdCtx, network, addr)
			if err != nil {
				logrus.Errorf("socks5 dial failed for cluster %s to %s: %v", clusterKey, addr, err)
				return nil, err
			}
			return wrapConnForSocks(clusterKey, addr, conn), nil
		},
	})
	if err != nil {
		return nil, err
	}

	return &socksServer{
		clusterKey: clusterKey,
		dialerKey:  dialerKey,
		port:       port,
		listener:   listener,
		server:     server,
	}, nil
}

func (s *socksServer) Serve() error {
	return s.server.Serve(s.listener)
}

type connAddrWrapper struct {
	net.Conn
	local      net.Addr
	remote     net.Addr
	clusterKey string
	target     string
	closeOnce  *sync.Once
}

func (c connAddrWrapper) LocalAddr() net.Addr {
	if c.local != nil {
		return c.local
	}
	return c.Conn.LocalAddr()
}

func (c connAddrWrapper) RemoteAddr() net.Addr {
	if c.remote != nil {
		return c.remote
	}
	return c.Conn.RemoteAddr()
}

func (c connAddrWrapper) Close() error {
	err := c.Conn.Close()
	if c.closeOnce != nil {
		c.closeOnce.Do(func() {
			logrus.Infof("socks5 connection closed for cluster %s to %s", c.clusterKey, c.target)
		})
	}
	return err
}

func wrapConnForSocks(clusterKey, target string, conn net.Conn) net.Conn {
	local := ensureTCPAddr(conn.LocalAddr())
	remote := ensureTCPAddr(conn.RemoteAddr())
	return connAddrWrapper{
		Conn:       conn,
		local:      local,
		remote:     remote,
		clusterKey: clusterKey,
		target:     target,
		closeOnce:  &sync.Once{},
	}
}

func ensureTCPAddr(addr net.Addr) *net.TCPAddr {
	if addr == nil {
		return &net.TCPAddr{IP: net.IPv4zero, Port: 0}
	}
	if tcpAddr, ok := addr.(*net.TCPAddr); ok {
		return tcpAddr
	}

	host, portStr, err := net.SplitHostPort(addr.String())
	if err != nil {
		return &net.TCPAddr{IP: net.IPv4zero, Port: 0}
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		port = 0
	}

	ip := net.ParseIP(host)
	if ip == nil {
		ip = net.IPv4zero
	}

	return &net.TCPAddr{IP: ip, Port: port}
}
