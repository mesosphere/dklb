// +build e2e

package e2e_test

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/mesosphere/dcos-edge-lb/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	extsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/mesosphere/dklb/pkg/constants"
	translatorapi "github.com/mesosphere/dklb/pkg/translator/api"
	"github.com/mesosphere/dklb/pkg/util/kubernetes"
	"github.com/mesosphere/dklb/pkg/util/retry"

	"github.com/mesosphere/dklb/pkg/util/pointers"
	"github.com/mesosphere/dklb/test/e2e/framework"
)

var _ = Describe("Ingress", func() {
	Context("not annotated for provisioning by EdgeLB", func() {
		It("is ignored by the admission webhook [HTTP] [Admission]", func() {
			f.WithTemporaryNamespace(func(namespace *corev1.Namespace) {
				_, err := f.CreateIngress(namespace.Name, "", func(ingress *extsv1beta1.Ingress) {
					// Use an invalid value for "kubernetes.dcos.io/dklb-config" (i.e. one for which ".size" is negative).
					// The resulting Ingress resource would be be invalid, and hence an error would be reported, should it be annotated for provisioning by EdgeLB.
					_ = translatorapi.SetIngressEdgeLBPoolSpec(ingress, &translatorapi.IngressEdgeLBPoolSpec{
						BaseEdgeLBPoolSpec: translatorapi.BaseEdgeLBPoolSpec{
							Size: pointers.NewInt32(-1),
						},
					})
					// Use a randomly-generated name for the Ingress resource.
					ingress.GenerateName = fmt.Sprintf("%s-", namespace.Name)
					// Define a default backend so that the Ingress resource is valid.
					ingress.Spec.Backend = &extsv1beta1.IngressBackend{
						ServiceName: "foo",
						ServicePort: intstr.FromInt(80),
					}
				})
				// Make sure that no error occurred (meaning the admission webhook has ignored the Ingress resource).
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Context("annotated for provisioning by EdgeLB", func() {
		It("created without the \"kubernetes.dcos.io/dklb-config\" annotation is mutated with a default configuration [TCP] [Admission]", func() {
			f.WithTemporaryNamespace(func(namespace *corev1.Namespace) {
				var (
					err     error
					objSpec translatorapi.IngressEdgeLBPoolSpec
					rawSpec string
					ing     *extsv1beta1.Ingress
				)

				// Create an Ingress resource annotated for provisioning with EdgeLB but without the "kubernetes.dcos.io/dklb-config" annotation.
				ing, err = f.CreateEdgeLBIngress(namespace.Name, "bare-ingress", func(ingress *extsv1beta1.Ingress) {
					ingress.Annotations = map[string]string{
						constants.DklbPaused: strconv.FormatBool(true),
					}
					ingress.Spec.Backend = &extsv1beta1.IngressBackend{
						ServiceName: "foo",
						ServicePort: intstr.FromString("bar"),
					}
				})
				Expect(err).NotTo(HaveOccurred(), "failed to create test ingress")

				// Make sure that the Ingress resource has been mutated with a non-empty value for the "kubernetes.dcos.io/dklb-config" annotation.
				rawSpec = ing.Annotations[constants.DklbConfigAnnotationKey]
				Expect(rawSpec).NotTo(BeEmpty(), "the \"kubernetes.dcos.io/dklb-config\" annotation is absent or empty")
				// Make sure that the value of the "kubernetes.dcos.io/dklb-config" annotation can be unmarshaled into an IngressEdgeLBPoolSpec object.
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
				Expect(*objSpec.Frontends.HTTP.Port).To(Equal(translatorapi.DefaultEdgeLBPoolPort))
			})
		})

		It("created with an invalid configuration is rejected by the admission webhook [HTTP] [Admission]", func() {
			f.WithTemporaryNamespace(func(namespace *corev1.Namespace) {
				tests := []struct {
					description               string
					fn                        framework.IngressCustomizer
					expectedErrorMessageRegex string
				}{
					{
						description: "\"kubernetes.dcos.io/dklb-config\" cannot be parsed as yaml",
						fn: func(ingress *extsv1beta1.Ingress) {
							ingress.Annotations[constants.DklbConfigAnnotationKey] = "invalid: yaml: str"
						},
						expectedErrorMessageRegex: "failed to parse the value of \"kubernetes.dcos.io/dklb-config\" as a configuration object",
					},
					{
						description: "\"kubernetes.dcos.io/dklb-config\" specifies an invalid edgelb pool name",
						fn: func(ingress *extsv1beta1.Ingress) {
							_ = translatorapi.SetIngressEdgeLBPoolSpec(ingress, &translatorapi.IngressEdgeLBPoolSpec{
								BaseEdgeLBPoolSpec: translatorapi.BaseEdgeLBPoolSpec{
									Name: pointers.NewString("__foo__"),
								},
							})
						},
						expectedErrorMessageRegex: "\"__foo__\" is not a valid edgelb pool name",
					},
					{
						description: "\"kubernetes.dcos.io/dklb-config\" specifies an invalid edgelb pool network",
						fn: func(ingress *extsv1beta1.Ingress) {
							_ = translatorapi.SetIngressEdgeLBPoolSpec(ingress, &translatorapi.IngressEdgeLBPoolSpec{
								BaseEdgeLBPoolSpec: translatorapi.BaseEdgeLBPoolSpec{
									Network: pointers.NewString("dcos"),
								},
							})
						},
						expectedErrorMessageRegex: "cannot join a virtual network when the pool's role is \"slave_public\"",
					},
					{
						description: "\"kubernetes.dcos.io/dklb-config\" specifies an invalid edgelb pool cpu request",
						fn: func(ingress *extsv1beta1.Ingress) {
							_ = translatorapi.SetIngressEdgeLBPoolSpec(ingress, &translatorapi.IngressEdgeLBPoolSpec{
								BaseEdgeLBPoolSpec: translatorapi.BaseEdgeLBPoolSpec{
									CPUs: pointers.NewFloat64(-0.1),
								},
							})
						},
						expectedErrorMessageRegex: "-0.100000 is not a valid cpu request",
					},
					{
						description: "\"kubernetes.dcos.io/dklb-config\" specifies an invalid edgelb pool memory request",
						fn: func(ingress *extsv1beta1.Ingress) {
							_ = translatorapi.SetIngressEdgeLBPoolSpec(ingress, &translatorapi.IngressEdgeLBPoolSpec{
								BaseEdgeLBPoolSpec: translatorapi.BaseEdgeLBPoolSpec{
									Memory: pointers.NewInt32(-256),
								},
							})
						},
						expectedErrorMessageRegex: "-256 is not a valid memory request",
					},
					{
						description: "\"kubernetes.dcos.io/dklb-config\" specifies an invalid edgelb pool size request",
						fn: func(ingress *extsv1beta1.Ingress) {
							_ = translatorapi.SetIngressEdgeLBPoolSpec(ingress, &translatorapi.IngressEdgeLBPoolSpec{
								BaseEdgeLBPoolSpec: translatorapi.BaseEdgeLBPoolSpec{
									Size: pointers.NewInt32(-1),
								},
							})
						},
						expectedErrorMessageRegex: "-1 is not a valid size request",
					},
					{
						description: "\"kubernetes.dcos.io/dklb-config\" specifies an invalid edgelb pool creation strategy",
						fn: func(ingress *extsv1beta1.Ingress) {
							strategy := translatorapi.EdgeLBPoolCreationStrategy("InvalidStrategy")
							_ = translatorapi.SetIngressEdgeLBPoolSpec(ingress, &translatorapi.IngressEdgeLBPoolSpec{
								BaseEdgeLBPoolSpec: translatorapi.BaseEdgeLBPoolSpec{
									Strategies: &translatorapi.EdgeLBPoolManagementStrategies{
										Creation: &strategy,
									},
								},
							})
						},
						expectedErrorMessageRegex: "failed to parse \"InvalidStrategy\" as an edgelb pool creation strategy",
					},
					{
						description: "\"kubernetes.dcos.io/dklb-config\" specifies an invalid edgelb frontend port",
						fn: func(ingress *extsv1beta1.Ingress) {
							_ = translatorapi.SetIngressEdgeLBPoolSpec(ingress, &translatorapi.IngressEdgeLBPoolSpec{
								Frontends: &translatorapi.IngressEdgeLBPoolFrontendsSpec{
									HTTP: &translatorapi.IngressEdgeLBPoolHTTPFrontendSpec{
										Port: pointers.NewInt32(123456),
									},
								},
							})
						},
						expectedErrorMessageRegex: "123456 is not a valid port number",
					},
				}
				for _, test := range tests {
					log.Infof("test case: %s", test.description)
					_, err := f.CreateEdgeLBIngress(namespace.Name, "foo", test.fn)
					Expect(err).To(HaveOccurred())
					statusErr, ok := err.(*errors.StatusError)
					Expect(ok).To(BeTrue())
					Expect(statusErr.ErrStatus.Message).To(MatchRegexp(test.expectedErrorMessageRegex))
				}
			})
		})

		It("created with a valid configuration and updated to an invalid one is rejected by the admission webhook [HTTP] [Admission]", func() {
			f.WithTemporaryNamespace(func(namespace *corev1.Namespace) {
				var (
					err            error
					initialIngress *extsv1beta1.Ingress
				)

				// Create an Ingress resource annotated for provisioning with EdgeLB and containing a valid EdgeLB pool specification.
				initialIngress, err = f.CreateEdgeLBIngress(namespace.Name, "http-echo", func(ingress *extsv1beta1.Ingress) {
					_ = translatorapi.SetIngressEdgeLBPoolSpec(ingress, &translatorapi.IngressEdgeLBPoolSpec{
						BaseEdgeLBPoolSpec: translatorapi.BaseEdgeLBPoolSpec{
							Name: pointers.NewString(namespace.Name),
							Role: pointers.NewString(constants.EdgeLBRolePrivate),
						},
					})
					// Request for translation to be paused so that no EdgeLB pool is actually created.
					ingress.Annotations[constants.DklbPaused] = strconv.FormatBool(true)
					// Define a default backend so that the Ingress resource can actually be created.
					ingress.Spec.Backend = &extsv1beta1.IngressBackend{
						ServiceName: "foo",
						ServicePort: intstr.FromString("http"),
					}
				})
				Expect(err).NotTo(HaveOccurred(), "failed to create test ingress")

				// Attempt to perform some forbidden updates to the target EdgeLB pool's specification.
				tests := []struct {
					description               string
					fn                        framework.IngressEdgeLBPoolSpecCustomizer
					expecterErrorMessageRegex string
				}{
					{
						description: "update the target edgelb pool's name",
						fn: func(spec *translatorapi.IngressEdgeLBPoolSpec) {
							spec.Name = pointers.NewString("new-name")
						},
						expecterErrorMessageRegex: "the name of the target edgelb pool cannot be changed",
					},
					{
						description: "update the target edgelb pool's role",
						fn: func(spec *translatorapi.IngressEdgeLBPoolSpec) {
							spec.Role = pointers.NewString("new-role")
						},
						expecterErrorMessageRegex: "the role of the target edgelb pool cannot be changed",
					},
					{
						description: "update the target edgelb pool's virtual network",
						fn: func(spec *translatorapi.IngressEdgeLBPoolSpec) {
							spec.Network = pointers.NewString("new-network")
						},
						expecterErrorMessageRegex: "the virtual network of the target edgelb pool cannot be changed",
					},
				}
				for _, test := range tests {
					log.Infof("test case: %s", test.description)
					// Update the initial Ingress resource with the desired EdgeLB pool specification.
					_, err = f.UpdateIngressEdgeLBPoolSpec(initialIngress.DeepCopy(), test.fn)
					// Make sure that the expected error has occurred.
					Expect(err).To(HaveOccurred())
					statusErr, ok := err.(*errors.StatusError)
					Expect(ok).To(BeTrue())
					Expect(statusErr.ErrStatus.Message).To(MatchRegexp(test.expecterErrorMessageRegex))
				}
			})
		})

		It("is correctly provisioned by EdgeLB [HTTP] [Public]", func() {
			f.WithTemporaryNamespace(func(namespace *corev1.Namespace) {
				var (
					echoPod1     *corev1.Pod
					echoPod2     *corev1.Pod
					echoPod3     *corev1.Pod
					echoPod4     *corev1.Pod
					echoSvc1     *corev1.Service
					echoSvc2     *corev1.Service
					echoSvc3     *corev1.Service
					echoSvc4     *corev1.Service
					err          error
					httpEchoSpec translatorapi.IngressEdgeLBPoolSpec
					ingress      *extsv1beta1.Ingress
					pool         *models.V2Pool
					publicIP     string
				)

				// Create the first "echo" pod.
				echoPod1, err = f.CreateEchoPod(namespace.Name, "http-echo-1")
				Expect(err).NotTo(HaveOccurred(), "failed to create echo pod")
				// Create the first "echo" service.
				echoSvc1, err = f.CreateServiceForEchoPod(echoPod1)
				Expect(err).NotTo(HaveOccurred(), "failed to create service for echo pod %q", kubernetes.Key(echoPod1))

				// Create the second "echo" pod.
				echoPod2, err = f.CreateEchoPod(namespace.Name, "http-echo-2")
				Expect(err).NotTo(HaveOccurred(), "failed to create echo pod")
				// Create the second "echo" service.
				echoSvc2, err = f.CreateServiceForEchoPod(echoPod2)
				Expect(err).NotTo(HaveOccurred(), "failed to create service for echo pod %q", kubernetes.Key(echoPod1))

				// Create the third "echo" pod.
				echoPod3, err = f.CreateEchoPod(namespace.Name, "http-echo-3")
				Expect(err).NotTo(HaveOccurred(), "failed to create echo pod")
				// Create the third "echo" service.
				echoSvc3, err = f.CreateServiceForEchoPod(echoPod3)
				Expect(err).NotTo(HaveOccurred(), "failed to create service for echo pod %q", kubernetes.Key(echoPod1))

				// Create the fourth "echo" pod.
				echoPod4, err = f.CreateEchoPod(namespace.Name, "http-echo-4")
				Expect(err).NotTo(HaveOccurred(), "failed to create echo pod")
				// Create the fourth "echo" service.
				echoSvc4, err = f.CreateServiceForEchoPod(echoPod4)
				Expect(err).NotTo(HaveOccurred(), "failed to create service for echo pod %q", kubernetes.Key(echoPod1))

				// Create an object holding the target EdgeLB pool's specification.
				httpEchoSpec = translatorapi.IngressEdgeLBPoolSpec{
					BaseEdgeLBPoolSpec: translatorapi.BaseEdgeLBPoolSpec{
						// Request for the EdgeLB pool to be called "<namespace-name>".
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
					Frontends: &translatorapi.IngressEdgeLBPoolFrontendsSpec{
						HTTP: &translatorapi.IngressEdgeLBPoolHTTPFrontendSpec{
							// Request for the EdgeLB pool to expose the ingress at 18080.
							Port: pointers.NewInt32(18080),
						},
					},
				}

				// Create an Ingress resource targeting the services above, annotating it to be provisioned by EdgeLB.
				// The following rules are defined on the Ingress resource:
				// * Requests for the "http-echo-4.com" host are (ALL) directed towards "http-echo-4".
				// * Requests for the "http-echo-3.com" host and the "/bar(/.*)?" path are directed towards "http-echo-3".
				// * Requests for the "http-echo-3.com" host and any other path are directed towards "http-echo-2".
				// * Unmatched requests are directed towards "http-echo-1".
				ingress, err = f.CreateEdgeLBIngress(namespace.Name, "http-echo", func(ingress *extsv1beta1.Ingress) {
					// Use "httpEchoSpec" as the specification for the target EdgeLB pool.
					_ = translatorapi.SetIngressEdgeLBPoolSpec(ingress, &httpEchoSpec)
					// Use "echoSvc1" as the default backend.
					ingress.Spec.Backend = &extsv1beta1.IngressBackend{
						ServiceName: echoSvc1.Name,
						ServicePort: intstr.FromString(echoSvc1.Spec.Ports[0].Name),
					}
					// Setup rules as described above.
					ingress.Spec.Rules = []extsv1beta1.IngressRule{
						{
							Host: "http-echo-3.com",
							IngressRuleValue: extsv1beta1.IngressRuleValue{
								HTTP: &extsv1beta1.HTTPIngressRuleValue{
									Paths: []extsv1beta1.HTTPIngressPath{
										{
											Backend: extsv1beta1.IngressBackend{
												ServiceName: echoSvc2.Name,
												ServicePort: intstr.FromInt(int(echoSvc2.Spec.Ports[0].Port)),
											},
										},
										{
											Path: "/bar(/.*)?",
											Backend: extsv1beta1.IngressBackend{
												ServiceName: echoSvc3.Name,
												ServicePort: intstr.FromString(echoSvc3.Spec.Ports[0].Name),
											},
										},
									},
								},
							},
						},
						{
							Host: "http-echo-4.com",
							IngressRuleValue: extsv1beta1.IngressRuleValue{
								HTTP: &extsv1beta1.HTTPIngressRuleValue{
									Paths: []extsv1beta1.HTTPIngressPath{
										{
											Backend: extsv1beta1.IngressBackend{
												ServiceName: echoSvc4.Name,
												ServicePort: intstr.FromString(echoSvc4.Spec.Ports[0].Name),
											},
										},
									},
								},
							},
						},
					}
				})
				Expect(err).NotTo(HaveOccurred(), "failed to create ingress")

				// Wait for EdgeLB to acknowledge the EdgeLB pool's creation.
				err = retry.WithTimeout(framework.DefaultRetryTimeout, framework.DefaultRetryInterval, func() (bool, error) {
					ctx, fn := context.WithTimeout(context.Background(), framework.DefaultRetryInterval/2)
					defer fn()
					pool, err = f.EdgeLBManager.GetPool(ctx, *httpEchoSpec.Name)
					return err == nil, nil
				})
				Expect(err).NotTo(HaveOccurred(), "timed out while waiting for the edgelb api server to acknowledge the pool's creation")

				// Make sure the pool is reporting the requested configuration.
				Expect(pool.Name).To(Equal(*httpEchoSpec.Name))
				Expect(pool.Role).To(Equal(*httpEchoSpec.Role))
				Expect(pool.Cpus).To(Equal(*httpEchoSpec.CPUs))
				Expect(pool.Mem).To(Equal(*httpEchoSpec.Memory))
				Expect(*pool.Count).To(Equal(*httpEchoSpec.Size))

				// Wait for the Ingress to be reachable.
				log.Debugf("waiting for the public ip for %q to be reported", kubernetes.Key(ingress))
				err = retry.WithTimeout(framework.DefaultRetryTimeout, framework.DefaultRetryInterval, func() (bool, error) {
					// Wait for the pool's public IP to be reported.
					ctx, fn := context.WithTimeout(context.Background(), framework.DefaultRetryTimeout)
					defer fn()
					publicIP, err = f.WaitForPublicIPForIngress(ctx, ingress)
					Expect(err).NotTo(HaveOccurred())
					Expect(publicIP).NotTo(BeEmpty())
					// Attempt to connect to the ingress using the reported IP.
					addr := fmt.Sprintf("http://%s:%d", publicIP, *httpEchoSpec.Frontends.HTTP.Port)
					log.Debugf("attempting to connect to %q at %q", kubernetes.Key(ingress), addr)
					r, err := f.HTTPClient.Get(addr)
					if err != nil {
						log.Debugf("waiting for the ingress to be reachable at %q", addr)
						return false, nil
					}
					log.Debugf("the ingress is reachable at %q", addr)
					return r.StatusCode == 200, nil
				})
				Expect(err).NotTo(HaveOccurred(), "timed out while waiting for the ingress to be reachable")

				// Make sure that requests are directed towards the appropriate backend and contain the expected headers.
				tests := []struct {
					host        string
					path        string
					expectedPod string
				}{
					// Test that requests whose path starts with "/bar" but whose host is "http-echo-4.com" are directed towards "http-echo-4".
					{
						host:        "http-echo-4.com",
						path:        "/bar",
						expectedPod: echoPod4.Name,
					},
					// Test that requests whose path starts with "/foo" and whose host is "http-echo-3.com" are directed towards "http-echo-2".
					{
						host:        "http-echo-3.com",
						path:        "/foo",
						expectedPod: echoPod2.Name,
					},
					// Test that requests whose path starts with "/bar" and whose host is "http-echo-3.com" are directed towards "http-echo-3".
					{
						host:        "http-echo-3.com",
						path:        "/bar",
						expectedPod: echoPod3.Name,
					},
					// Test that unmatched requests are directed towards "http-echo-1" (the default backend).
					{
						host:        publicIP,
						path:        "/foo",
						expectedPod: echoPod1.Name,
					},
				}
				for _, test := range tests {
					for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
						log.Debugf("test case: %s request to host %q and path %q is directed towards %q", method, test.host, test.path, test.expectedPod)
						res, err := f.EchoRequest(method, publicIP, *httpEchoSpec.Frontends.HTTP.Port, test.path, map[string]string{
							"Host": test.host,
						})
						Expect(err).NotTo(HaveOccurred(), "failed to perform http request")
						Expect(res.Host).To(Equal(test.host), "the reported host header doesn't match the expectation")
						Expect(res.Method).To(Equal(method), "the reported method doesn't match the expectation")
						Expect(res.K8sEnv.Namespace).To(Equal(namespace.Name), "the reported namespace doesn't match the expectation")
						Expect(res.K8sEnv.Pod).To(Equal(test.expectedPod), "the reported pod doesn't match the expectation")
						Expect(res.URI).To(Equal(test.path), "the reported path doesn't match the expectation")
						Expect(res.XForwardedForContains(f.ExternalIP)).To(BeTrue(), "external ip missing from the x-forwarded-for header")
					}
				}

				// Manually delete the Ingress resource now so that the target EdgeLB pool isn't possibly left dangling after namespace deletion.
				err = f.KubeClient.ExtensionsV1beta1().Ingresses(ingress.Namespace).Delete(ingress.Name, metav1.NewDeleteOptions(0))
				Expect(err).NotTo(HaveOccurred(), "failed to delete ingress %q", kubernetes.Key(ingress))
			})
		})

		It("can share a pool with an Ingress resource in a different namespace [HTTP] [Public]", func() {
			// Create two temporary namespaces for the test.
			f.WithTemporaryNamespace(func(namespace1 *corev1.Namespace) {
				f.WithTemporaryNamespace(func(namespace2 *corev1.Namespace) {
					var (
						echoPod1     *corev1.Pod
						echoPod2     *corev1.Pod
						echoSvc1     *corev1.Service
						echoSvc2     *corev1.Service
						err          error
						ingress1     *extsv1beta1.Ingress
						ingress1Spec translatorapi.IngressEdgeLBPoolSpec
						ingress2     *extsv1beta1.Ingress
						ingress2Spec translatorapi.IngressEdgeLBPoolSpec
						pool         *models.V2Pool
						publicIP     string
					)

					// Create the first "echo" pod.
					echoPod1, err = f.CreateEchoPod(namespace1.Name, "http-echo-1")
					Expect(err).NotTo(HaveOccurred(), "failed to create echo pod")
					// Create the first "echo" service.
					echoSvc1, err = f.CreateServiceForEchoPod(echoPod1)
					Expect(err).NotTo(HaveOccurred(), "failed to create service for echo pod %q", kubernetes.Key(echoPod1))

					// Create an object holding the target EdgeLB pool's specification for the "ingress1" Ingress.
					ingress1Spec = translatorapi.IngressEdgeLBPoolSpec{
						BaseEdgeLBPoolSpec: translatorapi.BaseEdgeLBPoolSpec{
							Name: pointers.NewString(namespace1.Name),
							Role: pointers.NewString(constants.EdgeLBRolePublic),
						},
						Frontends: &translatorapi.IngressEdgeLBPoolFrontendsSpec{
							HTTP: &translatorapi.IngressEdgeLBPoolHTTPFrontendSpec{
								Port: pointers.NewInt32(18080),
							},
						},
					}

					// Create an Ingress resource targeting the "http-echo-1" service above, annotating it to be provisioned by EdgeLB.
					// The Ingress is configured to direct all traffic under "/foo" to "http-echo-1".
					ingress1, err = f.CreateEdgeLBIngress(namespace1.Name, "http-echo", func(ingress *extsv1beta1.Ingress) {
						_ = translatorapi.SetIngressEdgeLBPoolSpec(ingress, &ingress1Spec)
						ingress.Spec.Rules = []extsv1beta1.IngressRule{
							{
								IngressRuleValue: extsv1beta1.IngressRuleValue{
									HTTP: &extsv1beta1.HTTPIngressRuleValue{
										Paths: []extsv1beta1.HTTPIngressPath{
											{
												Path: "/foo(/.*)?",
												Backend: extsv1beta1.IngressBackend{
													ServiceName: echoSvc1.Name,
													ServicePort: intstr.FromInt(int(echoSvc1.Spec.Ports[0].Port)),
												},
											},
										},
									},
								},
							},
						}
					})
					Expect(err).NotTo(HaveOccurred(), "failed to create ingress")

					// Wait for EdgeLB to acknowledge the pool's creation.
					err = retry.WithTimeout(framework.DefaultRetryTimeout, framework.DefaultRetryInterval, func() (bool, error) {
						ctx, fn := context.WithTimeout(context.Background(), framework.DefaultRetryInterval/2)
						defer fn()
						pool, err = f.EdgeLBManager.GetPool(ctx, *ingress1Spec.Name)
						return err == nil, nil
					})
					Expect(err).NotTo(HaveOccurred(), "timed out while waiting for the edgelb api server to acknowledge the pool's creation")

					// Wait for the Ingress to be reachable.
					log.Debugf("waiting for the public ip for %q to be reported", kubernetes.Key(ingress1))
					err = retry.WithTimeout(framework.DefaultRetryTimeout, framework.DefaultRetryInterval, func() (bool, error) {
						// Wait for the pool's public IP to be reported.
						ctx, fn := context.WithTimeout(context.Background(), framework.DefaultRetryTimeout)
						defer fn()
						publicIP, err = f.WaitForPublicIPForIngress(ctx, ingress1)
						Expect(err).NotTo(HaveOccurred())
						Expect(publicIP).NotTo(BeEmpty())
						// Attempt to connect to the ingress using the reported IP.
						addr := fmt.Sprintf("http://%s:%d", publicIP, *ingress1Spec.Frontends.HTTP.Port)
						log.Debugf("attempting to connect to %q at %q", kubernetes.Key(ingress1), addr)
						url := fmt.Sprintf("%s/foo", addr)
						r, err := f.HTTPClient.Get(url)
						if err != nil {
							log.Debugf("waiting for the ingress to be reachable at %s", url)
							return false, nil
						}
						log.Debugf("the ingress is reachable at %s", url)
						return r.StatusCode == 200, nil
					})
					Expect(err).NotTo(HaveOccurred(), "timed out while waiting for the ingress to be reachable")

					// Create the second "echo" pod.
					echoPod2, err = f.CreateEchoPod(namespace2.Name, "http-echo-2")
					Expect(err).NotTo(HaveOccurred(), "failed to create echo pod")
					// Create the second "echo" service.
					echoSvc2, err = f.CreateServiceForEchoPod(echoPod2)
					Expect(err).NotTo(HaveOccurred(), "failed to create service for echo pod %q", kubernetes.Key(echoPod2))

					// Create an object holding the target EdgeLB pool's specification for the "ingress2" Ingress.
					ingress2Spec = translatorapi.IngressEdgeLBPoolSpec{
						BaseEdgeLBPoolSpec: translatorapi.BaseEdgeLBPoolSpec{
							Name: pointers.NewString(pool.Name),
							Role: pointers.NewString(constants.EdgeLBRolePublic),
						},
						Frontends: &translatorapi.IngressEdgeLBPoolFrontendsSpec{
							HTTP: &translatorapi.IngressEdgeLBPoolHTTPFrontendSpec{
								// Request for the EdgeLB pool to expose this ingress at 18080.
								Port: pointers.NewInt32(28080),
							},
						},
					}

					// Create an Ingress resource targeting the "http-echo-2" service above, annotating it to be provisioned by EdgeLB.
					// The Ingress is configured to direct all traffic under "/foo" to "http-echo-2".
					ingress2, err = f.CreateEdgeLBIngress(namespace2.Name, "http-echo", func(ingress *extsv1beta1.Ingress) {
						_ = translatorapi.SetIngressEdgeLBPoolSpec(ingress, &ingress2Spec)
						ingress.Spec.Rules = []extsv1beta1.IngressRule{
							{
								IngressRuleValue: extsv1beta1.IngressRuleValue{
									HTTP: &extsv1beta1.HTTPIngressRuleValue{
										Paths: []extsv1beta1.HTTPIngressPath{
											{
												Path: "/foo(/.*)?",
												Backend: extsv1beta1.IngressBackend{
													ServiceName: echoSvc2.Name,
													ServicePort: intstr.FromInt(int(echoSvc2.Spec.Ports[0].Port)),
												},
											},
										},
									},
								},
							},
						}
					})
					Expect(err).NotTo(HaveOccurred(), "failed to create ingress")

					// Wait for the Ingress to be reachable.
					log.Debugf("waiting for the public ip for %q to be reported", kubernetes.Key(ingress2))
					err = retry.WithTimeout(framework.DefaultRetryTimeout, framework.DefaultRetryInterval, func() (bool, error) {
						// Wait for the pool's public IP to be reported.
						ctx, fn := context.WithTimeout(context.Background(), framework.DefaultRetryTimeout)
						defer fn()
						publicIP, err = f.WaitForPublicIPForIngress(ctx, ingress2)
						Expect(err).NotTo(HaveOccurred())
						Expect(publicIP).NotTo(BeEmpty())
						// Attempt to connect to the ingress using the reported IP.
						addr := fmt.Sprintf("http://%s:%d", publicIP, *ingress2Spec.Frontends.HTTP.Port)
						log.Debugf("attempting to connect to %q at %q", kubernetes.Key(ingress2), addr)
						url := fmt.Sprintf("%s/foo", addr)
						r, err := f.HTTPClient.Get(url)
						if err != nil {
							log.Debugf("waiting for the ingress to be reachable at %s", url)
							return false, nil
						}
						log.Debugf("the ingress is reachable at %s", url)
						return r.StatusCode == 200, nil
					})
					Expect(err).NotTo(HaveOccurred(), "timed out while waiting for the ingress to be reachable")

					// Make sure that both Ingress resources are reachable and directing requests towards the expected backend.
					tests := []struct {
						port              int32
						path              string
						expectedNamespace string
						expectedPod       string
					}{
						{
							port:              *ingress1Spec.Frontends.HTTP.Port,
							path:              "/foo",
							expectedNamespace: echoPod1.Namespace,
							expectedPod:       echoPod1.Name,
						},
						{
							port:              *ingress2Spec.Frontends.HTTP.Port,
							path:              "/foo",
							expectedNamespace: echoPod2.Namespace,
							expectedPod:       echoPod2.Name,
						},
					}
					for _, test := range tests {
						log.Debugf("test case: request to port %d and path %q is directed towards %q", test.port, test.path, test.expectedPod)
						res, err := f.EchoRequest("GET", publicIP, test.port, test.path, nil)
						Expect(err).NotTo(HaveOccurred(), "failed to perform http request")
						Expect(res.K8sEnv.Namespace).To(Equal(test.expectedNamespace), "the reported namespace doesn't match the expectation")
						Expect(res.K8sEnv.Pod).To(Equal(test.expectedPod), "the reported pod doesn't match the expectation")
						Expect(res.URI).To(Equal(test.path), "the reported path doesn't match the expectation")
						Expect(res.XForwardedForContains(f.ExternalIP)).To(BeTrue(), "external ip missing from the x-forwarded-for header")
					}

					// Make sure there is a single EdgeLB pool.
					ctx3, fn3 := context.WithTimeout(context.Background(), framework.DefaultEdgeLBOperationTimeout)
					defer fn3()
					pools, err := f.EdgeLBManager.GetPools(ctx3)
					Expect(err).NotTo(HaveOccurred(), "failed to list edgelb pools")
					Expect(len(pools)).To(Equal(1), "expecting a single edgelb pool, found %d", len(pools))

					// Manually delete the Ingress resources now in order to prevent the EdgeLB pool from being re-created during namespace deletion.
					err = f.KubeClient.ExtensionsV1beta1().Ingresses(ingress1.Namespace).Delete(ingress1.Name, metav1.NewDeleteOptions(0))
					Expect(err).NotTo(HaveOccurred(), "failed to delete ingress %q", kubernetes.Key(ingress1))
					err = f.KubeClient.ExtensionsV1beta1().Ingresses(ingress2.Namespace).Delete(ingress1.Name, metav1.NewDeleteOptions(0))
					Expect(err).NotTo(HaveOccurred(), "failed to delete ingress %q", kubernetes.Key(ingress2))
				})
			})
		})

		It("uses dklb as its default backend whenever one is not specified or a service is missing [HTTP] [Public]", func() {
			// Create a temporary namespace for the test.
			f.WithTemporaryNamespace(func(namespace *corev1.Namespace) {
				var (
					echoPod1 *corev1.Pod
					echoSpec translatorapi.IngressEdgeLBPoolSpec
					echoSvc1 *corev1.Service
					err      error
					ingress  *extsv1beta1.Ingress
					publicIP string
				)

				// Create the first "echo" pod.
				echoPod1, err = f.CreateEchoPod(namespace.Name, "http-echo-1")
				Expect(err).NotTo(HaveOccurred(), "failed to create echo pod")
				// Create the first "echo" service.
				echoSvc1, err = f.CreateServiceForEchoPod(echoPod1)
				Expect(err).NotTo(HaveOccurred(), "failed to create service for echo pod %q", kubernetes.Key(echoPod1))

				// Create an object holding the target EdgeLB pool's specification for the "http-echo" Ingress.
				echoSpec = translatorapi.IngressEdgeLBPoolSpec{
					BaseEdgeLBPoolSpec: translatorapi.BaseEdgeLBPoolSpec{
						Name:   pointers.NewString(namespace.Name),
						Role:   pointers.NewString(constants.EdgeLBRolePublic),
						CPUs:   pointers.NewFloat64(0.2),
						Memory: pointers.NewInt32(256),
						Size:   pointers.NewInt32(1),
					},
				}

				// Create an Ingress resource targeting the service above, annotating it to be provisioned by EdgeLB.
				ingress, err = f.CreateEdgeLBIngress(namespace.Name, "http-echo", func(ingress *extsv1beta1.Ingress) {
					_ = translatorapi.SetIngressEdgeLBPoolSpec(ingress, &echoSpec)
					ingress.Spec.Rules = []extsv1beta1.IngressRule{
						{
							IngressRuleValue: extsv1beta1.IngressRuleValue{
								HTTP: &extsv1beta1.HTTPIngressRuleValue{
									Paths: []extsv1beta1.HTTPIngressPath{
										{
											Path: "/foo(/.*)?",
											Backend: extsv1beta1.IngressBackend{
												ServiceName: echoSvc1.Name,
												ServicePort: intstr.FromInt(int(echoSvc1.Spec.Ports[0].Port)),
											},
										},
										{
											Path: "/bar(/.*)?",
											Backend: extsv1beta1.IngressBackend{
												ServiceName: "missing-service",
												ServicePort: intstr.FromString("http"),
											},
										},
									},
								},
							},
						},
					}
				})
				Expect(err).NotTo(HaveOccurred(), "failed to create ingress")

				// Wait for EdgeLB to acknowledge the pool's creation.
				err = retry.WithTimeout(framework.DefaultRetryTimeout, framework.DefaultRetryInterval, func() (bool, error) {
					ctx, fn := context.WithTimeout(context.Background(), framework.DefaultRetryInterval/2)
					defer fn()
					_, err = f.EdgeLBManager.GetPool(ctx, *echoSpec.Name)
					return err == nil, nil
				})
				Expect(err).NotTo(HaveOccurred(), "timed out while waiting for the edgelb api server to acknowledge the pool's creation")

				// Wait for the Ingress to respond with the default backend.
				log.Debugf("waiting for the public ip for %q to be reported", kubernetes.Key(ingress))
				err = retry.WithTimeout(framework.DefaultRetryTimeout, framework.DefaultRetryInterval, func() (bool, error) {
					// Wait for the pool's public IP to be reported.
					ctx, fn := context.WithTimeout(context.Background(), framework.DefaultRetryTimeout)
					defer fn()
					publicIP, err = f.WaitForPublicIPForIngress(ctx, ingress)
					Expect(err).NotTo(HaveOccurred())
					Expect(publicIP).NotTo(BeEmpty())
					// Attempt to connect to the ingress using the reported IP.
					log.Debugf("attempting to connect to %q at %q", kubernetes.Key(ingress), publicIP)
					r, err := f.HTTPClient.Get(fmt.Sprintf("http://%s/foo", publicIP))
					if err != nil {
						log.Debugf("waiting for the ingress to be reachable at %s", publicIP)
						return false, nil
					}
					log.Debugf("the ingress is reachable at %s", publicIP)
					return r.StatusCode == 200, nil
				})
				Expect(err).NotTo(HaveOccurred(), "timed out while waiting for the ingress to be reachable")

				// Make sure that requests are directed towards the expected backend.
				tests := []struct {
					description        string
					path               string
					expectedStatusCode int
					expectedBodyRegex  string
				}{
					// Test that requests made to "/foo" are directed towards "http-echo-1".
					{
						description:        "%s request to path /foo is directed towards http-echo-1",
						path:               "/foo",
						expectedStatusCode: 200,
						expectedBodyRegex:  "http-echo-1",
					},
					// Test that requests made to "/" are directed towards "dklb".
					{
						description:        "%s request to path / is directed towards \"dklb\"",
						path:               "/",
						expectedStatusCode: 503,
						expectedBodyRegex:  "No backend is available to service this request.",
					},
					// Test that requests made to "/bar" are directed towards "dklb".
					{
						description:        "%s request to path /bar is directed towards \"dklb\"",
						path:               "/bar",
						expectedStatusCode: 503,
						expectedBodyRegex:  "No backend is available to service this request.",
					},
				}
				for _, test := range tests {
					for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
						log.Debugf("test case: %s", fmt.Sprintf(test.description, method))
						status, body, err := f.Request(method, publicIP, test.path)
						Expect(err).NotTo(HaveOccurred(), "failed to perform http request")
						Expect(status).To(Equal(test.expectedStatusCode), "the response's status code doesn't match the expectation")
						Expect(body).To(MatchRegexp(test.expectedBodyRegex), "the response's body doesn't match the expectations")
					}
				}

				// Manually delete the Ingress resource now in order to prevent the EdgeLB pool from being re-created during namespace deletion.
				err = f.KubeClient.ExtensionsV1beta1().Ingresses(ingress.Namespace).Delete(ingress.Name, metav1.NewDeleteOptions(0))
				Expect(err).NotTo(HaveOccurred(), "failed to delete ingress %q", kubernetes.Key(ingress))
			})
		})

		It("supports both HTTP and HTTPS backends [HTTP] [Public]", func() {
			// NOTE: Contrary to the remaining tests, which run using dedicated namespaces, this test must use resources in the "default" namespace.

			var (
				err      error
				ingress  *extsv1beta1.Ingress
				publicIP string
				spec     translatorapi.IngressEdgeLBPoolSpec
				svc      *corev1.Service
			)

			// Read the "default/kubernetes" service so we can later update it in order to set its ".spec.type" to "NodePort".
			// This way we will have a suitable, TLS-enabled backend without having to deploy pods and managing TLS ourselves.
			svc, err = f.KubeClient.CoreV1().Services(corev1.NamespaceDefault).Get("kubernetes", metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			// Update "default/kubernetes" in order to set its ".spec.type" to "NodePort".
			svc.Spec.Type = corev1.ServiceTypeNodePort
			svc, err = f.KubeClient.CoreV1().Services(svc.Namespace).Update(svc)
			Expect(err).NotTo(HaveOccurred())

			// Create an object holding the target EdgeLB pool's specification for the "http-echo" Ingress.
			spec = translatorapi.IngressEdgeLBPoolSpec{
				BaseEdgeLBPoolSpec: translatorapi.BaseEdgeLBPoolSpec{
					Name: pointers.NewString("dklb-http-https"),
				},
			}

			// Create an Ingress resource targeting the "default/kubernetes" service, annotating it to be provisioned by EdgeLB.
			ingress, err = f.CreateEdgeLBIngress(svc.Namespace, svc.Name, func(ingress *extsv1beta1.Ingress) {
				_ = translatorapi.SetIngressEdgeLBPoolSpec(ingress, &spec)
				ingress.Spec.Rules = []extsv1beta1.IngressRule{
					{
						IngressRuleValue: extsv1beta1.IngressRuleValue{
							HTTP: &extsv1beta1.HTTPIngressRuleValue{
								Paths: []extsv1beta1.HTTPIngressPath{
									{
										Path: "/kubernetes",
										Backend: extsv1beta1.IngressBackend{
											ServiceName: svc.Name,
											ServicePort: intstr.FromInt(int(svc.Spec.Ports[0].Port)),
										},
									},
								},
							},
						},
					},
				}
			})
			Expect(err).NotTo(HaveOccurred(), "failed to create test ingress")

			// Wait for EdgeLB to acknowledge the pool's creation.
			err = retry.WithTimeout(framework.DefaultRetryTimeout, framework.DefaultRetryInterval, func() (bool, error) {
				ctx, fn := context.WithTimeout(context.Background(), framework.DefaultRetryInterval/2)
				defer fn()
				_, err = f.EdgeLBManager.GetPool(ctx, *spec.Name)
				return err == nil, nil
			})
			Expect(err).NotTo(HaveOccurred(), "timed out while waiting for the edgelb api server to acknowledge the pool's creation")

			// Wait for the Ingress to respond with the default backend.
			log.Debugf("waiting for the public ip for %q to be reported", kubernetes.Key(ingress))
			err = retry.WithTimeout(framework.DefaultRetryTimeout, framework.DefaultRetryInterval, func() (bool, error) {
				// Wait for the pool's public IP to be reported.
				ctx, fn := context.WithTimeout(context.Background(), framework.DefaultRetryTimeout)
				defer fn()
				publicIP, err = f.WaitForPublicIPForIngress(ctx, ingress)
				Expect(err).NotTo(HaveOccurred())
				Expect(publicIP).NotTo(BeEmpty())
				// Attempt to connect to the ingress using the reported IP.
				log.Debugf("attempting to connect to %q at %q", kubernetes.Key(ingress), publicIP)
				_, body, err := f.Request("GET", publicIP, "/foo")
				if err != nil {
					log.Debugf("waiting for the ingress to be reachable at %s", publicIP)
					return false, nil
				}
				// Make sure that we've got an answer from "dklb" itself (the default backend).
				// This guarantees that HTTP backends are supported.
				return strings.Contains(body, "No backend is available to service this request"), nil
			})
			Expect(err).NotTo(HaveOccurred(), "timed out while waiting for the ingress to be reachable")
			log.Debugf("the ingress is reachable at %s", publicIP)

			// Make sure that we can obtain a response from the Kubernetes API.
			// This guarantees that HTTPS backends are supported.
			status, body, err := f.Request("GET", publicIP, "/kubernetes")
			Expect(err).NotTo(HaveOccurred(), "failed to perform http request")
			Expect(status).To(Equal(403), "the response's status code doesn't match the expectation")
			Expect(body).To(MatchRegexp("anonymous"), "the response's body doesn't match the expectations")

			// Manually delete the Ingress resource.
			err = f.KubeClient.ExtensionsV1beta1().Ingresses(ingress.Namespace).Delete(ingress.Name, metav1.NewDeleteOptions(0))
			Expect(err).NotTo(HaveOccurred(), "failed to delete ingress %q", kubernetes.Key(ingress))

			// Undo the change made to the "default/kubernetes" service.
			svc, err = f.KubeClient.CoreV1().Services(svc.Namespace).Get(svc.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			svc.Spec.Type = corev1.ServiceTypeClusterIP
			svc.Spec.Ports[0].NodePort = 0
			svc, err = f.KubeClient.CoreV1().Services(svc.Namespace).Update(svc)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
