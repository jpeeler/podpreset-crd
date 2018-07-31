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

package podpreset

import (
	"context"
	"fmt"

	"github.com/golang/glog"
	settingsv1alpha1 "github.com/jpeeler/podpreset-crd/pkg/apis/settings/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	annotationPrefix = "podpreset.admission.kubernetes.io"
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new PodPreset Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
// USER ACTION REQUIRED: update cmd/manager/main.go to call this settings.Add(mgr) to install this Controller
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcilePodPreset{Client: mgr.GetClient(), scheme: mgr.GetScheme(), recorder: mgr.GetRecorder("podpreset-controller")}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("podpreset-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to PodPreset
	err = c.Watch(&source.Kind{Type: &settingsv1alpha1.PodPreset{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcilePodPreset{}

// ReconcilePodPreset reconciles a PodPreset object
type ReconcilePodPreset struct {
	client.Client
	scheme   *runtime.Scheme
	recorder record.EventRecorder
}

// Reconcile reads that state of the cluster for a PodPreset object and makes changes based on the state read
// and what is in the PodPreset.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  The scaffolding writes
// a Deployment as an example
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=settings.svcat.k8s.io,resources=podpresets,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcilePodPreset) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the PodPreset instance
	pp := &settingsv1alpha1.PodPreset{}
	err := r.Client.Get(context.TODO(), request.NamespacedName, pp)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	selector, err := metav1.LabelSelectorAsSelector(&pp.Spec.Selector)
	if err != nil {
		return reconcile.Result{}, err
	}
	deploymentList := &appsv1.DeploymentList{}
	r.Client.List(context.TODO(), &client.ListOptions{LabelSelector: selector}, deploymentList)

	for i, deployment := range deploymentList.Items {
		glog.V(6).Infof("(%v) Looking at deployment %v\n", i, deployment.Name)
		if selector.Matches(labels.Set(deployment.Spec.Template.ObjectMeta.Labels)) {
			bouncedKey := fmt.Sprintf("%s/bounced-%s", annotationPrefix, pp.GetName())
			resourceVersion, found := deployment.Spec.Template.ObjectMeta.Annotations[bouncedKey]
			if !found || found && resourceVersion < pp.GetResourceVersion() {
				// bounce pod since this is the first mutation or a later mutation has occurred
				glog.V(4).Infof("Detected deployment '%v' needs bouncing", deployment.Name)
				// TODO: may not need both of these events
				r.recorder.Eventf(pp, v1.EventTypeNormal, "DeploymentBounced", "Bounced %v-%v due to newly created or updated podpreset", deployment.Name, deployment.GetResourceVersion())
				r.recorder.Eventf(&deployment, v1.EventTypeNormal, "DeploymentBounced", "Bounced to newly created or updated podpreset %v-%v", pp.Name, pp.GetResourceVersion())
				metav1.SetMetaDataAnnotation(&deployment.Spec.Template.ObjectMeta, bouncedKey, pp.GetResourceVersion())
				err = r.Client.Update(context.TODO(), &deployment)
				if err != nil {
					return reconcile.Result{}, err
				}
			}
		}
	}

	return reconcile.Result{}, nil
}
