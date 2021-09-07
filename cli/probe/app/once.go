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
	"errors"
	"fmt"
	"strings"
	"time"

	kubeproberv1 "github.com/erda-project/kubeprober/apis/v1"
	"github.com/gosuri/uitable"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const KBNAMESPACE = "kubeprober"

var OnceCmd = &cobra.Command{
	Use:   "once",
	Short: "Perform one-time diagnostics of remote cluster or local cluster",
	Long:  "Perform one-time diagnostics of remote cluster or local cluster",
	RunE: func(cmd *cobra.Command, args []string) error {
		if clusterName == "" {
			return DoOnceProbeLocal(probes)
		} else {
			return DoOnceProbe(clusterName, probes)
		}
	},
}

func DoOnceProbeLocal(probes string) error {
	var err error
	var onceProbeList []kubeproberv1.Probe
	var onceProbeNameList []string
	probeList := &kubeproberv1.ProbeList{}
	inputNameList := strings.Split(probes, ",")
	onceId := fmt.Sprintf("%d", int32(time.Now().Unix()))
	if err = k8sRestClient.List(context.Background(), probeList); err != nil {
		fmt.Printf("Get probe list error: %+v\n", err)
		return err
	}
	for _, i := range probeList.Items {
		if i.Namespace != KBNAMESPACE {
			continue
		}
		if probes == "" {
			onceProbeList = append(onceProbeList, i)
		} else {
			if IsContain(inputNameList, i.Name) {
				onceProbeList = append(onceProbeList, i)
			}
		}
	}

	for _, i := range onceProbeList {
		onceProbeNameList = append(onceProbeNameList, i.Name)
		name := fmt.Sprintf("%s-oncelocal-%s", i.Name, onceId)

		i.Spec.Policy.RunInterval = 0
		pp := &kubeproberv1.Probe{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Probe",
				APIVersion: "kubeprober.erda.cloud/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: i.Namespace,
			},
			Spec: i.Spec,
		}

		err = k8sRestClient.Create(context.Background(), pp)
		if err != nil {
			return err
		}
	}
	//wait for once probe finish
	now := time.Now()
	for i := 0; i < 30; i++ {
		time.Sleep(10 * time.Second)
		status, err = getOncePorbeStatus(k8sRestClient, KBNAMESPACE, onceId)
		if err != nil {
			return err
		}
		sub := time.Now().Sub(now)
		fmt.Printf("\rTime: %ds,   Status: %s,   One-Time Probe: %s", int(sub/time.Second), status, onceProbeNameList)
		if status == "Succeeded" {
			break
		}
	}
	fmt.Println()
	if status == "Succeeded" {
		if err = PrintOnceProbeStatus(k8sRestClient, KBNAMESPACE, onceId); err != nil {
			return err
		}
	} else {
		return errors.New("get once probe status timeout!")
	}
	return nil
}

func DoOnceProbe(clusterName string, probes string) error {
	var err error
	var c client.Client
	var onceProbeNameList []string
	var onceProbeList []kubeproberv1.Probe
	var status string

	onceId := fmt.Sprintf("%d", int32(time.Now().Unix()))
	cluster := &kubeproberv1.Cluster{}
	if err = k8sRestClient.Get(context.Background(), client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      clusterName,
	}, cluster); err != nil {
		fmt.Printf("Get cluster info error: %+v\n", err)
		return err
	}

	if c, err = GenerateProbeClient(cluster); err != nil {
		return err
	}
	namespace := cluster.Spec.ClusterConfig.ProbeNamespaces
	if probes == "" {
		onceProbeNameList = cluster.Status.AttachedProbes
	} else {
		onceProbeNameList = strings.Split(probes, ",")
	}
	for _, i := range onceProbeNameList {
		probe := &kubeproberv1.Probe{}
		if err = k8sRestClient.Get(context.Background(), client.ObjectKey{
			Namespace: metav1.NamespaceDefault,
			Name:      i,
		}, probe); err != nil {
			return err
		}
		onceProbeList = append(onceProbeList, *probe)
	}

	for _, i := range onceProbeList {
		name := fmt.Sprintf("%s-once-%s", i.Name, onceId)

		i.Spec.Policy.RunInterval = 0
		pp := &kubeproberv1.Probe{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Probe",
				APIVersion: "kubeprober.erda.cloud/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: i.Spec,
		}

		err = c.Create(context.Background(), pp)
		if err != nil {
			return err
		}
	}

	//update once probe status of cluster
	if err = updateClusterOnceProbeStatus(cluster, onceId, onceProbeNameList); err != nil {
		return err
	}

	//wait for once probe finish
	now := time.Now()
	for i := 0; i < 30; i++ {
		time.Sleep(10 * time.Second)
		status, err = getOncePorbeStatus(c, namespace, onceId)
		if err != nil {
			return err
		}
		sub := time.Now().Sub(now)
		fmt.Printf("\rTime: %ds,   Status: %s,   One-Time Probe: %s", int(sub/time.Second), status, onceProbeNameList)
		if status == "Succeeded" {
			break
		}
	}
	if err = updateOnceProbeStatusFinishTime(cluster, onceId); err != nil {
		return err
	}
	fmt.Println()
	if status == "Succeeded" {
		if err = PrintOnceProbeStatus(c, namespace, onceId); err != nil {
			return err
		}
	} else {
		return errors.New("get once probe status timeout!")
	}

	return nil
}

