package dns_resolution_checker

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/erda-project/kubeprober/pkg/kubeclient"
	proberchecker "github.com/erda-project/kubeprober/pkg/probe-checker"
)

func initEnv() {
	os.Setenv("USE_MOCK", "true")
}

func TestDnsChecker(t *testing.T) {
	var (
		err error
		dc  *DnsChecker
	)

	initEnv()

	defer func() {
		if err != nil {
			panic(err)
		}
	}()

	// check log debug level
	if cfg.Debug {
		logrus.SetLevel(logrus.DebugLevel)
		logrus.Debug("DEBUG MODE")
	}

	// create checkers
	// dns checker
	dc, err = NewChecker()
	if err != nil {
		err = fmt.Errorf("new dns checker failed, error: %v", err)
		return
	}

	// run checkers
	err = proberchecker.RunCheckers(proberchecker.CheckerList{dc})
	if err != nil {
		err = fmt.Errorf("run dns checker failed, private domain: %s, public domain: %s, error: %v", cfg.PrivateDomain, cfg.PublicDomain, err)
		return
	}
	logrus.Infof("run dns check success for for private domain: %s, public domain: %s", cfg.PrivateDomain, cfg.PublicDomain)

}

func TestGetIpsFromEndpoint(t *testing.T) {
	client, err := kubeclient.Client(filepath.Join(os.Getenv("HOME"), ".kube", "config"))
	if err != nil {
		t.Fatalf("Unable to create kube client")
	}
	endpoints, err := client.CoreV1().Endpoints("kube-system").List(context.TODO(), metav1.ListOptions{LabelSelector: "k8s-app=kube-dns"})
	if err != nil {
		t.Fatalf("Unable to get endpoint list")
	}

	ips, err := getIpsFromEndpoint(endpoints)
	if err != nil {
		t.Fatalf(err.Error())
	}
	if len(ips) < 1 {
		t.Fatalf("No ips found from endpoint list")
	}

}

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
