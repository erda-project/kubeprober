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
	"os"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"
	// required for oidc kubectl testing
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	probev1 "github.com/erda-project/kubeprober/apis/v1"
	"github.com/erda-project/kubeprober/pkg/kubeclient"
	status "github.com/erda-project/kubeprober/pkg/probe-status"
)

const maxTimeInFailure = 60 * time.Second
const defaultCheckTimeout = 5 * time.Minute

// KubeConfigFile is a variable containing file path of Kubernetes config files
var KubeConfigFile = filepath.Join(os.Getenv("HOME"), ".kube", "config")

// CheckTimeout is a variable for how long code should run before it should retry
var CheckTimeout time.Duration

// Hostname is a variable for container/pod name
var Hostname string

// NodeName is a variable for the node where the container/pod is created
var NodeName string

// Namespace where dns pods live
var namespace string

// Label selector used for dns pods
var labelSelector string

var TestPublicDomain string
var now time.Time

// Checker validates that DNS is functioning correctly
type Checker struct {
	client           *kubernetes.Clientset
	MaxTimeInFailure time.Duration
	Hostname         string
}

func init() {

	// Set check time limit to default
	CheckTimeout = defaultCheckTimeout

	Hostname = os.Getenv("HOSTNAME")
	if len(Hostname) == 0 {
		log.Errorln("ERROR: The ENDPOINT environment variable has not been set.")
		return
	}

	NodeName = os.Getenv("NODE_NAME")
	if len(NodeName) == 0 {
		log.Errorln("ERROR: Failed to retrieve NODE_NAME environment variable.")
		return
	}
	log.Infoln("Check pod is running on node:", NodeName)

	namespace = os.Getenv("NAMESPACE")
	if len(namespace) > 0 {
		log.Infoln("Looking for DNS pods in namespace:", namespace)
	}

	labelSelector = os.Getenv("DNS_POD_SELECTOR")
	if len(labelSelector) > 0 {
		log.Infoln("Looking for DNS pods with label:", labelSelector)
	}
	TestPublicDomain = os.Getenv("TEST_PUBLIC_DOMAIN")
	now = time.Now()
}

func main() {
	client, err := kubeclient.Client(KubeConfigFile)
	if err != nil {
		log.Fatalln("Unable to create kubernetes client", err)
	}

	dc := New()
	err = dc.Run(client)
	if err != nil {
		log.Errorln("Error running DNS Status check for hostname:", Hostname)
	}
	log.Infoln("Done running DNS Status check for hostname:", Hostname)
}

// New returns a new DNS Checker
func New() *Checker {
	return &Checker{
		Hostname:         Hostname,
		MaxTimeInFailure: maxTimeInFailure,
	}
}

// Run implements the entrypoint for check execution
func (dc *Checker) Run(client *kubernetes.Clientset) error {
	log.Infoln("Running DNS status checker")
	doneChan := make(chan error)

	dc.client = client
	// run the check in a goroutine and notify the doneChan when completed
	go func(doneChan chan error) {
		err := dc.doChecks()
		doneChan <- err
	}(doneChan)

	now := metav1.Now()
	dnsChecker := probev1.ProbeCheckerStatus{
		Name:    "check-dns-resolution",
		Status:  probev1.CheckerStatusPass,
		Message: "",
		LastRun: &now,
	}

	// wait for either a timeout or job completion
	select {
	case <-time.After(CheckTimeout):
		// The check has timed out after its specified timeout period
		errorMessage := "Failed to complete DNS Status check in time! Timeout was reached."
		dnsChecker.Status = probev1.CheckerStatusError
		dnsChecker.Message = errorMessage
		err := status.ReportProbeStatus([]probev1.ProbeCheckerStatus{dnsChecker})
		if err != nil {
			log.Println("Error reporting failure to probeagent servers:", err)
			return err
		}
		return err
	case err := <-doneChan:
		if err != nil {
			dnsChecker.Status = probev1.CheckerStatusError
			dnsChecker.Message = err.Error()
			return status.ReportProbeStatus([]probev1.ProbeCheckerStatus{dnsChecker})
		}
		return status.ReportProbeStatus([]probev1.ProbeCheckerStatus{dnsChecker})
	}
}

