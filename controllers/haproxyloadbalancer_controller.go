/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/spectrocloud/cluster-api-provider-vsphere-static-ip/pkg/ipam"
	"github.com/spectrocloud/cluster-api-provider-vsphere-static-ip/pkg/ipam/factory"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/spectrocloud/cluster-api-provider-vsphere-static-ip/pkg/util"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	ctrl "sigs.k8s.io/controller-runtime"
)

// HAProxyLoadBalancerReconciler reconciles a HAProxyLoadBalancer object
type HAProxyLoadBalancerReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=haproxyloadbalancers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=haproxyloadbalancers/status,verbs=get;update;patch

func (r *HAProxyLoadBalancerReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("haproxyloadbalancer", req.NamespacedName)
	var res *ctrl.Result
	var err error

	cluster := &capi.Cluster{}
	if err := r.Get(ctx, req.NamespacedName, cluster); err != nil {
		return ctrl.Result{}, util.IgnoreNotFound(err)
	}

	haProxyLB := &infrav1.HAProxyLoadBalancer{}
	if err := r.Get(ctx, req.NamespacedName, haProxyLB); err != nil {
		return ctrl.Result{}, util.IgnoreNotFound(err)
	}

	res, err = r.reconcileLoadBalancerIPAddress(cluster, haProxyLB)
	if err != nil {
		log.Error(err, "failed to reconcile HAProxyLoadbalancer IP")
	}

	if res == nil {
		res = &ctrl.Result{}
	}

	return *res, err
}

func (r *HAProxyLoadBalancerReconciler) reconcileLoadBalancerIPAddress(cluster *capi.Cluster, lb *infrav1.HAProxyLoadBalancer) (*ctrl.Result, error) {
	if lb == nil {
		r.Log.V(0).Info("invalid HAProxyLoadBalancer, skipping reconcile IPAddress")
		return &ctrl.Result{}, nil
	}

	log := r.Log.WithValues("haProxyLoadBalancer", lb.Name, "namespace", lb.Namespace)
	devices := lb.Spec.VirtualMachineConfiguration.Network.Devices
	log.V(0).Info("reconcile IP address for HAProxyLoadBalancer")
	if len(devices) == 0 {
		log.V(0).Info("no network device found for HAProxyLoadBalancer")
		return &ctrl.Result{}, nil
	}

	if util.IsMachineIPAllocationDHCP(devices) {
		log.V(0).Info("HAProxyLoadBalancer has allocation type DHCP")
		return &ctrl.Result{}, nil
	}

	updatedDevices := []infrav1.NetworkDeviceSpec{}
	dataPatch := client.MergeFrom(lb.DeepCopy())

	newIpamFunc, ok := factory.IpamFactory[ipam.IpamTypeMetal3io]
	if !ok {
		log.V(0).Info("ipam type not supported")
		return &ctrl.Result{}, nil
	}

	ipamFunc := newIpamFunc(r.Client, log)

	for i, dev := range devices {
		if util.IsDeviceIPAllocationDHCP(dev) || len(dev.IPAddrs) > 0 {
			updatedDevices = append(updatedDevices, dev)
			continue
		}

		ipPool, err := ipamFunc.GetAvailableIPPool(lb.Labels, cluster.ObjectMeta)
		if err != nil {
			log.Error(err, "failed to get an available IPPool")
			return &ctrl.Result{}, nil
		}
		if ipPool == nil {
			log.V(0).Info("waiting for IPPool to be available")
			return &ctrl.Result{}, nil
		}

		ipName := util.GetFormattedClaimName(lb.Name, i)
		ip, err := ipamFunc.GetIP(ipName, ipPool)
		if err != nil {
			return &ctrl.Result{}, errors.Wrapf(err, "failed to get allocated IP address for HAProxyLoadBalancer %s", lb.Name)
		}

		if ip == nil {
			if _, err := ipamFunc.AllocateIP(ipName, ipPool, lb); err != nil {
				return &ctrl.Result{}, errors.Wrapf(err, "failed to allocate IP address for HAProxyLoadBalancer %s", lb.Name)
			}

			log.V(0).Info("waiting for IP address to be available for the HAProxyLoadBalancer")
			return &ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}

		if err := util.ValidateIP(ip); err != nil {
			return &ctrl.Result{}, errors.Wrapf(err, "invalid IP address retrieved for HAProxyLoadBalancer: %s", lb.Name)
		}

		log.V(0).Info(fmt.Sprintf("static IP for HAProxyLoadBalancer is %s", ip.GetName()))

		//capv expects static-ip in the CIDR format
		ipCidr := fmt.Sprintf("%s/%d", util.GetAddress(ip), util.GetMask(ip))
		log.V(0).Info(fmt.Sprintf("assigning IP address %s to HAProxyLoadBalancer", util.GetAddress(ip)))
		dev.IPAddrs = []string{ipCidr}
		gateway := util.GetGateway(ip)
		//TODO: handle ipv6
		//gateway4 is required if DHCP4 is disabled, gateway6 is required if DHCP6 is disabled
		dev.Gateway4 = gateway
		dev.Nameservers = util.GetDNSServers(ipPool)
		dev.SearchDomains = util.GetSearchDomains(ipPool)

		updatedDevices = append(updatedDevices, dev)
	}

	lb.Spec.VirtualMachineConfiguration.Network.Devices = updatedDevices
	if err := r.Patch(context.TODO(), lb.DeepCopyObject(), dataPatch); err != nil {
		return &ctrl.Result{}, errors.Wrapf(err, "failed to patch HAProxyLoadBalancer %s", lb.Name)
	}

	log.V(0).Info("successfully reconciled IP address for HAProxyLoadBalancer")

	return &ctrl.Result{}, nil
}

func (r *HAProxyLoadBalancerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.HAProxyLoadBalancer{}).
		Complete(r)
}
