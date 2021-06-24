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

package controller

import (
	"context"
	"encoding/base64"
	"k8s.io/apimachinery/pkg/util/json"
	"reflect"
	"strings"

	probev1alpha1 "github.com/erda-project/kubeprober/pkg/probe-agent/apis/v1alpha1"
	clusterv1 "github.com/erda-project/kubeprober/pkg/probe-master/apis/v1"
	dialclient "github.com/erda-project/kubeprober/pkg/probe-master/tunnel-client"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ClusterReconciler reconciles a Cluster object
type ClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

//+kubebuilder:rbac:groups=kubeprober.erda.cloud,resources=clusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kubeprober.erda.cloud,resources=clusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kubeprober.erda.cloud,resources=clusters/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Cluster object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *ClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var err error
	var labelKeys []string

	klog.Errorf("____________________cluster_____________________________________, %+v\n", req.NamespacedName)
	cluster := &clusterv1.Cluster{}
	if err = r.Get(ctx, req.NamespacedName, cluster); err != nil {
		klog.Errorf("get cluster spec [%s] error:  %+v\n", req.Name, err)
		return ctrl.Result{}, err
	}

	//get probe labels of cluster
	labels := cluster.GetLabels()
	for k, v := range labels {
		if v == "true" && strings.Split(k, "/")[0] == "probe" {
			labelKeys = append(labelKeys, strings.Split(k, "/")[1])
		}
	}
	klog.Infof("labels of cluster [%s] is: %+v\n", req.Name, labelKeys)

	//add probe
	for i, _ := range labelKeys {
		if !IsContain(cluster.Status.AttachedProbes, labelKeys[i]) {
			probe := &probev1alpha1.Probe{}
			if err = r.Get(ctx, types.NamespacedName{
				Namespace: "default",
				Name:      labelKeys[i],
			}, probe); err != nil {
				klog.Infof("fail to get probe [%s], error: %+v\n", labelKeys[i], err)
				return ctrl.Result{}, err
			}
			klog.Errorf("create probe [%s] for cluster [%s]\n", probe.Name, cluster.Name)
			//TODO: 处理already exist的情况
			if err = AddProbeToCluster(cluster, probe); err != nil {
				klog.Errorf("create probe [%s] for cluster [%s] err: %+v\n", probe.Name, cluster.Name, err)
				return ctrl.Result{}, err
			}
		}
	}
	//delete probe
	for i, _ := range cluster.Status.AttachedProbes {
		if !IsContain(labelKeys, cluster.Status.AttachedProbes[i]) {
			//TODO: 处理not found的情况
			klog.Infof("delete probe [%s] for cluster [%s]\n", cluster.Status.AttachedProbes[i], cluster.Name)
			if err = DeleteProbeOfCluster(cluster, cluster.Status.AttachedProbes[i]); err != nil {
				klog.Errorf("delete probe [%s] for cluster [%s] err: %+v\n", cluster.Status.AttachedProbes[i], cluster.Name, err)
				return ctrl.Result{}, err
			}
		}
	}

	//update status of cluster
	cluster.Status.AttachedProbes = labelKeys
	if err = r.Status().Update(ctx, cluster); err != nil {
		klog.Errorf("update cluster [%s] status error: %+v\n", req.Name, err)
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&clusterv1.Cluster{}).WithEventFilter(&ClusterPredicate{}).
		Complete(r)
}

func IsContain(items []string, item string) bool {
	for _, eachItem := range items {
		if eachItem == item {
			return true
		}
	}
	return false
}

// add probe to cluster
func AddProbeToCluster(cluster *clusterv1.Cluster, probe *probev1alpha1.Probe) error {
	var err error
	var c client.Client

	c, err = GenerateProbeClient(cluster)
	if err != nil {
		return err
	}

	pp := &probev1alpha1.Probe{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Probe",
			APIVersion: "kubeprober.erda.cloud/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      probe.Name,
			Namespace: cluster.Spec.ClusterConfig.ProbeNamespaces,
		},
		Spec: probe.Spec,
	}

	err = c.Create(context.Background(), pp)
	if err != nil {
		return err
	}

	return nil
}