// create a Resolver object to use for DNS queries
func createResolver(ip string) (*net.Resolver, error) {
	r := &net.Resolver{}
	// if we're supplied a null string, return an error
	if len(ip) < 1 {
		return r, errors.New("Need a valid ip to create Resolver")
	}
	// attempt to create the resolver based on the string
	r = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address2 string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Millisecond * time.Duration(10000),
			}
			return d.DialContext(ctx, "udp", ip+":53")
		},
	}
	return r, nil
}

func getIpsFromEndpoint(endpoints *v1.EndpointsList) ([]string, error) {
	var ipList []string
	if len(endpoints.Items) == 0 {
		return ipList, errors.New("No endpoints found")
	}
	// loop through endpoint list, subsets, and finally addresses to get backend DNS ip's to query
	for ep := 0; ep < len(endpoints.Items); ep++ {
		for sub := 0; sub < len(endpoints.Items[ep].Subsets); sub++ {
			for address := 0; address < len(endpoints.Items[ep].Subsets[sub].Addresses); address++ {
				// create resolver based on backends found in the dns endpoint
				ipList = append(ipList, endpoints.Items[ep].Subsets[sub].Addresses[address].IP)
			}
		}
	}
	if len(ipList) != 0 {
		return ipList, nil
	}
	return ipList, errors.New("No Ip's found in endpoints list")
}

func dnsLookup(r *net.Resolver, host string) error {
	_, err := r.LookupHost(context.Background(), host)
	if err != nil {
		errorMessage := "DNS Status check determined that " + host + " is DOWN: " + err.Error()
		return errors.New(errorMessage)
	}
	return nil
}

func (dc *Checker) checkEndpoints() error {
	endpoints, err := dc.client.CoreV1().Endpoints(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		message := "DNS status check unable to get dns endpoints from cluster: " + err.Error()
		log.Errorln(message)
		return errors.New(message)
	}

	//get ips from endpoint list to check
	ips, err := getIpsFromEndpoint(endpoints)
	if err != nil {
		return err
	}

	//given that we got valid ips from endpoint list, parse them
	log.Infof("valid ips from endpoint list is %+v\n", ips)
	if len(ips) > 0 {
		for ip := 0; ip < len(ips); ip++ {
			//create a resolver for each ip and return any error
			r, err := createResolver(ips[ip])
			if err != nil {
				return err
			}
			//run a lookup for each ip if we successfully created a resolver, return error
			err = dnsLookup(r, dc.Hostname)
			if err != nil {
				return err
			}
			//
			err = dnsLookup(r, TestPublicDomain)
			if err != nil {
				return err
			}
		}
		log.Infoln("DNS Status check from service endpoint determined that", dc.Hostname, "was OK.")
		return nil
	}
	return errors.New("No ips found in endpoint with label: " + labelSelector)
}

// doChecks does validations on the DNS call to the endpoint
func (dc *Checker) doChecks() error {

	var err error

	log.Infoln("DNS Status check testing hostname:", dc.Hostname)

	// if there's a label selector, do checks against endpoints
	if len(labelSelector) > 0 {
		err := dc.checkEndpoints()
		if err != nil {
			return err
		}
		return nil
	}

	// otherwise do lookup against service endpoint
	_, err = net.LookupHost(dc.Hostname)
	if err != nil {
		errorMessage := "DNS Status check determined that " + dc.Hostname + " is DOWN: " + err.Error()
		log.Errorln(errorMessage)
		return errors.New(errorMessage)
	}

	_, err = net.LookupHost(TestPublicDomain)
	if err != nil {
		errorMessage := "DNS Status check determined that " + TestPublicDomain + " is DOWN: " + err.Error()
		log.Errorln(errorMessage)
		return errors.New(errorMessage)
	}

	log.Infoln("DNS Status check from service endpoint determined that", dc.Hostname, "was OK.")
	return nil
}
