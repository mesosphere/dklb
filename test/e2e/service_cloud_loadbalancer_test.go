// +build e2e

package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-redis/redis"
	"github.com/mesosphere/dcos-edge-lb/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/mesosphere/dklb/pkg/constants"
	translatorapi "github.com/mesosphere/dklb/pkg/translator/api"
	"github.com/mesosphere/dklb/pkg/util/kubernetes"
	"github.com/mesosphere/dklb/pkg/util/pointers"
	"github.com/mesosphere/dklb/pkg/util/retry"
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

			f.WithTemporaryNamespace(func(namespace *corev1.Namespace) {
				var (
					err           error
					hostname      string
					initialPool   *models.V2Pool
					initialSpec   *translatorapi.ServiceEdgeLBPoolSpec
					finalPool     *models.V2Pool
					finalPoolName string
					redisPod      *corev1.Pod
					redisSvc      *corev1.Service
					redisSvcName  string
					redisSvcPort  int32
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

				// Define the name and service port of the "redis" Service resource we will be creating, so we can use it in the cloud load-balancer's configuration.
				redisSvcName = "redis"
				redisSvcPort = 6379

				// Create an object holding the target EdgeLB pool's initial specification for the "redis" service.
				initialSpec = &translatorapi.ServiceEdgeLBPoolSpec{
					BaseEdgeLBPoolSpec: translatorapi.BaseEdgeLBPoolSpec{
						Name: pointers.NewString(namespace.Name),
						Role: pointers.NewString(constants.EdgeLBRolePublic),
					},
					Frontends: []translatorapi.ServiceEdgeLBPoolFrontendSpec{
						{
							Port:        pointers.NewInt32(16379),
							ServicePort: redisSvcPort,
						},
					},
				}

				// Create a service of type LoadBalancer targeting the pod created above.
				redisSvc, err = f.CreateServiceOfTypeLoadBalancer(namespace.Name, redisSvcName, func(svc *corev1.Service) {
					_ = translatorapi.SetServiceEdgeLBPoolSpec(svc, initialSpec)
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
					initialPool, err = f.EdgeLBManager.GetPool(ctx, *initialSpec.Name)
					return err == nil, nil
				})
				Expect(err).NotTo(HaveOccurred(), "timed out while waiting for the edgelb api server to acknowledge the pool's creation")

				// Make sure that the initial EdgeLB pool has been deployed to a single, public DC/OS agent, and that the frontend's bind port is the expected one.
				Expect(initialPool.Role).To(Equal(constants.EdgeLBRolePublic))
				Expect(initialPool.Haproxy.Frontends).To(HaveLen(1))
				Expect(*initialPool.Haproxy.Frontends[0].BindPort).To(Equal(*((*initialSpec).Frontends[0].Port)))

				// Create the desired cloud-provider configuration.
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

				// Update the target EdgeLB pool specification with the cloud-provider configuration.
				finalPoolName = fmt.Sprintf("%s--%s", constants.EdgeLBCloudProviderPoolNamePrefix, *initialSpec.Name)
				redisSvc, err = f.UpdateServiceEdgeLBPoolSpec(redisSvc, func(spec *translatorapi.ServiceEdgeLBPoolSpec) {
					spec.Name = &finalPoolName
					spec.CloudProviderConfiguration = pointers.NewString(string(redisCfgBytes))
					spec.Size = pointers.NewInt32(2)
					spec.Role = pointers.NewString(constants.EdgeLBRolePrivate)
				})
				Expect(err).NotTo(HaveOccurred(), "failed to update the test service with the cloud-provider configuration")

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
				Expect(*finalPool.Count).To(Equal(int32(2)))
				// Make sure that the initial EdgeLB pool has one frontend that binds to port "0" (dynamic).
				Expect(err).NotTo(HaveOccurred())
				Expect(finalPool.Haproxy.Frontends).To(HaveLen(1))
				Expect(*finalPool.Haproxy.Frontends[0].BindPort).To(Equal(int32(0)))

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
				err = f.EdgeLBManager.DeletePool(ctx, *initialSpec.Name)
				Expect(err).NotTo(HaveOccurred(), "failed to delete edgelb pool %q", initialSpec.Name)
			})
		})
	})
})
