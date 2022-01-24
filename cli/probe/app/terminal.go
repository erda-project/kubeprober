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

package app

import (
	"context"
	"fmt"
	"net/url"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/util/term"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
	tunnelclient "github.com/erda-project/kubeprober/cli/probe/tunnel-client"
)

var TerminalCmd = &cobra.Command{
	Use:   "terminal",
	Short: "Terminal for cluster by Kubeprober",
	Long:  "Terminal for cluster by Kubeprober",
	RunE: func(cmd *cobra.Command, args []string) error {
		return terminal()
	},
}

func terminal() error {
	cluster := &kubeproberv1.Cluster{}
	if err := k8sRestClient.Get(context.Background(), client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      clusterName,
	}, cluster); err != nil {
		fmt.Printf("Get cluster info error: %+v\n", err)
		return err
	}

	conf, err := tunnelclient.GenerateProbeClientConf(cluster)
	if err != nil {
		return err
	}

	t := term.TTY{
		Out: os.Stdout,
		In:  os.Stdin,
		Raw: true,
	}
	sizeQueue := t.MonitorSize(t.GetSize())

	url, err := url.Parse(tunnelclient.MasterAddr)
	if err != nil {
		return err
	}
	if url.Scheme == "ws" {
		url.Scheme = "http"
	} else if url.Scheme == "wss" {
		url.Scheme = "https"
	} else {
		return errors.Errorf("invalid schema %s in %s", url.Scheme, tunnelclient.MasterAddr)
	}
	url.Path = fmt.Sprintf("/api/k8s/clusters/%s", clusterName)
	fn := func() error {
		exec, err := remotecommand.NewSPDYExecutor(conf, "POST", url)
		if err != nil {
			return err
		}
		err = exec.Stream(remotecommand.StreamOptions{
			Stdin:             os.Stdin,
			Stdout:            os.Stdout,
			Stderr:            os.Stderr,
			Tty:               true,
			TerminalSizeQueue: sizeQueue,
		})
		if err != nil {
			fmt.Printf("terminal exec stream error: %v\n", err)
			return err
		}
		return nil
	}

	if err := t.Safe(fn); err != nil {
		return err
	}

	return nil
}
