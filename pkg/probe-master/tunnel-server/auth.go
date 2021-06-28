// Copyright (c) 2021 Terminus, Inc.
//
// This program is free software: you can use, redistribute, and/or modify
// it under the terms of the GNU Affero General Public License, version 3
// or later ("AGPL"), as published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful, but WITHOUT
// ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or
// FITNESS FOR A PARTICULAR PURPOSE.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package server

import (
	"context"
	"net/http"

	kubeprobev1 "github.com/erda-project/kubeprober/apis/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Authorizer(req *http.Request) (string, bool, error) {
	var err error
	var uid string
	// inner proxy not need auth
	if req.URL.Path == "/clusterdialer" {
		return "proxy", true, nil
	}
	clusterName := req.Header.Get("X-Cluster-Name")
	secretKey := req.Header.Get("Secret-Key")
	if uid, err = getUidByClusterName(clusterName); err != nil {
		klog.Errorf("[remote dialer authorizer] get uid by cluster name [%s] error: %+v\n", clusterName, err)
		return clusterName, false, err
	}
	return clusterName, secretKey == uid, nil
}

func getUidByClusterName(name string) (string, error) {
	cluster := &kubeprobev1.Cluster{}
	err := clusterRestClient.Get(context.Background(), client.ObjectKey{
		Name:      name,
		Namespace: metav1.NamespaceDefault,
	}, cluster)
	if err != nil {
		return "", err
	}
	return string(cluster.ObjectMeta.UID), nil
}
