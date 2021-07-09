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

package clusterdialer

import (
	"context"
	"net"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var session TunnelSession

func init() {
	clusterDialerEndpoint := "ws://127.0.0.1:8088/clusterdialer"
	go session.initialize(clusterDialerEndpoint)
}

type DialContextFunc func(ctx context.Context, network, address string) (net.Conn, error)
type DialContextProtoFunc func(ctx context.Context, address string) (net.Conn, error)

func DialContext(clusterKey string) DialContextFunc {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		logrus.Debugf("use cluster dialer, key:%s", clusterKey)
		f := session.getClusterDialer(ctx, clusterKey)
		if f == nil {
			return nil, errors.New("get cluster dialer failed")
		}
		return f(ctx, network, addr)
	}
}

func DialContextProto(clusterKey, proto string) DialContextProtoFunc {
	return func(ctx context.Context, addr string) (net.Conn, error) {
		logrus.Debugf("use cluster dialer, key:%s", clusterKey)
		f := session.getClusterDialer(ctx, clusterKey)
		if f == nil {
			return nil, errors.New("get cluster dialer failed")
		}
		return f(ctx, proto, addr)
	}
}

func DialContextTCP(clusterKey string) DialContextProtoFunc {
	return DialContextProto(clusterKey, "tcp")
}
