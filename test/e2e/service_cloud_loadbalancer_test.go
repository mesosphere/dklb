// +build e2e

package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/go-redis/redis"
	"github.com/mesosphere/dcos-edge-lb/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/mesosphere/dklb/pkg/constants"
	"github.com/mesosphere/dklb/pkg/translator"
	"github.com/mesosphere/dklb/pkg/util/kubernetes"
	"github.com/mesosphere/dklb/pkg/util/pointers"
	"github.com/mesosphere/dklb/pkg/util/retry"
	"github.com/mesosphere/dklb/pkg/util/strings"
	"github.com/mesosphere/dklb/test/e2e/framework"
)

var _ = Describe("Service", func() {
	Context("of type LoadBalancer and for which a cloud load-balancer has been requested", func() {
		It("is correctly provisioned by EdgeLB [TCP] [Public] [Cloud]", func() {
			// Skip the test if no AWS public subnet ID was specified.
			if awsPublicSubnetId == "" {
				Skip(fmt.Sprintf("a non-empty value for --%s must be specified", awsPublicSubnetIdFlagName))
				return
			}

			// Create a temporary namespace for the current test.
			f.WithTemporaryNamespace(func(namespace *corev1.Namespace) {
				var (
					err                 error
					hostname            string
					initialPool         *models.V2Pool
					initialPoolBindPort int
					initialPoolName     string
					finalPool           *models.V2Pool
					finalPoolName       string
					redisCfgMap         *corev1.ConfigMap
					redisPod            *corev1.Pod
					redisSvc            *corev1.Service
					redisSvcName        string
					redisSvcPort        int32
				)

				// Create a pod running Redis.
				redisPod, err = f.CreatePod(namespace.Name, "redis", func(pod *corev1.Pod) {
					pod.ObjectMeta.Labels = map[string]string{
						"app": "redis",
					}
					pod.Spec.Containers = []corev1.Container{
						{
							Name:  "redis",
							Image: "redis:5.0.3",
							Ports: []corev1.ContainerPort{
								{
									Name:          "redis",
									Protocol:      corev1.ProtocolTCP,
									ContainerPort: 6379,
								},
							},
						},
					}
				})
				Expect(err).NotTo(HaveOccurred(), "failed to create redis test pod")

				// Define the name of the initial EdgeLB pool.
				initialPoolName = namespace.Name

				// Define the name and service port of the "redis" Service resource we will be creating, so we can use it in the cloud load-balancer's configuration.
				redisSvcName = "redis"
				redisSvcPort = 6379

				// Create a service of type LoadBalancer targeting the pod created above.
				redisSvc, err = f.CreateServiceOfTypeLoadBalancer(namespace.Name, redisSvcName, func(svc *corev1.Service) {
					svc.Annotations = map[string]string{
						constants.EdgeLBPoolNameAnnotationKey: initialPoolName,
					}
					svc.Spec.Ports = []corev1.ServicePort{
						{
							Name:       "redis",
							Protocol:   corev1.ProtocolTCP,
							Port:       redisSvcPort,
							TargetPort: intstr.FromInt(int(redisPod.Spec.Containers[0].Ports[0].ContainerPort)),
						},
					}
					svc.Spec.Selector = redisPod.ObjectMeta.Labels
				})
				Expect(err).NotTo(HaveOccurred(), "failed to create test service")

				// Wait for EdgeLB to acknowledge the pool's creation.
				err = retry.WithTimeout(framework.DefaultRetryTimeout, framework.DefaultRetryInterval, func() (bool, error) {
					ctx, fn := context.WithTimeout(context.Background(), framework.DefaultRetryInterval/2)
					defer fn()
					initialPool, err = f.EdgeLBManager.GetPool(ctx, initialPoolName)
					return err == nil, nil
				})
				Expect(err).NotTo(HaveOccurred(), "timed out while waiting for the edgelb api server to acknowledge the pool's creation")

				// Make sure that the initial EdgeLB pool has been deployed to a public DC/OS agent.
				Expect(initialPool.Role).To(Equal(constants.EdgeLBRolePublic))
				// Make sure that the initial EdgeLB pool has one frontend that binds to port "redisSvcPort", and that that binding is reflected as an annotation on the Service resource.
				initialPoolBindPort, err = strconv.Atoi(redisSvc.Annotations[fmt.Sprintf("%s%d", constants.EdgeLBPoolPortMapKeyPrefix, redisSvcPort)])
				Expect(err).NotTo(HaveOccurred())
				Expect(initialPool.Haproxy.Frontends).To(HaveLen(1))
				Expect(*initialPool.Haproxy.Frontends[0].BindPort).To(Equal(int32(initialPoolBindPort)))
				Expect(*initialPool.Haproxy.Frontends[0].BindPort).To(Equal(int32(redisSvcPort)))

				// Create a configmap containing the cloud load-balancer's configuration.
				redisCfgBytes, _ := json.Marshal(&models.V2CloudProvider{
					Aws: &models.V2CloudProviderAws{
						Elb: []*models.V2CloudProviderAwsElb{
							{
								Internal: pointers.NewBool(false),
								Listeners: []*models.V2CloudProviderAwsElbListener{
									{
										Port:         pointers.NewInt32(redisSvcPort),
										LinkFrontend: pointers.NewString(initialPool.Haproxy.Frontends[0].Name),
									},
								},
								Name: pointers.NewString(redisSvcName),
								Subnets: []string{
									awsPublicSubnetId,
								},
								Type: pointers.NewString("NLB"),
							},
						},
					},
				})
				redisCfgMap, err = f.CreateConfigMap(namespace.Name, "redis-aws-nlb", func(configMap *corev1.ConfigMap) {
					configMap.Data = map[string]string{
						constants.CloudLoadBalancerSpecKey: string(redisCfgBytes),
					}
				})

				// Re-read the Service resource from the Kubernetes API as it may have been updated in the meantime.
				redisSvc, err = f.KubeClient.CoreV1().Services(redisSvc.Namespace).Get(redisSvc.Name, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred(), "failed to read an updated version of the service resource")

				// Set the annotation specifying the name of the configmap used to configure a cloud load-balancer.
				redisSvc.Annotations[constants.CloudLoadBalancerConfigMapNameAnnotationKey] = redisCfgMap.Name
				redisSvc, err = f.KubeClient.CoreV1().Services(redisSvc.Namespace).Update(redisSvc)
				Expect(err).ToNot(HaveOccurred(), "failed to update the test service")

				// Compute the (expected) name of the target (final) EdgeLB pool.
				finalPoolName = translator.ComputeEdgeLBPoolName(constants.EdgeLBCloudLoadBalancerPoolNamePrefix, strings.ReplaceForwardSlashes(f.ClusterName, "--"), redisSvc.Namespace, redisSvc.Name)

				// Wait for EdgeLB to acknowledge the final EdgeLB pool's creation.
				err = retry.WithTimeout(framework.DefaultRetryTimeout, framework.DefaultRetryInterval, func() (bool, error) {
					ctx, fn := context.WithTimeout(context.Background(), framework.DefaultRetryInterval/2)
					defer fn()
					finalPool, err = f.EdgeLBManager.GetPool(ctx, finalPoolName)
					return err == nil, nil
				})
				Expect(err).NotTo(HaveOccurred(), "timed out while waiting for the edgelb api server to acknowledge the pool's creation")

				// Make sure that the final EdgeLB pool has been deployed to a private DC/OS agent.
				Expect(finalPool.Role).To(Equal(constants.EdgeLBRolePrivate))
				// Make sure that the initial EdgeLB pool has one frontend that binds to port "0" (dynamic).
				Expect(err).NotTo(HaveOccurred())
				Expect(finalPool.Haproxy.Frontends).To(HaveLen(1))
				Expect(*finalPool.Haproxy.Frontends[0].BindPort).To(Equal(int32(0)))
				Expect(redisSvc.Annotations).NotTo(HaveKey(constants.EdgeLBPoolNameAnnotationKey))
				Expect(redisSvc.Annotations).NotTo(HaveKey(constants.EdgeLBPoolRoleAnnotationKey))
				Expect(redisSvc.Annotations).NotTo(HaveKey(constants.EdgeLBPoolNetworkAnnotationKey))
				Expect(redisSvc.Annotations).NotTo(HaveKey(constants.EdgeLBPoolCpusAnnotationKey))
				Expect(redisSvc.Annotations).NotTo(HaveKey(constants.EdgeLBPoolMemAnnotationKey))
				Expect(redisSvc.Annotations).NotTo(HaveKey(constants.EdgeLBPoolSizeAnnotationKey))
				Expect(redisSvc.Annotations).NotTo(HaveKey(fmt.Sprintf("%s%d", constants.EdgeLBPoolPortMapKeyPrefix, redisSvcPort)))

				// Connect to Redis using the cloud load-balancer.
				// Use a larget value for the retry timeout since provisioning of the cloud load-balancer may take a long time.
				log.Debugf("waiting for the hostname for %q to be reported", kubernetes.Key(redisSvc))
				err = retry.WithTimeout(3*framework.DefaultRetryTimeout, framework.DefaultRetryInterval, func() (bool, error) {
					// Wait for the cloud load-balancer's hostname to be reported.
					ctx, fn := context.WithTimeout(context.Background(), framework.DefaultRetryTimeout)
					defer fn()
					hostname, err = f.WaitForHostnameForService(ctx, redisSvc)
					Expect(err).NotTo(HaveOccurred())
					Expect(hostname).NotTo(BeEmpty())
					// Attempt to connect to Redis using the reported hostname.
					log.Debugf("attempting to connect to %q at %q", kubernetes.Key(redisSvc), hostname)
					redisClient := redis.NewClient(&redis.Options{
						Addr: fmt.Sprintf("%s:%d", hostname, 6379),
						DB:   0,
					})
					p, _ := redisClient.Ping().Result()
					return p == "PONG", nil
				})
				Expect(err).NotTo(HaveOccurred(), "timed out while attempting to connect to redis at %q", hostname)

				// Manually delete the Service resource now in order to prevent the EdgeLB pool from being re-created during namespace deletion.
				err = f.KubeClient.CoreV1().Services(redisSvc.Namespace).Delete(redisSvc.Name, metav1.NewDeleteOptions(0))
				Expect(err).NotTo(HaveOccurred(), "failed to delete service %q", kubernetes.Key(redisSvc))

				// Manually delete the initial EdgeLB pool.
				ctx, fn := context.WithTimeout(context.Background(), framework.DefaultEdgeLBOperationTimeout)
				defer fn()
				err = f.EdgeLBManager.DeletePool(ctx, initialPoolName)
				Expect(err).NotTo(HaveOccurred(), "failed to delete edgelb pool %q", initialPoolName)
			})
		})
	})
})
