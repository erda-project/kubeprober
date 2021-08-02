package main

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/erda-project/kubeprober/pkg/kubeclient"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
)

// Checker validates that DNS is functioning correctly
type DnsChecker struct {
	client  *kubernetes.Clientset
	Name    string
	Status  kubeproberv1.CheckerStatus
	Timeout time.Duration
}

// New returns a new DNS Checker
func NewDnsChecker() (*DnsChecker, error) {
	// get kubernetes client
	client, err := kubeclient.Client(cfg.KubeConfigFile)
	if err != nil {
		logrus.Fatalln("Unable to create kubernetes client", err)
		return nil, err
	}
	return &DnsChecker{
		client:  client,
		Name:    "dns-resolution-check",
		Timeout: cfg.CheckTimeout,
	}, nil
}

func (dc *DnsChecker) GetName() string {
	return dc.Name
}

func (dc *DnsChecker) SetName(n string) {
	dc.Name = n
}

func (dc *DnsChecker) GetStatus() kubeproberv1.CheckerStatus {
	return dc.Status
}

func (dc *DnsChecker) SetStatus(s kubeproberv1.CheckerStatus) {
	dc.Status = s
}

func (dc *DnsChecker) GetTimeout() time.Duration {
	return dc.Timeout
}

func (dc *DnsChecker) SetTimeout(t time.Duration) {
	dc.Timeout = t
}

// doChecks does validations on the DNS call to the endpoint
func (dc *DnsChecker) DoCheck() error {

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

func (dc *DnsChecker) checkEndpoints() error {
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
		logrus.Infof("DNS Status check from service endpoint determined, private domain: %s, public domain: %s was OK.", cfg.PrivateDomain, cfg.PublicDomain)
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