func getOncePorbeStatus(c client.Client, ns string, onceID string) (string, error) {
	var err error
	var status string

	status = "Succeeded"
	podList := &corev1.PodList{}
	if err = c.List(context.Background(), podList, client.InNamespace(ns)); err != nil {
		return "", err
	}
	//just print once probe status
	for _, i := range podList.Items {
		if strings.Contains(i.Name, onceID) {
			if i.Status.Phase != "Succeeded" {
				status = string(i.Status.Phase)
			}
		}
	}
	return status, nil
}

func updateClusterOnceProbeStatus(cluster *kubeproberv1.Cluster, onceID string, onceProbeNameList []string) error {
	var err error
	//update once probe status of cluster
	cluster.Status.OnceProbeList = append(cluster.Status.OnceProbeList, kubeproberv1.OnceProbeItem{
		ID:         onceID,
		CreateTime: time.Now().Format("2006-01-02 15:04:05"),
		FinishTime: time.Now().Format("2006-01-02 15:04:05"),
		Probes:     onceProbeNameList,
	})
	if len(cluster.Status.OnceProbeList) > 5 {
		cluster.Status.OnceProbeList = cluster.Status.OnceProbeList[len(cluster.Status.OnceProbeList)-5:]
	}
	var patch []byte
	statusPatch := kubeproberv1.Cluster{
		Status: kubeproberv1.ClusterStatus{
			OnceProbeList: cluster.Status.OnceProbeList,
		},
	}
	if patch, err = json.Marshal(statusPatch); err != nil {
		return err
	}

	if err = k8sRestClient.Status().Patch(context.Background(), &kubeproberv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name,
			Namespace: metav1.NamespaceDefault,
		},
	}, client.RawPatch(types.MergePatchType, patch)); err != nil {
		errMsg := fmt.Sprintf("update cluster [%s] status error: %+v\n", cluster.Name, err)
		return errors.New(errMsg)
	}
	return nil
}

func updateOnceProbeStatusFinishTime(cluster *kubeproberv1.Cluster, onceID string) error {
	var err error
	//update once probe status of cluster
	for i := range cluster.Status.OnceProbeList {
		if cluster.Status.OnceProbeList[i].ID == onceID {
			cluster.Status.OnceProbeList[i].FinishTime = time.Now().Format("2006-01-02 15:04:05")
		}
	}
	var patch []byte
	statusPatch := kubeproberv1.Cluster{
		Status: kubeproberv1.ClusterStatus{
			OnceProbeList: cluster.Status.OnceProbeList,
		},
	}
	if patch, err = json.Marshal(statusPatch); err != nil {
		return err
	}

	if err = k8sRestClient.Status().Patch(context.Background(), &kubeproberv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name,
			Namespace: metav1.NamespaceDefault,
		},
	}, client.RawPatch(types.MergePatchType, patch)); err != nil {
		errMsg := fmt.Sprintf("update cluster [%s] status error: %+v\n", cluster.Name, err)
		return errors.New(errMsg)
	}
	return nil
}

func PrintOnceProbeStatus(c client.Client, ns string, onceID string) error {
	var err error

	probeStatusList := &kubeproberv1.ProbeStatusList{}
	if err = c.List(context.Background(), probeStatusList, client.InNamespace(ns)); err != nil {
		return err
	}
	//just print once probe status
	table := uitable.New()
	table.MaxColWidth = 45
	table.Wrap = true
	table.AddRow("PROBER", "CHECKER", "STATUS", "MESSAGE", "LASTRUN")
	for _, i := range probeStatusList.Items {
		if strings.Contains(i.Name, onceID) {
			for _, j := range i.Spec.Checkers {
				table.AddRow(i.Name, j.Name, j.Status, strings.TrimSpace(j.Message), j.LastRun.Format("2006-01-02 15:04:05"))
			}
		}
	}
	fmt.Println(table)
	return nil
}
