// +build e2e

package e2e_test

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis"
	"github.com/mesosphere/dcos-edge-lb/models"
	"github.com/mongodb/mongo-go-driver/mongo"
	"github.com/mongodb/mongo-go-driver/mongo/readpref"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/mesosphere/dklb/pkg/constants"
	"github.com/mesosphere/dklb/pkg/util/kubernetes"
	"github.com/mesosphere/dklb/pkg/util/pointers"
	"github.com/mesosphere/dklb/pkg/util/retry"
	"github.com/mesosphere/dklb/pkg/util/strings"
	"github.com/mesosphere/dklb/test/e2e/framework"
)

var _ = Describe("Service", func() {
	Context("of type other than LoadBalancer", func() {
		It("is ignored by the admission webhook [TCP] [Admission]", func() {
			// Create a temporary namespace for the current test.
			f.WithTemporaryNamespace(func(namespace *corev1.Namespace) {
				// Test every type of service other than "LoadBalancer".
				for _, t := range []corev1.ServiceType{corev1.ServiceTypeClusterIP, corev1.ServiceTypeNodePort, corev1.ServiceTypeExternalName, ""} {
					log.Infof("test case: service of type %q", t)
					_, err := f.CreateService(namespace.Name, "", func(service *corev1.Service) {
						// Set an annotation with a value that would be invalid should the Service resource be of type "LoadBalancer".
						service.Annotations = map[string]string{
							constants.EdgeLBPoolNameAnnotationKey: "__invalid_edgelb_pool_name__",
						}
						// ExternalName must be set for a service of type "ExternalName" to be valid.
						service.Spec.ExternalName = "foo"
						// Use a randomly-generated name for the Service resource.
						service.GenerateName = fmt.Sprintf("%s-", namespace.Name)
						// Define a basic set of ports.
						service.Spec.Ports = []corev1.ServicePort{
							{
								Port: 80,
							},
						}
						// Define a basic selector.
						service.Spec.Selector = map[string]string{
							"foo": "bar",
						}
						// Use "t" as the type of the service.
						service.Spec.Type = t
					})
					// Make sure that no error occurred (meaning the admission webhook has ignored the Service resource).
					Expect(err).NotTo(HaveOccurred())
				}
			})
		})
	})

	Context("of type LoadBalancer", func() {
		It("created with an invalid configuration is rejected by the admission webhook [TCP] [Admission]", func() {
			// Create a temporary namespace for the current test.
			f.WithTemporaryNamespace(func(namespace *corev1.Namespace) {
				tests := []struct {
					description               string
					fn                        framework.ServiceCustomizer
					expectedErrorMessageRegex string
				}{
					{
						description: "invalid edgelb pool name",
						fn: func(service *corev1.Service) {
							service.Annotations = map[string]string{
								constants.EdgeLBPoolNameAnnotationKey: "__foo__",
							}
						},
						expectedErrorMessageRegex: "\"__foo__\" is not valid as an edgelb pool name",
					},
					{
						description: "invalid edgelb pool network",
						fn: func(service *corev1.Service) {
							service.Annotations = map[string]string{
								constants.EdgeLBPoolNetworkAnnotationKey: "dcos",
							}
						},
						expectedErrorMessageRegex: "cannot join a dcos virtual network when the pool's role is \"slave_public\"",
					},
					{
						description: "invalid edgelb pool cpu request",
						fn: func(service *corev1.Service) {
							service.Annotations = map[string]string{
								constants.EdgeLBPoolCpusAnnotationKey: "foo",
							}
						},
						expectedErrorMessageRegex: "failed to parse \"foo\" as the amount of cpus to request",
					},
					{
						description: "invalid edgelb pool memory request",
						fn: func(service *corev1.Service) {
							service.Annotations = map[string]string{
								constants.EdgeLBPoolMemAnnotationKey: "foo",
							}
						},
						expectedErrorMessageRegex: "failed to parse \"foo\" as the amount of memory to request",
					},
					{
						description: "invalid edgelb pool size request",
						fn: func(service *corev1.Service) {
							service.Annotations = map[string]string{
								constants.EdgeLBPoolSizeAnnotationKey: "foo",
							}
						},
						expectedErrorMessageRegex: "failed to parse \"foo\" as the size to request for the edgelb pool",
					},
					{
						description: "invalid edgelb pool creation strategy",
						fn: func(service *corev1.Service) {
							service.Annotations = map[string]string{
								constants.EdgeLBPoolCreationStrategyAnnotationKey: "InvalidStrategy",
							}
						},
						expectedErrorMessageRegex: "failed to parse \"InvalidStrategy\" as a pool creation strategy",
					},
					{
						description: "invalid edgelb pool port map",
						fn: func(service *corev1.Service) {
							service.Annotations = map[string]string{
								fmt.Sprintf("%s%d", constants.EdgeLBPoolPortMapKeyPrefix, 80): "-1",
							}
							service.Spec.Ports = []corev1.ServicePort{
								{
									Port: 80,
								},
							}
						},
						expectedErrorMessageRegex: "-1 is not a valid port number",
					},
				}
				for _, test := range tests {
					log.Infof("test case: %s", test.description)
					_, err := f.CreateServiceOfTypeLoadBalancer(namespace.Name, "foo", test.fn)
					Expect(err).To(HaveOccurred())
					statusErr, ok := err.(*errors.StatusError)
					Expect(ok).To(BeTrue())
					Expect(statusErr.ErrStatus.Message).To(MatchRegexp(test.expectedErrorMessageRegex))
				}
			})
		})

		It("created with a valid configuration and updated to an invalid one is rejected by the admission webhook [TCP] [Admission]", func() {
			// Create a temporary namespace for the current test.
			f.WithTemporaryNamespace(func(namespace *corev1.Namespace) {
				var (
					err         error
					originalSvc *corev1.Service
				)

				// Create a dummy Service resource of type LoadBalancer.
				originalSvc, err = f.CreateServiceOfTypeLoadBalancer(namespace.Name, "redis", func(svc *corev1.Service) {
					svc.ObjectMeta.Annotations = map[string]string{
						// Request for the pool to be called "<namespace>".
						constants.EdgeLBPoolNameAnnotationKey: namespace.Name,
						// Request for the pool to be deployed to a private DC/OS agent.
						constants.EdgeLBPoolRoleAnnotationKey: "*",
						// Request for translation to be paused so that no EdgeLB pool is actually created.
						constants.EdgeLBPoolTranslationPaused: "true",
					}
					svc.Spec.Ports = []corev1.ServicePort{
						{
							Port: 80,
						},
					}
				})
				Expect(err).NotTo(HaveOccurred(), "failed to create test service")

				// Attempt to perform some forbidden updates on the values of the annotations and make sure an error is returned.
				tests := []struct {
					description               string
					fn                        func(*corev1.Service)
					expecterErrorMessageRegex string
				}{
					{
						description: "update the target edgelb pool's name",
						fn: func(service *corev1.Service) {
							service.Annotations[constants.EdgeLBPoolNameAnnotationKey] = "new-name"
						},
						expecterErrorMessageRegex: "the name of the target edgelb pool cannot be changed",
					},
					{
						description: "update the target edgelb pool's role",
						fn: func(service *corev1.Service) {
							service.Annotations[constants.EdgeLBPoolRoleAnnotationKey] = "new-role"
						},
						expecterErrorMessageRegex: "the role of the target edgelb pool cannot be changed",
					},
					{
						description: "update the target edgelb pool's virtual network",
						fn: func(service *corev1.Service) {
							service.Annotations[constants.EdgeLBPoolNetworkAnnotationKey] = "new-name"
						},
						expecterErrorMessageRegex: "the virtual network of the target edgelb pool cannot be changed",
					},
				}
				for _, test := range tests {
					log.Infof("test case: %s", test.description)
					// Create a clone of "originalSvc" so we can start each test case with a fresh, valid copy.
					updatedSvc := originalSvc.DeepCopy()
					// Update the clone.
					test.fn(updatedSvc)
					// Make sure an error is returned.
					_, err := f.KubeClient.CoreV1().Services(updatedSvc.Namespace).Update(updatedSvc)
					Expect(err).To(HaveOccurred())
					statusErr, ok := err.(*errors.StatusError)
					Expect(ok).To(BeTrue())
					Expect(statusErr.ErrStatus.Message).To(MatchRegexp(test.expecterErrorMessageRegex))
				}
			})
		})

		It("requested for public exposure is correctly provisioned by EdgeLB [TCP] [Public]", func() {
			// Create a temporary namespace for the current test.
			f.WithTemporaryNamespace(func(namespace *corev1.Namespace) {
				var (
					err      error
					pool     *models.V2Pool
					redisPod *corev1.Pod
					redisSvc *corev1.Service
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

				// We'd like to map port 6379 to a different frontend bind port, so we build the required annotation beforehand.
				portmapAnnotationKey := fmt.Sprintf("%s%d", constants.EdgeLBPoolPortMapKeyPrefix, redisPod.Spec.Containers[0].Ports[0].ContainerPort)

				// Create a service of type LoadBalancer targeting the pod created above.
				redisSvc, err = f.CreateServiceOfTypeLoadBalancer(namespace.Name, "redis", func(svc *corev1.Service) {
					svc.ObjectMeta.Annotations = map[string]string{
						// Request for the pool to be deployed to an agent with the "slave_public" role.
						constants.EdgeLBPoolRoleAnnotationKey: constants.EdgeLBRolePublic,
						// Request for the pool to be given 0.2 CPUs.
						constants.EdgeLBPoolCpusAnnotationKey: "200m",
						// Request for the pool to be given 256MiB of RAM.
						constants.EdgeLBPoolMemAnnotationKey: "256Mi",
						// Request for the pool to be deployed into a single agent.
						constants.EdgeLBPoolSizeAnnotationKey: "1",
						// Request for the 6379 port of the service to be mapped into the 16379 port.
						portmapAnnotationKey: "16379",
					}
					svc.Spec.Ports = []corev1.ServicePort{
						{
							Name:       "redis",
							Protocol:   corev1.ProtocolTCP,
							Port:       redisPod.Spec.Containers[0].Ports[0].ContainerPort,
							TargetPort: intstr.FromInt(int(redisPod.Spec.Containers[0].Ports[0].ContainerPort)),
						},
					}
					svc.Spec.Selector = redisPod.ObjectMeta.Labels
				})
				Expect(err).NotTo(HaveOccurred(), "failed to create test service")

				// Compute the (expected) name of the target EdgeLB pool.
				expectedPoolName := fmt.Sprintf("%s--%s--%s", strings.ReplaceForwardSlashes(f.ClusterName, "--"), redisSvc.Namespace, redisSvc.Name)

				// Wait for EdgeLB to acknowledge the pool's creation.
				err = retry.WithTimeout(framework.DefaultRetryTimeout, framework.DefaultRetryInterval, func() (bool, error) {
					ctx, fn := context.WithTimeout(context.Background(), framework.DefaultRetryInterval/2)
					defer fn()
					pool, err = f.EdgeLBManager.GetPoolByName(ctx, expectedPoolName)
					return err == nil, nil
				})
				Expect(err).NotTo(HaveOccurred(), "timed out while waiting for the edgelb api server to acknowledge the pool's creation")

				// Make sure the pool is reporting the requested configuration, as well as a name that contains the Service resource's namespace and name.
				Expect(pool.Name).To(Equal(expectedPoolName))
				Expect(pool.Role).To(Equal(redisSvc.Annotations[constants.EdgeLBPoolRoleAnnotationKey]))
				Expect(pool.Cpus).To(Equal(0.2))
				Expect(pool.Mem).To(Equal(int32(256)))
				Expect(pool.Count).To(Equal(pointers.NewInt32(1)))

				// TODO (@bcustodio) Wait for the pool's IP(s) to be reported.

				// Connect to the Redis instance using the EdgeLB pool.
				err = retry.WithTimeout(framework.DefaultRetryTimeout, framework.DefaultRetryInterval, func() (bool, error) {
					redisClient := redis.NewClient(&redis.Options{
						Addr: fmt.Sprintf("%s:%s", publicIP, redisSvc.Annotations[portmapAnnotationKey]),
						DB:   0,
					})
					p, _ := redisClient.Ping().Result()
					return p == "PONG", nil
				})
				Expect(err).NotTo(HaveOccurred(), "timed out while attempting to ping redis")

				// Manually delete the services now in order to prevent the EdgeLB pool from being re-created during namespace deletion.
				err = f.KubeClient.CoreV1().Services(redisSvc.Namespace).Delete(redisSvc.Name, metav1.NewDeleteOptions(0))
				Expect(err).NotTo(HaveOccurred(), "failed to delete service %q", kubernetes.Key(redisSvc))
			})
		})

		It("requested for public exposure can share a pool with a service in a different namespace [TCP] [Public]", func() {
			// Create two temporary namespaces for the test.
			f.WithTemporaryNamespace(func(namespace1 *corev1.Namespace) {
				f.WithTemporaryNamespace(func(namespace2 *corev1.Namespace) {
					var (
						err      error
						mongoPod *corev1.Pod
						mongoSvc *corev1.Service
						redisPod *corev1.Pod
						redisSvc *corev1.Service
						pool     *models.V2Pool
					)

					// Create a pod running Mongo.
					mongoPod, err = f.CreatePod(namespace1.Name, "mongo", func(pod *corev1.Pod) {
						pod.ObjectMeta.Labels = map[string]string{
							"app": "mongo",
						}
						pod.Spec.Containers = []corev1.Container{
							{
								Name:  "mongo",
								Image: "mongo:4.0.4",
								Ports: []corev1.ContainerPort{
									{
										Name:          "mongo",
										Protocol:      corev1.ProtocolTCP,
										ContainerPort: 27017,
									},
								},
							},
						}
					})
					Expect(err).NotTo(HaveOccurred(), "failed to create mongo test pod")

					// Create a service of type LoadBalancer targeting the Mongo pod created above.
					mongoSvc, err = f.CreateServiceOfTypeLoadBalancer(namespace1.Name, "mongo", func(svc *corev1.Service) {
						svc.ObjectMeta.Annotations = map[string]string{
							constants.EdgeLBPoolNameAnnotationKey: fmt.Sprintf("%s-%s", namespace1.Name, namespace2.Name),
							constants.EdgeLBPoolRoleAnnotationKey: constants.EdgeLBRolePublic,
						}
						svc.Spec.Ports = []corev1.ServicePort{
							{
								Name:       "mongo",
								Protocol:   corev1.ProtocolTCP,
								Port:       mongoPod.Spec.Containers[0].Ports[0].ContainerPort,
								TargetPort: intstr.FromInt(int(mongoPod.Spec.Containers[0].Ports[0].ContainerPort)),
							},
						}
						svc.Spec.Selector = mongoPod.ObjectMeta.Labels
					})
					Expect(err).NotTo(HaveOccurred(), "failed to create mongo test service")

					// Wait for EdgeLB to acknowledge the pool's creation.
					err = retry.WithTimeout(framework.DefaultRetryTimeout, framework.DefaultRetryInterval, func() (bool, error) {
						ctx, fn := context.WithTimeout(context.Background(), framework.DefaultRetryInterval/2)
						defer fn()
						pool, err = f.EdgeLBManager.GetPoolByName(ctx, mongoSvc.Annotations[constants.EdgeLBPoolNameAnnotationKey])
						return err == nil, nil
					})
					Expect(err).NotTo(HaveOccurred(), "timed out while waiting for the edgelb api server to acknowledge the pool's creation")

					// TODO (@bcustodio) Wait for the pool's IP(s) to be reported.

					// Wait for Mongo to be reachable.
					err = retry.WithTimeout(framework.DefaultRetryTimeout, framework.DefaultRetryInterval, func() (bool, error) {
						// Attempt to connect to Mongo.
						ctx1, fn1 := context.WithTimeout(context.Background(), framework.DefaultRetryInterval/2)
						defer fn1()
						mongoClient, err := mongo.Connect(ctx1, fmt.Sprintf("mongodb://%s:%d", publicIP, mongoSvc.Spec.Ports[0].Port))
						if err != nil {
							return false, nil
						}
						ctx2, fn2 := context.WithTimeout(context.Background(), framework.DefaultRetryInterval/2)
						defer fn2()
						err = mongoClient.Ping(ctx2, readpref.Primary())
						if err != nil {
							return false, nil
						}
						return true, nil
					})
					Expect(err).NotTo(HaveOccurred(), "timed out while attempting to ping mongo")

					// Create a pod running Redis.
					redisPod, err = f.CreatePod(namespace2.Name, "redis", func(pod *corev1.Pod) {
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

					// Create a service of type LoadBalancer targeting the Redis pod created above.
					redisSvc, err = f.CreateServiceOfTypeLoadBalancer(namespace2.Name, "redis", func(svc *corev1.Service) {
						svc.ObjectMeta.Annotations = map[string]string{
							// Reuse the existing EdgeLB pool.
							constants.EdgeLBPoolNameAnnotationKey: pool.Name,
						}
						svc.Spec.Ports = []corev1.ServicePort{
							{
								Name:       "redis",
								Protocol:   corev1.ProtocolTCP,
								Port:       redisPod.Spec.Containers[0].Ports[0].ContainerPort,
								TargetPort: intstr.FromInt(int(redisPod.Spec.Containers[0].Ports[0].ContainerPort)),
							},
						}
						svc.Spec.Selector = redisPod.ObjectMeta.Labels
					})
					Expect(err).NotTo(HaveOccurred(), "failed to create redis test service")

					// TODO (@bcustodio) Wait for the pool's IP(s) to be reported.

					// Wait for Redis to be reachable.
					err = retry.WithTimeout(framework.DefaultRetryTimeout, framework.DefaultRetryInterval, func() (bool, error) {
						redisClient := redis.NewClient(&redis.Options{
							Addr: fmt.Sprintf("%s:%d", publicIP, redisSvc.Spec.Ports[0].Port),
							DB:   0,
						})
						p, _ := redisClient.Ping().Result()
						return p == "PONG", nil
					})
					Expect(err).NotTo(HaveOccurred(), "timed out while attempting to ping mongo and/or redis")

					// Make sure we can still connect to Mongo.
					ctx1, fn1 := context.WithTimeout(context.Background(), 2*time.Second)
					defer fn1()
					mongoClient, err := mongo.Connect(ctx1, fmt.Sprintf("mongodb://%s:%d", publicIP, mongoSvc.Spec.Ports[0].Port))
					Expect(err).NotTo(HaveOccurred(), "failed to create mongo client")
					ctx2, fn2 := context.WithTimeout(context.Background(), 2*time.Second)
					defer fn2()
					err = mongoClient.Ping(ctx2, readpref.Primary())
					Expect(err).NotTo(HaveOccurred(), "failed to ping mongo")

					// Make sure there is a single EdgeLB pool.
					ctx3, fn3 := context.WithTimeout(context.Background(), framework.DefaultEdgeLBOperationTimeout)
					defer fn3()
					pools, err := f.EdgeLBManager.GetPools(ctx3)
					Expect(err).NotTo(HaveOccurred(), "failed to list edgelb pools")
					Expect(len(pools)).To(Equal(1), "expecting a single edgelb pool, found %d", len(pools))

					// Manually delete the two services now in order to prevent the EdgeLB pool from being re-created during namespace deletion.
					err = f.KubeClient.CoreV1().Services(redisSvc.Namespace).Delete(redisSvc.Name, metav1.NewDeleteOptions(0))
					Expect(err).NotTo(HaveOccurred(), "failed to delete service %q", kubernetes.Key(redisSvc))
					err = f.KubeClient.CoreV1().Services(mongoSvc.Namespace).Delete(mongoSvc.Name, metav1.NewDeleteOptions(0))
					Expect(err).NotTo(HaveOccurred(), "failed to delete service %q", kubernetes.Key(mongoSvc))
				})
			})
		})

		It("requested for public exposure and whose pool has been manually deleted is correctly re-provisioned by EdgeLB [TCP] [Public]", func() {
			// Create a temporary namespace for the current test.
			f.WithTemporaryNamespace(func(namespace *corev1.Namespace) {
				var (
					err      error
					pool     *models.V2Pool
					redisPod *corev1.Pod
					redisSvc *corev1.Service
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

				// We'd like to map port 6379 to a different frontend bind port, so we build the required annotation beforehand.
				portmapAnnotationKey := fmt.Sprintf("%s%d", constants.EdgeLBPoolPortMapKeyPrefix, redisPod.Spec.Containers[0].Ports[0].ContainerPort)

				// Create a service of type LoadBalancer targeting the pod created above.
				redisSvc, err = f.CreateServiceOfTypeLoadBalancer(namespace.Name, "redis", func(svc *corev1.Service) {
					svc.ObjectMeta.Annotations = map[string]string{
						// Request for the pool to be deployed to an agent with the "slave_public" role.
						constants.EdgeLBPoolRoleAnnotationKey: constants.EdgeLBRolePublic,
						// Request for the pool to be given 0.2 CPUs.
						constants.EdgeLBPoolCpusAnnotationKey: "200m",
						// Request for the pool to be given 256MiB of RAM.
						constants.EdgeLBPoolMemAnnotationKey: "256Mi",
						// Request for the pool to be deployed into a single agent.
						constants.EdgeLBPoolSizeAnnotationKey: "1",
						// Request for the 6379 port of the service to be mapped into the 16379 port.
						portmapAnnotationKey: "16379",
					}
					svc.Spec.Ports = []corev1.ServicePort{
						{
							Name:       "redis",
							Protocol:   corev1.ProtocolTCP,
							Port:       redisPod.Spec.Containers[0].Ports[0].ContainerPort,
							TargetPort: intstr.FromInt(int(redisPod.Spec.Containers[0].Ports[0].ContainerPort)),
						},
					}
					svc.Spec.Selector = redisPod.ObjectMeta.Labels
				})
				Expect(err).NotTo(HaveOccurred(), "failed to create test service")

				// Compute the (expected) name of the target EdgeLB pool.
				expectedPoolName := fmt.Sprintf("%s--%s--%s", strings.ReplaceForwardSlashes(f.ClusterName, "--"), redisSvc.Namespace, redisSvc.Name)

				// Wait for EdgeLB to acknowledge the pool's creation.
				err = retry.WithTimeout(framework.DefaultRetryTimeout, framework.DefaultRetryInterval, func() (bool, error) {
					ctx, fn := context.WithTimeout(context.Background(), framework.DefaultRetryInterval/2)
					defer fn()
					pool, err = f.EdgeLBManager.GetPoolByName(ctx, expectedPoolName)
					return err == nil, nil
				})
				Expect(err).NotTo(HaveOccurred(), "timed out while waiting for the edgelb api server to acknowledge the pool's creation")

				// Wait for Redis to be reachable.
				err = retry.WithTimeout(framework.DefaultRetryTimeout, framework.DefaultRetryInterval, func() (bool, error) {
					redisClient := redis.NewClient(&redis.Options{
						Addr: fmt.Sprintf("%s:%s", publicIP, redisSvc.Annotations[portmapAnnotationKey]),
						DB:   0,
					})
					p, _ := redisClient.Ping().Result()
					return p == "PONG", nil
				})
				Expect(err).NotTo(HaveOccurred(), "timed out while attempting to ping redis")

				// Pause translation of the Service resource.
				// This will cause the service controller to stop processing this resource, and will prevent the pool from being re-created too soon.
				// This is required in order to prevent https://jira.mesosphere.com/browse/DCOS-46508 from happening due to the controller's resync period elapsing in the meantime.
				redisSvc.Annotations[constants.EdgeLBPoolTranslationPaused] = "1"
				redisSvc, err = f.KubeClient.CoreV1().Services(redisSvc.Namespace).Update(redisSvc)
				Expect(err).NotTo(HaveOccurred(), "failed to set the type of %q to %q", kubernetes.Key(redisSvc), corev1.ServiceTypeNodePort)

				// Delete the pool.
				f.DeleteEdgeLBPool(pool)

				// Wait for Redis to NOT be reachable (meaning the pool and all load-balancer instances have been effectively deleted).
				err = retry.WithTimeout(framework.DefaultRetryTimeout, framework.DefaultRetryInterval, func() (bool, error) {
					redisClient := redis.NewClient(&redis.Options{
						Addr: fmt.Sprintf("%s:%s", publicIP, redisSvc.Annotations[portmapAnnotationKey]),
						DB:   0,
					})
					_, err := redisClient.Ping().Result()
					return err != nil, nil
				})
				Expect(err).NotTo(HaveOccurred(), "timed out while waiting for redis to become unreachable")

				// Give the pool's framework scheduler some extra time to perform cleanup before causing the pool to be recreated.
				// https://jira.mesosphere.com/browse/DCOS-46508
				time.Sleep(30 * time.Second)

				// Resume translation of the Service resource.
				// This will cause the service controller to restart processing this resource, and re-create the pool since we're using the default pool re-creation strategy.
				redisSvc.Annotations[constants.EdgeLBPoolTranslationPaused] = "0"
				redisSvc, err = f.KubeClient.CoreV1().Services(redisSvc.Namespace).Update(redisSvc)
				Expect(err).NotTo(HaveOccurred(), "failed to set the type of %q to %q", kubernetes.Key(redisSvc), corev1.ServiceTypeNodePort)

				// Wait for the pool to be re-provisioned, making Redis reachable again.
				err = retry.WithTimeout(framework.DefaultRetryTimeout, framework.DefaultRetryInterval, func() (bool, error) {
					redisClient := redis.NewClient(&redis.Options{
						Addr: fmt.Sprintf("%s:%s", publicIP, redisSvc.Annotations[portmapAnnotationKey]),
						DB:   0,
					})
					p, _ := redisClient.Ping().Result()
					return p == "PONG", nil
				})
				Expect(err).NotTo(HaveOccurred(), "timed out while attempting to ping redis")

				// Manually delete the two services now in order to prevent the EdgeLB pool from being re-created during namespace deletion.
				err = f.KubeClient.CoreV1().Services(redisSvc.Namespace).Delete(redisSvc.Name, metav1.NewDeleteOptions(0))
				Expect(err).NotTo(HaveOccurred(), "failed to delete service %q", kubernetes.Key(redisSvc))
			})
		})
	})
})
