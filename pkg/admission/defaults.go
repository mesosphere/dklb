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
	annotations[constants.EdgeLBPoolNameAnnotationKey] = options.EdgeLBPoolName
	annotations[constants.EdgeLBPoolRoleAnnotationKey] = options.EdgeLBPoolRole
	annotations[constants.EdgeLBPoolNetworkAnnotationKey] = options.EdgeLBPoolNetwork
	annotations[constants.EdgeLBPoolCpusAnnotationKey] = options.EdgeLBPoolCpus.String()
	annotations[constants.EdgeLBPoolMemAnnotationKey] = options.EdgeLBPoolMem.String()
	annotations[constants.EdgeLBPoolSizeAnnotationKey] = strconv.Itoa(options.EdgeLBPoolSize)
	annotations[constants.EdgeLBPoolTranslationPaused] = strconv.FormatBool(options.EdgeLBPoolTranslationPaused)
	object.SetAnnotations(annotations)
}

// setDefaultsOnIngress sets default values for each missing annotation on the specified "Ingress" resource.
func setDefaultsOnIngress(ingress *extsv1beta1.Ingress, options *translator.IngressTranslationOptions) {
	setBaseDefaults(ingress, &options.BaseTranslationOptions)
	ingress.Annotations[constants.EdgeLBPoolPortKey] = strconv.Itoa(int(options.EdgeLBPoolPort))
}

// setDefaultsOnService sets default values for each missing annotation on the specified "Service" resource.
func setDefaultsOnService(service *corev1.Service, options *translator.ServiceTranslationOptions) {
	setBaseDefaults(service, &options.BaseTranslationOptions)
	for sp, fp := range options.EdgeLBPoolPortMap {
		service.Annotations[fmt.Sprintf("%s%d", constants.EdgeLBPoolPortMapKeyPrefix, sp)] = strconv.Itoa(int(fp))
	}
}
