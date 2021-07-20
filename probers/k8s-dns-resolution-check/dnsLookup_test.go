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

package main

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

func TestDnsLookup(t *testing.T) {
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address2 string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Millisecond * time.Duration(10000),
			}
			return d.DialContext(ctx, "udp", "8.8.8.8:53")
		},
	}
	testCase := make(map[string]error)
	testCase["bad.host.com"] = errors.New("DNS Status check determined that bad.host.com is DOWN")
	testCase["google.com"] = nil

	for arg, expectedValue := range testCase {
		host := arg

		err := dnsLookup(r, host)
		switch err {
		case nil:
			if host != "google.com" {
				t.Fatalf("doChecks failed to validate hostname correctly. Hostname: %s, Expected Check Result: %v", arg, expectedValue)
			}
			t.Logf("doChecks correctly validated hostname. ")
		default:
			if !strings.Contains(err.Error(), expectedValue.Error()) {
				t.Fatalf("doChecks failed to validate hostname correctly. Hostname: %s, Expected Check Result: %v", arg, expectedValue)
			}
			t.Logf("doChecks correctly validated hostname. ")
		}
	}
}
