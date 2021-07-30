package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	kubeprobev1 "github.com/erda-project/kubeprober/apis/v1"
	"github.com/erda-project/kubeprober/pkg/kubeclient"
	probestatus "github.com/erda-project/kubeprober/pkg/probe-status"
)

// Checker validates that DNS is functioning correctly
type Checker struct {
	client *kubernetes.Clientset
}

// New returns a new DNS Checker
func NewChecker() (*Checker, error) {
	// get kubernetes client
	client, err := kubeclient.Client(cfg.KubeConfigFile)
	if err != nil {
		logrus.Fatalln("Unable to create kubernetes client", err)
		return nil, err
	}
	return &Checker{
		client: client,
	}, nil
}

// Run implements the entrypoint for check execution
func (dc *Checker) Run() error {
	logrus.Infoln("Running DNS status checker")
	doneChan := make(chan error)

	// run the check in a goroutine and notify the doneChan when completed
	go func(doneChan chan error) {
		err := dc.doChecks()
		doneChan <- err
	}(doneChan)

	now := metav1.Now()
	// init dns check status
	dnsChecker := kubeprobev1.ProbeCheckerStatus{
		Name:    "check-dns-resolution",
		Status:  kubeprobev1.CheckerStatusPass,
		Message: "",
		LastRun: &now,
	}

	// wait for either a timeout or job completion
	select {
	case <-time.After(cfg.CheckTimeout):
		// The check has timed out after its specified timeout period
		dnsChecker.Status = kubeprobev1.CheckerStatusError
		dnsChecker.Message = fmt.Sprintf("Failed to complete DNS Status check in time! Timeout(%s) was reached.", cfg.CheckTimeout)
		err := probestatus.ReportProbeStatus([]kubeprobev1.ProbeCheckerStatus{dnsChecker})
		if err != nil {
			logrus.Println("Error reporting failure to probeagent:", err)
			return err
		}
		return err
	case err := <-doneChan:
		if err != nil {
			dnsChecker.Status = kubeprobev1.CheckerStatusError
			dnsChecker.Message = err.Error()
			return probestatus.ReportProbeStatus([]kubeprobev1.ProbeCheckerStatus{dnsChecker})
		}
		return probestatus.ReportProbeStatus([]kubeprobev1.ProbeCheckerStatus{dnsChecker})
	}
}

// doChecks does validations on the DNS call to the endpoint
func (dc *Checker) doChecks() error {

	var err error

	logrus.Infof("dns check private domain: %s, public domain: %s", cfg.PrivateDomain, cfg.PublicDomain)

	// if there's a label selector, do checks against endpoints
	if len(cfg.LabelSelector) > 0 {
		err := dc.checkEndpoints()
		if err != nil {
			return err
		}
		return nil
	}

	// otherwise do lookup against service endpoint
	_, err = net.LookupHost(cfg.PrivateDomain)
	if err != nil {
		errorMessage := "DNS Status check determined that private domain" + cfg.PrivateDomain + " is DOWN: " + err.Error()
		logrus.Errorln(errorMessage)
		return errors.New(errorMessage)
	}

	_, err = net.LookupHost(cfg.PublicDomain)
	if err != nil {
		errorMessage := "DNS Status check determined that public domain " + cfg.PublicDomain + " is DOWN: " + err.Error()
		logrus.Errorln(errorMessage)
		return errors.New(errorMessage)
	}

	logrus.Infof("dns check ok, private domain: %s, public domain: %s", cfg.PrivateDomain, cfg.PublicDomain)
	return nil
}

func (dc *Checker) checkEndpoints() error {
	// get dns endpoint
	endpoints, err := dc.client.CoreV1().Endpoints(cfg.Namespace).List(context.Background(), metav1.ListOptions{LabelSelector: cfg.LabelSelector})
	if err != nil {
		message := "DNS status check unable to get dns endpoints from cluster: " + err.Error()
		logrus.Errorln(message)
		return errors.New(message)
	}

	// get ips from endpoint list to check
	ips, err := getIpsFromEndpoint(endpoints)
	if err != nil {
		return err
	}

	// given that we got valid ips from endpoint list, parse them
	logrus.Infof("valid ips from endpoint list is %+v\n", ips)
	if len(ips) > 0 {
		for ip := 0; ip < len(ips); ip++ {
			//create a resolver for each ip and return any error
			r, err := createResolver(ips[ip])
			if err != nil {
				return err
			}
			//run a lookup for each ip if we successfully created a resolver, return error
			err = dnsLookup(r, cfg.PrivateDomain)
			if err != nil {
				return err
			}
			//
			err = dnsLookup(r, cfg.PublicDomain)
			if err != nil {
				return err
			}
		}
		logrus.Infoln("DNS Status check from service endpoint determined that", cfg.PrivateDomain, cfg.PublicDomain, "was OK.")
		return nil
	}
	return errors.New("No ips found in endpoint with label: " + cfg.LabelSelector)
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