//delete probe of cluster
func DeleteProbeOfCluster(cluster *clusterv1.Cluster, probeName string) error {
	var err error
	var c client.Client

	c, err = GenerateProbeClient(cluster)
	if err != nil {
		return err
	}

	pp := &unstructured.Unstructured{}
	pp.SetName(probeName)
	pp.SetNamespace(cluster.Spec.ClusterConfig.ProbeNamespaces)
	pp.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "kubeprober.erda.cloud",
		Kind:    "Probe",
		Version: "v1alpha1",
	})

	err = c.Delete(context.Background(), pp)
	if err != nil {
		return err
	}
	return nil
}

// get probe of cluster
func GetProbeOfCluster(cluster *clusterv1.Cluster, probeName string) (*probev1alpha1.Probe, error) {
	var err error
	var c client.Client

	c, err = GenerateProbeClient(cluster)
	if err != nil {
		return nil, err
	}

	probe := &probev1alpha1.Probe{}

	err = c.Get(context.Background(), client.ObjectKey{
		Namespace: cluster.Spec.ClusterConfig.ProbeNamespaces,
		Name:      probeName,
	}, probe)
	if err != nil {
		return nil, err
	}

	return probe, nil
}

// update probe of cluster
func UpdateProbeOfCluster(cluster *clusterv1.Cluster, probe *probev1alpha1.Probe) error {
	var err error
	var c client.Client
	var patch []byte

	c, err = GenerateProbeClient(cluster)
	if err != nil {
		return err
	}
	patchBody := probev1alpha1.Probe{
		Spec: probe.Spec,
	}
	if patch, err = json.Marshal(patchBody); err != nil {
		return err
	}
	if err = c.Patch(context.Background(), &probev1alpha1.Probe{
		ObjectMeta: metav1.ObjectMeta{
			Name:      probe.Name,
			Namespace: cluster.Spec.ClusterConfig.ProbeNamespaces,
		},
	}, client.RawPatch(types.MergePatchType, patch)); err != nil {
		return err
	}

	return nil
}

//Generate k8sclient of cluster
func GenerateProbeClient(cluster *clusterv1.Cluster) (client.Client, error) {
	var clusterToken []byte
	var err error
	var c client.Client

	if clusterToken, err = base64.StdEncoding.DecodeString(cluster.Spec.ClusterConfig.Token); err != nil {
		return nil, err
	}
	config, err := dialclient.GetDialerRestConfig(cluster.Name, &dialclient.ManageConfig{
		Type:    dialclient.ManageProxy,
		Address: cluster.Spec.ClusterConfig.Address,
		Token:   strings.Trim(string(clusterToken), "\n"),
	})
	if err != nil {
		return nil, err
	}
	klog.Errorf("ffffffffff  config: %+v\n", config)
	scheme := runtime.NewScheme()
	probev1alpha1.AddToScheme(scheme)
	c, err = client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		return nil, err
	}
	return c, nil
}

type ClusterPredicate struct {
	predicate.Funcs
}

func (rl *ClusterPredicate) Update(e event.UpdateEvent) bool {
	//only label or extrainfo changed event hadnled
	ns := e.ObjectNew.GetNamespace()
	if ns != metav1.NamespaceDefault {
		return false
	}
	if !reflect.DeepEqual(e.ObjectNew.GetLabels(), e.ObjectOld.GetLabels()) {
		return true
	}
	oldobj, ok1 := e.ObjectOld.(*clusterv1.Cluster)
	newobj, ok2 := e.ObjectNew.(*clusterv1.Cluster)
	if ok1 && ok2 {
		if !reflect.DeepEqual(oldobj.Spec.ExtraInfo, newobj.Spec.ExtraInfo) {
			return true
		}
	}
	return false
}

func (rl *ClusterPredicate) Create(e event.CreateEvent) bool {
	ns := e.Object.GetNamespace()
	if ns != metav1.NamespaceDefault {
		return false
	}
	return true
}

func (rl *ClusterPredicate) Delete(e event.DeleteEvent) bool {
	ns := e.Object.GetNamespace()
	if ns != metav1.NamespaceDefault {
		return false
	}
	return true
}

func (rl *ClusterPredicate) Generic(e event.GenericEvent) bool {
	ns := e.Object.GetNamespace()
	if ns != metav1.NamespaceDefault {
		return false
	}
	return true
}
