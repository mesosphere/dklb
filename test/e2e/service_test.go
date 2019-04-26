// +build e2e

package e2e_test

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/go-redis/redis"
	"github.com/mesosphere/dcos-edge-lb/models"
	"github.com/mongodb/mongo-go-driver/mongo"
	"github.com/mongodb/mongo-go-driver/mongo/readpref"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/mesosphere/dklb/pkg/constants"
	"github.com/mesosphere/dklb/pkg/util/kubernetes"
	"github.com/mesosphere/dklb/pkg/util/pointers"
	"github.com/mesosphere/dklb/pkg/util/retry"
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
						// Use an invalid value for "kubernetes.dcos.io/dklb-config" (i.e. one for which ".size" is negative).
						// The resulting Service resource would be be invalid, and hence an error would be reported, should it be of type "LoadBalancer".
						_ = translatorapi.SetServiceEdgeLBPoolSpec(service, &translatorapi.ServiceEdgeLBPoolSpec{
							BaseEdgeLBPoolSpec: translatorapi.BaseEdgeLBPoolSpec{
								Size: pointers.NewInt32(-1),
							},
						})
						// ExternalName must be set for a Service resource of type "ExternalName" to be valid.
						if t == corev1.ServiceTypeExternalName {
							service.Spec.ExternalName = "foo"
						}
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
		It("created without the \"kubernetes.dcos.io/dklb-config\" annotation is mutated with a default configuration [TCP] [Admission]", func() {
			f.WithTemporaryNamespace(func(namespace *corev1.Namespace) {
				var (
					err     error
					objSpec translatorapi.ServiceEdgeLBPoolSpec
					rawSpec string
					svc     *corev1.Service
				)

				// Create a Service resource of type LoadBalancer without the "kubernetes.dcos.io/dklb-config" annotation.
				svc, err = f.CreateServiceOfTypeLoadBalancer(namespace.Name, "bare-service", func(service *corev1.Service) {
					service.Annotations = map[string]string{
						constants.DklbPaused: strconv.FormatBool(true),
					}
					service.Spec.Ports = []corev1.ServicePort{
						{
							Port: 80,
						},
					}
					service.Spec.Selector = map[string]string{
						"foo": "bar",
					}
				})
				Expect(err).NotTo(HaveOccurred(), "failed to create test service")

				// Make sure that the Service resource has been mutated with a non-empty value for the "kubernetes.dcos.io/dklb-config" annotation.
				rawSpec = svc.Annotations[constants.DklbConfigAnnotationKey]
				Expect(rawSpec).NotTo(BeEmpty(), "the \"kubernetes.dcos.io/dklb-config\" annotation is absent or empty")
				// Make sure that the value of the "kubernetes.dcos.io/dklb-config" annotation can be unmarshaled into an ServiceEdgeLBPoolSpec object.
				err = yaml.UnmarshalStrict([]byte(rawSpec), &objSpec)
				Expect(err).NotTo(HaveOccurred(), "failed to unmarshal the value of the \"kubernetes.dcos.io/dklb-config\" annotation")
				// Make sure that the default values are set on the ServiceEdgeLBPoolSpec.
				Expect(*objSpec.Name).To(MatchRegexp(constants.EdgeLBPoolNameRegex))
				Expect(*objSpec.Name).To(MatchRegexp("^.*--[a-z0-9]{5}$"))
				Expect(*objSpec.Role).To(Equal(translatorapi.DefaultEdgeLBPoolRole))
				Expect(*objSpec.Network).To(Equal(constants.EdgeLBHostNetwork))
				Expect(*objSpec.CPUs).To(Equal(translatorapi.DefaultEdgeLBPoolCpus))
				Expect(*objSpec.Memory).To(Equal(translatorapi.DefaultEdgeLBPoolMemory))
				Expect(*objSpec.Size).To(Equal(translatorapi.DefaultEdgeLBPoolSize))
				Expect(objSpec.Frontends).To(HaveLen(len(svc.Spec.Ports)))
				Expect(objSpec.Frontends[0].ServicePort).To(Equal(svc.Spec.Ports[0].Port))
				Expect(*objSpec.Frontends[0].Port).To(Equal(svc.Spec.Ports[0].Port))
			})
		})

		It("created with an invalid configuration is rejected by the admission webhook [TCP] [Admission]", func() {
			// Create a temporary namespace for the current test.
			f.WithTemporaryNamespace(func(namespace *corev1.Namespace) {
				tests := []struct {
					description               string
					fn                        framework.ServiceCustomizer
					expectedErrorMessageRegex string
				}{
					{
						description: "\"kubernetes.dcos.io/dklb-config\" cannot be parsed as yaml",
						fn: func(svc *corev1.Service) {
							svc.Annotations[constants.DklbConfigAnnotationKey] = "invalid: yaml: str"
							svc.Spec.Ports = []corev1.ServicePort{
								{
									Port: 80,
								},
							}
						},
						expectedErrorMessageRegex: "failed to parse the value of \"kubernetes.dcos.io/dklb-config\" as a configuration object",
					},
					{
						description: "\"kubernetes.dcos.io/dklb-config\" specifies an invalid edgelb pool name",
						fn: func(svc *corev1.Service) {
							_ = translatorapi.SetServiceEdgeLBPoolSpec(svc, &translatorapi.ServiceEdgeLBPoolSpec{
								BaseEdgeLBPoolSpec: translatorapi.BaseEdgeLBPoolSpec{
									Name: pointers.NewString("__foo__"),
								},
							})
							svc.Spec.Ports = []corev1.ServicePort{
								{
									Port: 80,
								},
							}
						},
						expectedErrorMessageRegex: "\"__foo__\" is not a valid edgelb pool name",
					},
					{
						description: "\"kubernetes.dcos.io/dklb-config\" specifies an invalid edgelb pool network",
						fn: func(svc *corev1.Service) {
							_ = translatorapi.SetServiceEdgeLBPoolSpec(svc, &translatorapi.ServiceEdgeLBPoolSpec{
								BaseEdgeLBPoolSpec: translatorapi.BaseEdgeLBPoolSpec{
									Network: pointers.NewString("dcos"),
								},
							})
							svc.Spec.Ports = []corev1.ServicePort{
								{
									Port: 80,
								},
							}
						},
						expectedErrorMessageRegex: "cannot join a virtual network when the pool's role is \"slave_public\"",
					},
					{
						description: "\"kubernetes.dcos.io/dklb-config\" specifies an invalid edgelb pool cpu request",
						fn: func(svc *corev1.Service) {
							_ = translatorapi.SetServiceEdgeLBPoolSpec(svc, &translatorapi.ServiceEdgeLBPoolSpec{
								BaseEdgeLBPoolSpec: translatorapi.BaseEdgeLBPoolSpec{
									CPUs: pointers.NewFloat64(-0.1),
								},
							})
							svc.Spec.Ports = []corev1.ServicePort{
								{
									Port: 80,
								},
							}
						},
						expectedErrorMessageRegex: "-0.100000 is not a valid cpu request",
					},
					{
						description: "\"kubernetes.dcos.io/dklb-config\" specifies an invalid edgelb pool memory request",
						fn: func(svc *corev1.Service) {
							_ = translatorapi.SetServiceEdgeLBPoolSpec(svc, &translatorapi.ServiceEdgeLBPoolSpec{
								BaseEdgeLBPoolSpec: translatorapi.BaseEdgeLBPoolSpec{
									Memory: pointers.NewInt32(-256),
								},
							})
							svc.Spec.Ports = []corev1.ServicePort{
								{
									Port: 80,
								},
							}
						},
						expectedErrorMessageRegex: "-256 is not a valid memory request",
					},
					{
						description: "\"kubernetes.dcos.io/dklb-config\" specifies an invalid edgelb pool size request",
						fn: func(svc *corev1.Service) {
							_ = translatorapi.SetServiceEdgeLBPoolSpec(svc, &translatorapi.ServiceEdgeLBPoolSpec{
								BaseEdgeLBPoolSpec: translatorapi.BaseEdgeLBPoolSpec{
									Size: pointers.NewInt32(-1),
								},
							})
							svc.Spec.Ports = []corev1.ServicePort{
								{
									Port: 80,
								},
							}
						},
						expectedErrorMessageRegex: "-1 is not a valid size request",
					},
					{
						description: "\"kubernetes.dcos.io/dklb-config\" specifies an invalid edgelb pool creation strategy",
						fn: func(svc *corev1.Service) {
							strategy := translatorapi.EdgeLBPoolCreationStrategy("InvalidStrategy")
							_ = translatorapi.SetServiceEdgeLBPoolSpec(svc, &translatorapi.ServiceEdgeLBPoolSpec{
								BaseEdgeLBPoolSpec: translatorapi.BaseEdgeLBPoolSpec{
									Strategies: &translatorapi.EdgeLBPoolManagementStrategies{
										Creation: &strategy,
									},
								},
							})
							svc.Spec.Ports = []corev1.ServicePort{
								{
									Port: 80,
								},
							}
						},
						expectedErrorMessageRegex: "failed to parse \"InvalidStrategy\" as an edgelb pool creation strategy",
					},
					{
						description: "\"kubernetes.dcos.io/dklb-config\" specifies an invalid edgelb frontend port",
						fn: func(svc *corev1.Service) {
							_ = translatorapi.SetServiceEdgeLBPoolSpec(svc, &translatorapi.ServiceEdgeLBPoolSpec{
								Frontends: []translatorapi.ServiceEdgeLBPoolFrontendSpec{
									{
										Port:        pointers.NewInt32(123456),
										ServicePort: 80,
									},
								},
							})
							svc.Spec.Ports = []corev1.ServicePort{
								{
									Port: 80,
								},
							}
						},
						expectedErrorMessageRegex: "123456 is not a valid port number",
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
					err            error
					initialService *corev1.Service
				)

				// Create a Service resource of type "LoadBalancer" containing a valid EdgeLB pool specification.
				initialService, err = f.CreateServiceOfTypeLoadBalancer(namespace.Name, "redis", func(svc *corev1.Service) {
					_ = translatorapi.SetServiceEdgeLBPoolSpec(svc, &translatorapi.ServiceEdgeLBPoolSpec{
						BaseEdgeLBPoolSpec: translatorapi.BaseEdgeLBPoolSpec{
							Name: pointers.NewString(namespace.Name),
							Role: pointers.NewString(constants.EdgeLBRolePrivate),
						},
					})
					// Request for translation to be paused so that no EdgeLB pool is actually created.
					svc.ObjectMeta.Annotations[constants.DklbPaused] = strconv.FormatBool(true)
					// Define a service port so that the Service resource can actually be created.
					svc.Spec.Ports = []corev1.ServicePort{
						{
							Port: 80,
						},
					}
				})
				Expect(err).NotTo(HaveOccurred(), "failed to create test service")

				// Attempt to perform some forbidden updates to the target EdgeLB pool's specification.
				tests := []struct {
					description               string
					fn                        func(*translatorapi.ServiceEdgeLBPoolSpec)
					expecterErrorMessageRegex string
				}{
					{
						description: "update the target edgelb pool's name",
						fn: func(spec *translatorapi.ServiceEdgeLBPoolSpec) {
							spec.Name = pointers.NewString("new-name")
						},
						expecterErrorMessageRegex: "the name of the target edgelb pool cannot be changed",
					},
					{
						description: "update the target edgelb pool's role",
						fn: func(spec *translatorapi.ServiceEdgeLBPoolSpec) {
							spec.Role = pointers.NewString("new-role")
						},
						expecterErrorMessageRegex: "the role of the target edgelb pool cannot be changed",
					},
					{
						description: "update the target edgelb pool's virtual network",
						fn: func(spec *translatorapi.ServiceEdgeLBPoolSpec) {
							spec.Network = pointers.NewString("new-network")
						},
						expecterErrorMessageRegex: "the virtual network of the target edgelb pool cannot be changed",
					},
				}
				for _, test := range tests {
					log.Infof("test case: %s", test.description)
					// Update the initial Service resource with the desired EdgeLB pool specification.
					_, err = f.UpdateServiceEdgeLBPoolSpec(initialService.DeepCopy(), test.fn)
					// Make sure that the expected error has occurred.
					Expect(err).To(HaveOccurred())
					statusErr, ok := err.(*errors.StatusError)
					Expect(ok).To(BeTrue())
					Expect(statusErr.ErrStatus.Message).To(MatchRegexp(test.expecterErrorMessageRegex))
				}
			})
		})

		It("is correctly provisioned by EdgeLB [TCP] [Public]", func() {
			// Create a temporary namespace for the current test.
			f.WithTemporaryNamespace(func(namespace *corev1.Namespace) {
				var (
					err       error
					pool      *models.V2Pool
					redisPod  *corev1.Pod
					redisSpec translatorapi.ServiceEdgeLBPoolSpec
					redisSvc  *corev1.Service
					publicIP  string
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

				// Create an object holding the target EdgeLB pool's specification.
				redisSpec = translatorapi.ServiceEdgeLBPoolSpec{
					BaseEdgeLBPoolSpec: translatorapi.BaseEdgeLBPoolSpec{
						// Request for the EdgeLB pool to have the same name as the current namespace.
						Name: pointers.NewString(namespace.Name),
						// Request for the EdgeLB pool to be deployed to an agent with the "slave_public" role.
						Role: pointers.NewString(constants.EdgeLBRolePublic),
						// Request for the EdgeLB pool to be given 0.2 CPUs.
						CPUs: pointers.NewFloat64(0.2),
						// Request for the EdgeLB pool to be given 256MiB of RAM.
						Memory: pointers.NewInt32(256),
						// Request for the EdgeLB pool to have a single instance.
						Size: pointers.NewInt32(1),
					},
					Frontends: []translatorapi.ServiceEdgeLBPoolFrontendSpec{
						{
							// Request for the 6379 service port to be mapped into the 16379 frontend port.
							Port:        pointers.NewInt32(16379),
							ServicePort: redisPod.Spec.Containers[0].Ports[0].ContainerPort,
						},
					},
				}

				// Create a service of type LoadBalancer targeting the pod created above and using the EdgeLB pool specification created above..
				redisSvc, err = f.CreateServiceOfTypeLoadBalancer(namespace.Name, "redis", func(svc *corev1.Service) {
					_ = translatorapi.SetServiceEdgeLBPoolSpec(svc, &redisSpec)
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

				// Wait for EdgeLB to acknowledge the pool's creation.
				err = retry.WithTimeout(framework.DefaultRetryTimeout, framework.DefaultRetryInterval, func() (bool, error) {
					ctx, fn := context.WithTimeout(context.Background(), framework.DefaultRetryInterval/2)
					defer fn()
					pool, err = f.EdgeLBManager.GetPool(ctx, *redisSpec.Name)
					return err == nil, nil
				})
				Expect(err).NotTo(HaveOccurred(), "timed out while waiting for the edgelb api server to acknowledge the pool's creation")

				// Make sure the pool is reporting the requested configuration, as well as a name that contains the Service resource's namespace and name.
				Expect(pool.Name).To(Equal(*redisSpec.Name))
				Expect(pool.Role).To(Equal(*redisSpec.Role))
				Expect(pool.Cpus).To(Equal(*redisSpec.CPUs))
				Expect(pool.Mem).To(Equal(*redisSpec.Memory))
				Expect(*pool.Count).To(Equal(*redisSpec.Size))

				// Connect to Redis using the EdgeLB pool.
				log.Debugf("waiting for the public ip for %q to be reported", kubernetes.Key(redisSvc))
				err = retry.WithTimeout(framework.DefaultRetryTimeout, framework.DefaultRetryInterval, func() (bool, error) {
					// Wait for the pool's public IP to be reported.
					ctx, fn := context.WithTimeout(context.Background(), framework.DefaultRetryTimeout)
					defer fn()
					publicIP, err = f.WaitForPublicIPForService(ctx, redisSvc)
					Expect(err).NotTo(HaveOccurred())
					Expect(publicIP).NotTo(BeEmpty())
					// Attempt to connect to Redis using the reported IP.
					addr := fmt.Sprintf("%s:%d", publicIP, *redisSpec.Frontends[0].Port)
					log.Debugf("attempting to connect to %q at %q", kubernetes.Key(redisSvc), addr)
					redisClient := redis.NewClient(&redis.Options{
						Addr: addr,
						DB:   0,
					})
					p, _ := redisClient.Ping().Result()
					return p == "PONG", nil
				})
				Expect(err).NotTo(HaveOccurred(), "timed out while attempting to ping redis")

				// Manually delete the Service resource now so that the target EdgeLB pool isn't possibly left dangling after namespace deletion.
				err = f.KubeClient.CoreV1().Services(redisSvc.Namespace).Delete(redisSvc.Name, metav1.NewDeleteOptions(0))
				Expect(err).NotTo(HaveOccurred(), "failed to delete service %q", kubernetes.Key(redisSvc))
			})
		})

		It("can share a pool with a service in a different namespace [TCP] [Public]", func() {
			// Create two temporary namespaces for the test.
			f.WithTemporaryNamespace(func(namespace1 *corev1.Namespace) {
				f.WithTemporaryNamespace(func(namespace2 *corev1.Namespace) {
					var (
						err       error
						mongoPod  *corev1.Pod
						mongoSpec translatorapi.ServiceEdgeLBPoolSpec
						mongoSvc  *corev1.Service
						redisPod  *corev1.Pod
						redisSpec translatorapi.ServiceEdgeLBPoolSpec
						redisSvc  *corev1.Service
						pool      *models.V2Pool
						publicIP  string
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

					// Create an object holding the target EdgeLB pool's specification for the "mongo" service.
					mongoSpec = translatorapi.ServiceEdgeLBPoolSpec{
						BaseEdgeLBPoolSpec: translatorapi.BaseEdgeLBPoolSpec{
							Name: pointers.NewString(namespace1.Name),
							Role: pointers.NewString(constants.EdgeLBRolePublic),
						},
						Frontends: []translatorapi.ServiceEdgeLBPoolFrontendSpec{
							{
								Port:        pointers.NewInt32(mongoPod.Spec.Containers[0].Ports[0].ContainerPort),
								ServicePort: mongoPod.Spec.Containers[0].Ports[0].ContainerPort,
							},
						},
					}

					// Create a service of type LoadBalancer targeting the Mongo pod created above.
					mongoSvc, err = f.CreateServiceOfTypeLoadBalancer(namespace1.Name, "mongo", func(svc *corev1.Service) {
						_ = translatorapi.SetServiceEdgeLBPoolSpec(svc, &mongoSpec)
						svc.Spec.Ports = []corev1.ServicePort{
							{
								Name:       "mongo",
								Protocol:   corev1.ProtocolTCP,
								Port:       mongoSpec.Frontends[0].ServicePort,
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
						pool, err = f.EdgeLBManager.GetPool(ctx, *mongoSpec.Name)
						return err == nil, nil
					})
					Expect(err).NotTo(HaveOccurred(), "timed out while waiting for the edgelb api server to acknowledge the pool's creation")

					// Wait for Mongo to be reachable.
					log.Debugf("waiting for the public ip for %q to be reported", kubernetes.Key(mongoSvc))
					err = retry.WithTimeout(framework.DefaultRetryTimeout, framework.DefaultRetryInterval, func() (bool, error) {
						// Wait for the pool's public IP to be reported.
						ctx, fn := context.WithTimeout(context.Background(), framework.DefaultRetryTimeout)
						defer fn()
						publicIP, err = f.WaitForPublicIPForService(ctx, mongoSvc)
						Expect(err).NotTo(HaveOccurred())
						Expect(publicIP).NotTo(BeEmpty())
						// Attempt to connect to Mongo using the reported IP.
						addr := fmt.Sprintf("%s:%d", publicIP, *mongoSpec.Frontends[0].Port)
						log.Debugf("attempting to connect to %q at %q", kubernetes.Key(mongoSvc), addr)
						ctx1, fn1 := context.WithTimeout(context.Background(), framework.DefaultRetryInterval/2)
						defer fn1()
						mongoClient, err := mongo.Connect(ctx1, fmt.Sprintf("mongodb://%s", addr))
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

					// Create an object holding the target EdgeLB pool's specification for the "redis" service.
					redisSpec = translatorapi.ServiceEdgeLBPoolSpec{
						BaseEdgeLBPoolSpec: translatorapi.BaseEdgeLBPoolSpec{
							// Reuse the EdgeLB pool used by the "mongo" service.
							Name: pointers.NewString(pool.Name),
						},
						Frontends: []translatorapi.ServiceEdgeLBPoolFrontendSpec{
							{
								Port:        pointers.NewInt32(redisPod.Spec.Containers[0].Ports[0].ContainerPort),
								ServicePort: redisPod.Spec.Containers[0].Ports[0].ContainerPort,
							},
						},
					}

					// Create a service of type LoadBalancer targeting the Redis pod created above.
					redisSvc, err = f.CreateServiceOfTypeLoadBalancer(namespace2.Name, "redis", func(svc *corev1.Service) {
						_ = translatorapi.SetServiceEdgeLBPoolSpec(svc, &redisSpec)
						svc.Spec.Ports = []corev1.ServicePort{
							{
								Name:       "redis",
								Protocol:   corev1.ProtocolTCP,
								Port:       redisSpec.Frontends[0].ServicePort,
								TargetPort: intstr.FromInt(int(redisPod.Spec.Containers[0].Ports[0].ContainerPort)),
							},
						}
						svc.Spec.Selector = redisPod.ObjectMeta.Labels
					})
					Expect(err).NotTo(HaveOccurred(), "failed to create redis test service")

					// Wait for Redis to be reachable.
					log.Debugf("waiting for the public ip for %q to be reported", kubernetes.Key(redisSvc))
					err = retry.WithTimeout(framework.DefaultRetryTimeout, framework.DefaultRetryInterval, func() (bool, error) {
						// Wait for the pool's public IP to be reported.
						ctx, fn := context.WithTimeout(context.Background(), framework.DefaultRetryTimeout)
						defer fn()
						publicIP, err = f.WaitForPublicIPForService(ctx, redisSvc)
						Expect(err).NotTo(HaveOccurred())
						Expect(publicIP).NotTo(BeEmpty())
						addr := fmt.Sprintf("%s:%d", publicIP, *redisSpec.Frontends[0].Port)
						log.Debugf("attempting to connect to %q at %q", kubernetes.Key(redisSvc), addr)
						// Attempt to connect to Redis using the reported IP.
						redisClient := redis.NewClient(&redis.Options{
							Addr: addr,
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

		It("and whose pool has been manually deleted is correctly re-provisioned by EdgeLB [TCP] [Public]", func() {
			// Create a temporary namespace for the current test.
			f.WithTemporaryNamespace(func(namespace *corev1.Namespace) {
				var (
					err       error
					pool      *models.V2Pool
					redisPod  *corev1.Pod
					redisSpec translatorapi.ServiceEdgeLBPoolSpec
					redisSvc  *corev1.Service
					publicIP  string
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

				// Create an object holding the target EdgeLB pool's specification.
				redisSpec = translatorapi.ServiceEdgeLBPoolSpec{
					BaseEdgeLBPoolSpec: translatorapi.BaseEdgeLBPoolSpec{
						// Request for the EdgeLB pool to have the same name as the current namespace.
						Name: pointers.NewString(namespace.Name),
						// Request for the EdgeLB pool to be deployed to an agent with the "slave_public" role.
						Role: pointers.NewString(constants.EdgeLBRolePublic),
						// Request for the EdgeLB pool to be given 0.2 CPUs.
						CPUs: pointers.NewFloat64(0.2),
						// Request for the EdgeLB pool to be given 256MiB of RAM.
						Memory: pointers.NewInt32(256),
						// Request for the EdgeLB pool to have a single instance.
						Size: pointers.NewInt32(1),
					},
					Frontends: []translatorapi.ServiceEdgeLBPoolFrontendSpec{
						{
							// Request for the 6379 service port to be mapped into the 16379 frontend port.
							Port:        pointers.NewInt32(16379),
							ServicePort: redisPod.Spec.Containers[0].Ports[0].ContainerPort,
						},
					},
				}

				// Create a service of type LoadBalancer targeting the pod created above.
				redisSvc, err = f.CreateServiceOfTypeLoadBalancer(namespace.Name, "redis", func(svc *corev1.Service) {
					_ = translatorapi.SetServiceEdgeLBPoolSpec(svc, &redisSpec)
					svc.Spec.Ports = []corev1.ServicePort{
						{
							Name:       "redis",
							Protocol:   corev1.ProtocolTCP,
							Port:       redisSpec.Frontends[0].ServicePort,
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
					pool, err = f.EdgeLBManager.GetPool(ctx, *redisSpec.Name)
					return err == nil, nil
				})
				Expect(err).NotTo(HaveOccurred(), "timed out while waiting for the edgelb api server to acknowledge the pool's creation")

				// Wait for Redis to be reachable.
				log.Debugf("waiting for the public ip for %q to be reported", kubernetes.Key(redisSvc))
				err = retry.WithTimeout(framework.DefaultRetryTimeout, framework.DefaultRetryInterval, func() (bool, error) {
					// Wait for the pool's public IP to be reported.
					ctx, fn := context.WithTimeout(context.Background(), framework.DefaultRetryTimeout)
					defer fn()
					publicIP, err = f.WaitForPublicIPForService(ctx, redisSvc)
					Expect(err).NotTo(HaveOccurred())
					Expect(publicIP).NotTo(BeEmpty())
					// Attempt to connect to Redis using the reported IP.
					addr := fmt.Sprintf("%s:%d", publicIP, *redisSpec.Frontends[0].Port)
					log.Debugf("attempting to connect to %q at %q", kubernetes.Key(redisSvc), addr)
					redisClient := redis.NewClient(&redis.Options{
						Addr: addr,
						DB:   0,
					})
					p, _ := redisClient.Ping().Result()
					return p == "PONG", nil
				})
				Expect(err).NotTo(HaveOccurred(), "timed out while attempting to ping redis")

				// Pause translation of the Service resource.
				// This will cause the service controller to stop processing this resource, and will prevent the pool from being re-created too soon.
				// This is required in order to prevent https://jira.mesosphere.com/browse/DCOS-46508 from happening due to the controller's resync period elapsing in the meantime.
				// NOTE: redisSvc must be re-read from the Kubernetes API in order to avoid "409 CONFLICT" errors when updating the resource.
				// These errors would otherwise happen, as the "service controller" has updated the resource's status in the meantime (in order to report the IPs).
				redisSvc, err = f.KubeClient.CoreV1().Services(redisSvc.Namespace).Get(redisSvc.Name, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred(), "failed to read an updated version of the service resource")
				redisSvc.Annotations[constants.DklbPaused] = "1"
				redisSvc, err = f.KubeClient.CoreV1().Services(redisSvc.Namespace).Update(redisSvc)
				Expect(err).NotTo(HaveOccurred(), "failed to set the type of %q to %q", kubernetes.Key(redisSvc), corev1.ServiceTypeNodePort)

				// Delete the pool.
				f.DeleteEdgeLBPool(pool)

				// Wait for Redis to NOT be reachable (meaning the pool and all load-balancer instances have been effectively deleted).
				err = retry.WithTimeout(framework.DefaultRetryTimeout, framework.DefaultRetryInterval, func() (bool, error) {
					redisClient := redis.NewClient(&redis.Options{
						Addr: fmt.Sprintf("%s:%d", publicIP, *redisSpec.Frontends[0].Port),
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
				redisSvc.Annotations[constants.DklbPaused] = "0"
				redisSvc, err = f.KubeClient.CoreV1().Services(redisSvc.Namespace).Update(redisSvc)
				Expect(err).NotTo(HaveOccurred(), "failed to set the type of %q to %q", kubernetes.Key(redisSvc), corev1.ServiceTypeNodePort)

				// Wait for the pool to be re-provisioned, making Redis reachable again.
				log.Debugf("waiting for the public ip for %q to be reported", kubernetes.Key(redisSvc))
				err = retry.WithTimeout(framework.DefaultRetryTimeout, framework.DefaultRetryInterval, func() (bool, error) {
					// Wait for the pool's public IP to be reported.
					ctx, fn := context.WithTimeout(context.Background(), framework.DefaultRetryTimeout)
					defer fn()
					publicIP, err = f.WaitForPublicIPForService(ctx, redisSvc)
					Expect(err).NotTo(HaveOccurred())
					Expect(publicIP).NotTo(BeEmpty())
					// Attempt to connect to Redis using the reported IP.
					addr := fmt.Sprintf("%s:%d", publicIP, *redisSpec.Frontends[0].Port)
					log.Debugf("attempting to connect to %q at %q", kubernetes.Key(redisSvc), addr)
					redisClient := redis.NewClient(&redis.Options{
						Addr: addr,
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
