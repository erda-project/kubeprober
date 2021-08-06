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

package server

import (
	"net/http"
	"os"
)

func Authorizer(req *http.Request) (string, bool, error) {
	var ServersecretKey string
	// inner proxy not need auth
	if req.URL.Path == "/clusterdialer" {
		return "proxy", true, nil
	}
	ServersecretKey = os.Getenv("SERVER_SECRET_KEY")
	clusterName := req.Header.Get("X-Cluster-Name")
	secretKey := req.Header.Get("Secret-Key")
	return clusterName, secretKey == ServersecretKey, nil
}

//func getUidByClusterName(name string) (string, error) {
//	cluster := &kubeproberv1.Cluster{}
//	err := clusterRestClient.Get(context.Background(), client.ObjectKey{
//		Name:      name,
//		Namespace: metav1.NamespaceDefault,
//	}, cluster)
//	if err != nil {
//		return "", err
//	}
//	return string(cluster.ObjectMeta.UID), nil
//}
