package admission

import (
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	extsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/mesosphere/dklb/pkg/constants"
	"github.com/mesosphere/dklb/pkg/translator"
)

// setBaseDefaults sets the base, common defaults on the specified resource.
func setBaseDefaults(object metav1.Object, options *translator.BaseTranslationOptions) {
	annotations := object.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	// If no cloud load-balancer configuration is specified, we explicitly set the values of the "kubernetes.dcos.io/edgelb-pool-*" annotations.
	// Otherwise, we explicitly remove them as their values must be controlled by ourselves.
	if options.CloudLoadBalancerConfigMapName == nil {
		annotations[constants.CloudLoadBalancerConfigMapNameAnnotationKey] = ""
		annotations[constants.EdgeLBPoolNameAnnotationKey] = options.EdgeLBPoolName
		annotations[constants.EdgeLBPoolRoleAnnotationKey] = options.EdgeLBPoolRole
		annotations[constants.EdgeLBPoolNetworkAnnotationKey] = options.EdgeLBPoolNetwork
		annotations[constants.EdgeLBPoolCpusAnnotationKey] = options.EdgeLBPoolCpus.String()
		annotations[constants.EdgeLBPoolMemAnnotationKey] = options.EdgeLBPoolMem.String()
		annotations[constants.EdgeLBPoolSizeAnnotationKey] = strconv.Itoa(options.EdgeLBPoolSize)
	} else {
		annotations[constants.CloudLoadBalancerConfigMapNameAnnotationKey] = *options.CloudLoadBalancerConfigMapName
		delete(annotations, constants.EdgeLBPoolNameAnnotationKey)
		delete(annotations, constants.EdgeLBPoolRoleAnnotationKey)
		delete(annotations, constants.EdgeLBPoolNetworkAnnotationKey)
		delete(annotations, constants.EdgeLBPoolCpusAnnotationKey)
		delete(annotations, constants.EdgeLBPoolMemAnnotationKey)
		delete(annotations, constants.EdgeLBPoolSizeAnnotationKey)
	}
	object.SetAnnotations(annotations)
}

// setDefaultsOnIngress sets default values for each missing annotation on the specified "Ingress" resource.
func setDefaultsOnIngress(ingress *extsv1beta1.Ingress, options *translator.IngressTranslationOptions) {
	setBaseDefaults(ingress, &options.BaseTranslationOptions)
	if options.CloudLoadBalancerConfigMapName == nil {
		ingress.Annotations[constants.EdgeLBPoolPortAnnotationKey] = strconv.Itoa(int(options.EdgeLBPoolPort))
	} else {
		delete(ingress.Annotations, constants.EdgeLBPoolPortAnnotationKey)
	}
}

// setDefaultsOnService sets default values for each missing annotation on the specified "Service" resource.
func setDefaultsOnService(service *corev1.Service, options *translator.ServiceTranslationOptions) {
	setBaseDefaults(service, &options.BaseTranslationOptions)
	for sp, fp := range options.EdgeLBPoolPortMap {
		key := fmt.Sprintf("%s%d", constants.EdgeLBPoolPortMapKeyPrefix, sp)
		if options.CloudLoadBalancerConfigMapName == nil {
			service.Annotations[key] = strconv.Itoa(int(fp))
		} else {
			delete(service.Annotations, key)
		}
	}
}
