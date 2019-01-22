// +build e2e

package e2e_test

import (
	"context"
	"fmt"

	"github.com/mesosphere/dcos-edge-lb/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	extsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/mesosphere/dklb/pkg/constants"
	"github.com/mesosphere/dklb/pkg/util/kubernetes"
	"github.com/mesosphere/dklb/pkg/util/pointers"
	"github.com/mesosphere/dklb/pkg/util/retry"
	"github.com/mesosphere/dklb/test/e2e/framework"
)

var _ = Describe("Ingress", func() {
	Context("not annotated for provisioning by EdgeLB", func() {
		It("is ignored by the admission webhook [TCP] [Admission]", func() {
			// Create a temporary namespace for the current test.
			f.WithTemporaryNamespace(func(namespace *corev1.Namespace) {
				_, err := f.CreateIngress(namespace.Name, "", func(ingress *extsv1beta1.Ingress) {
					// Set an annotation with a value that would be invalid should the Ingress resource be annotated for provisioning by EdgeLB.
					ingress.Annotations = map[string]string{
						constants.EdgeLBPoolNameAnnotationKey: "__invalid_edgelb_pool_name__",
					}
					// Define a default backend so that the Ingress resource is valid.
					ingress.Spec.Backend = &extsv1beta1.IngressBackend{
						ServiceName: "foo",
						ServicePort: intstr.FromInt(80),
					}
					// Use a randomly-generated name for the Ingress resource.
					ingress.GenerateName = fmt.Sprintf("%s-", namespace.Name)
				})
				// Make sure that no error occurred (meaning the admission webhook has ignored the Ingress resource).
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Context("annotated for provisioning by EdgeLB", func() {
		It("created with an invalid configuration is rejected by the admission webhook [HTTP] [Admission]", func() {
			// Create a temporary namespace for the current test.
			f.WithTemporaryNamespace(func(namespace *corev1.Namespace) {
				tests := []struct {
					description               string
					fn                        framework.IngressCustomizer
					expectedErrorMessageRegex string
				}{
					{
						description: "invalid edgelb pool name",
						fn: func(ingress *extsv1beta1.Ingress) {
							ingress.Annotations = map[string]string{
								constants.EdgeLBPoolNameAnnotationKey: "__foo__",
							}
						},
						expectedErrorMessageRegex: "\"__foo__\" is not valid as an edgelb pool name",
					},
					{
						description: "invalid edgelb pool network",
						fn: func(ingress *extsv1beta1.Ingress) {
							ingress.Annotations = map[string]string{
								constants.EdgeLBPoolNetworkAnnotationKey: "dcos",
							}
						},
						expectedErrorMessageRegex: "cannot join a dcos virtual network when the pool's role is \"slave_public\"",
					},
					{
						description: "invalid edgelb pool cpu request",
						fn: func(ingress *extsv1beta1.Ingress) {
							ingress.Annotations = map[string]string{
								constants.EdgeLBPoolCpusAnnotationKey: "foo",
							}
						},
						expectedErrorMessageRegex: "failed to parse \"foo\" as the amount of cpus to request",
					},
					{
						description: "invalid edgelb pool memory request",
						fn: func(ingress *extsv1beta1.Ingress) {
							ingress.Annotations = map[string]string{
								constants.EdgeLBPoolMemAnnotationKey: "foo",
							}
						},
						expectedErrorMessageRegex: "failed to parse \"foo\" as the amount of memory to request",
					},
					{
						description: "invalid edgelb pool size request",
						fn: func(ingress *extsv1beta1.Ingress) {
							ingress.Annotations = map[string]string{
								constants.EdgeLBPoolSizeAnnotationKey: "foo",
							}
						},
						expectedErrorMessageRegex: "failed to parse \"foo\" as the size to request for the edgelb pool",
					},
					{
						description: "invalid edgelb pool creation strategy",
						fn: func(ingress *extsv1beta1.Ingress) {
							ingress.Annotations = map[string]string{
								constants.EdgeLBPoolCreationStrategyAnnotationKey: "InvalidStrategy",
							}
						},
						expectedErrorMessageRegex: "failed to parse \"InvalidStrategy\" as a pool creation strategy",
					},
					{
						description: "invalid edgelb pool port",
						fn: func(ingress *extsv1beta1.Ingress) {
							ingress.Annotations = map[string]string{
								constants.EdgeLBPoolPortAnnotationKey: "-1",
							}
						},
						expectedErrorMessageRegex: "-1 is not a valid port number",
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
			// Create a temporary namespace for the current test.
			f.WithTemporaryNamespace(func(namespace *corev1.Namespace) {
				var (
					err        error
					initialIng *extsv1beta1.Ingress
				)

				// Create a dummy Ingress resource annotated for provisioning with EdgeLB.
				initialIng, err = f.CreateEdgeLBIngress(namespace.Name, "http-echo", func(ingress *extsv1beta1.Ingress) {
					ingress.Annotations = map[string]string{
						// Request for the EdgeLB pool to be called "<namespace-name>".
						constants.EdgeLBPoolNameAnnotationKey: namespace.Name,
						// Request for the EdgeLB pool to be deployed to a private DC/OS agent.
						constants.EdgeLBPoolRoleAnnotationKey: "*",
						// Request for translation to be paused so that no EdgeLB pool is actually created.
						constants.EdgeLBPoolTranslationPaused: "true",
					}
					ingress.Spec.Backend = &extsv1beta1.IngressBackend{
						ServiceName: "foo",
						ServicePort: intstr.FromString("http"),
					}
				})
				Expect(err).NotTo(HaveOccurred(), "failed to create ingress")

				// Attempt to perform some forbidden updates on the values of the annotations and make sure an error is returned.
				tests := []struct {
					description               string
					fn                        func(*extsv1beta1.Ingress)
					expecterErrorMessageRegex string
				}{
					{
						description: "update the target edgelb pool's name",
						fn: func(ingress *extsv1beta1.Ingress) {
							ingress.Annotations[constants.EdgeLBPoolNameAnnotationKey] = "new-name"
						},
						expecterErrorMessageRegex: "the name of the target edgelb pool cannot be changed",
					},
					{
						description: "update the target edgelb pool's role",
						fn: func(ingress *extsv1beta1.Ingress) {
							ingress.Annotations[constants.EdgeLBPoolRoleAnnotationKey] = "new-role"
						},
						expecterErrorMessageRegex: "the role of the target edgelb pool cannot be changed",
					},
					{
						description: "update the target edgelb pool's virtual network",
						fn: func(ingress *extsv1beta1.Ingress) {
							ingress.Annotations[constants.EdgeLBPoolNetworkAnnotationKey] = "new-name"
						},
						expecterErrorMessageRegex: "the virtual network of the target edgelb pool cannot be changed",
					},
				}
				for _, test := range tests {
					log.Infof("test case: %s", test.description)
					// Create a clone of "initialIng" so we can start each test case with a fresh, valid copy.
					updatedIng := initialIng.DeepCopy()
					// Update the clone.
					test.fn(updatedIng)
					// Make sure an error is returned.
					_, err := f.KubeClient.ExtensionsV1beta1().Ingresses(updatedIng.Namespace).Update(updatedIng)
					Expect(err).To(HaveOccurred())
					statusErr, ok := err.(*errors.StatusError)
					Expect(ok).To(BeTrue())
					Expect(statusErr.ErrStatus.Message).To(MatchRegexp(test.expecterErrorMessageRegex))
				}
			})
		})

		It("is correctly provisioned by EdgeLB [HTTP] [Public]", func() {
			// Create a temporary namespace for the test.
			f.WithTemporaryNamespace(func(namespace *corev1.Namespace) {
				var (
					echoPod1 *corev1.Pod
					echoPod2 *corev1.Pod
					echoPod3 *corev1.Pod
					echoPod4 *corev1.Pod
					echoSvc1 *corev1.Service
					echoSvc2 *corev1.Service
					echoSvc3 *corev1.Service
					echoSvc4 *corev1.Service
					err      error
					ingress  *extsv1beta1.Ingress
					pool     *models.V2Pool
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

				// Create an Ingress resource targeting the services above, annotating it to be provisioned by EdgeLB.
				// The following rules are defined on the Ingress resource:
				// * Requests for the "http-echo-4.com" host are (ALL) directed towards "http-echo-4".
				// * Requests for "*/foo(/.*)?" are directed towards "http-echo-2".
				// * Requests for "*/bar(/.*)?" are directed towards "http-echo-3".
				// * Unmatched requests are directed towards "http-echo-1".
				ingress, err = f.CreateEdgeLBIngress(namespace.Name, "http-echo", func(ingress *extsv1beta1.Ingress) {
					ingress.Annotations = map[string]string{
						// Request for the EdgeLB pool to be called "<namespace-name>".
						constants.EdgeLBPoolNameAnnotationKey: namespace.Name,
						// Request for the EdgeLB pool to be deployed to an agent with the "slave_public" role.
						constants.EdgeLBPoolRoleAnnotationKey: constants.EdgeLBRolePublic,
						// Request for the EdgeLB pool to be given 0.2 CPUs.
						constants.EdgeLBPoolCpusAnnotationKey: "200m",
						// Request for the EdgeLB pool to be given 256MiB of RAM.
						constants.EdgeLBPoolMemAnnotationKey: "256Mi",
						// Request for the EdgeLB pool to be deployed into a single agent.
						constants.EdgeLBPoolSizeAnnotationKey: "1",
					}
					ingress.Spec.Backend = &extsv1beta1.IngressBackend{
						ServiceName: echoSvc1.Name,
						ServicePort: intstr.FromString(echoSvc1.Spec.Ports[0].Name),
					}
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

				// Wait for EdgeLB to acknowledge the pool's creation.
				err = retry.WithTimeout(framework.DefaultRetryTimeout, framework.DefaultRetryInterval, func() (bool, error) {
					ctx, fn := context.WithTimeout(context.Background(), framework.DefaultRetryInterval/2)
					defer fn()
					pool, err = f.EdgeLBManager.GetPoolByName(ctx, ingress.Annotations[constants.EdgeLBPoolNameAnnotationKey])
					return err == nil, nil
				})
				Expect(err).NotTo(HaveOccurred(), "timed out while waiting for the edgelb api server to acknowledge the pool's creation")

				// Make sure the pool is reporting the requested configuration.
				Expect(pool.Name).To(Equal(ingress.Annotations[constants.EdgeLBPoolNameAnnotationKey]))
				Expect(pool.Role).To(Equal(ingress.Annotations[constants.EdgeLBPoolRoleAnnotationKey]))
				Expect(pool.Cpus).To(Equal(0.2))
				Expect(pool.Mem).To(Equal(int32(256)))
				Expect(pool.Count).To(Equal(pointers.NewInt32(1)))

				// TODO (@bcustodio) Wait for the pool's IP(s) to be reported.

				// Wait for the Ingress to be reachable at "<public-ip>".
				err = retry.WithTimeout(framework.DefaultRetryTimeout, framework.DefaultRetryInterval, func() (bool, error) {
					r, err := f.HTTPClient.Get(fmt.Sprintf("http://%s/", publicIP))
					if err != nil {
						log.Debugf("waiting for the ingress to be reachable at %s", publicIP)
						return false, nil
					}
					log.Debugf("the ingress is reachable at %s", publicIP)
					return r.StatusCode == 200, nil
				})
				Expect(err).NotTo(HaveOccurred(), "timed out while waiting for the ingress to be reachable")

				// Make sure that requests are directed towards the appropriate backend and contain the expected headers.
				tests := []struct {
					host        string
					path        string
					expectedPod string
				}{
					// Test that requests whose path starts with "/foo" but whose host is "http-echo-4.com" are directed towards "http-echo-4".
					{
						host:        "http-echo-4.com",
						path:        "/foo",
						expectedPod: echoPod4.Name,
					},
					// Test that requests whose path starts with "/foo" are directed towards "http-echo-2".
					{
						host:        publicIP,
						path:        "/foo",
						expectedPod: echoPod2.Name,
					},
					// Test that requests whose path contains "/bar" are directed towards "http-echo-2" (and not "http-echo-3") as the path starts with "/foo".
					{
						host:        publicIP,
						path:        "/foo/bar",
						expectedPod: echoPod2.Name,
					},
					// Test that requests whose path starts with "/bar" are directed towards "http-echo-3".
					{
						host:        publicIP,
						path:        "/bar?foo=bar",
						expectedPod: echoPod3.Name,
					},
					// Test that requests whose path contains "/foo" are directed towards "http-echo-3" (and not "http-echo-2") as the path starts with "/bar".
					{
						host:        publicIP,
						path:        "/bar/foo",
						expectedPod: echoPod3.Name,
					},
					// Test that unmatched requests are directed towards "http-echo-1" (the default backend).
					{
						host:        publicIP,
						path:        "/baz",
						expectedPod: echoPod1.Name,
					},
				}
				for _, test := range tests {
					for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
						log.Debugf("test case: %s request to host %q and path %q is directed towards %q", method, test.host, test.path, test.expectedPod)
						res, err := f.EchoRequest(method, publicIP, test.path, map[string]string{
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

				// Manually delete the Ingress resource now in order to prevent the EdgeLB pool from being re-created during namespace deletion.
				err = f.KubeClient.ExtensionsV1beta1().Ingresses(ingress.Namespace).Delete(ingress.Name, metav1.NewDeleteOptions(0))
				Expect(err).NotTo(HaveOccurred(), "failed to delete ingress %q", kubernetes.Key(ingress))
			})
		})

		It("can share a pool with an Ingress resource in a different namespace [HTTP] [Public]", func() {
			// Create two temporary namespaces for the test.
			f.WithTemporaryNamespace(func(namespace1 *corev1.Namespace) {
				f.WithTemporaryNamespace(func(namespace2 *corev1.Namespace) {

					var (
						echoPod1 *corev1.Pod
						echoPod2 *corev1.Pod
						echoSvc1 *corev1.Service
						echoSvc2 *corev1.Service
						err      error
						ingress1 *extsv1beta1.Ingress
						ingress2 *extsv1beta1.Ingress
						pool     *models.V2Pool
					)

					// Create the first "echo" pod.
					echoPod1, err = f.CreateEchoPod(namespace1.Name, "http-echo-1")
					Expect(err).NotTo(HaveOccurred(), "failed to create echo pod")
					// Create the first "echo" service.
					echoSvc1, err = f.CreateServiceForEchoPod(echoPod1)
					Expect(err).NotTo(HaveOccurred(), "failed to create service for echo pod %q", kubernetes.Key(echoPod1))

					// Create an Ingress resource targeting the "http-echo-1" service above, annotating it to be provisioned by EdgeLB.
					// The Ingress is configured to direct all traffic under "/foo" to "http-echo-1".
					ingress1, err = f.CreateEdgeLBIngress(namespace1.Name, "http-echo", func(ingress *extsv1beta1.Ingress) {
						ingress.Annotations = map[string]string{
							// Request for the EdgeLB pool to be called "<namespace-name>".
							constants.EdgeLBPoolNameAnnotationKey: fmt.Sprintf("%s-%s", namespace1.Name, namespace2.Name),
							// Request for the EdgeLB pool to be deployed to an agent with the "slave_public" role.
							constants.EdgeLBPoolRoleAnnotationKey: constants.EdgeLBRolePublic,
							// Request for the EdgeLB pool to use the "18080" frontend bind port.
							constants.EdgeLBPoolPortAnnotationKey: "18080",
						}
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
						pool, err = f.EdgeLBManager.GetPoolByName(ctx, ingress1.Annotations[constants.EdgeLBPoolNameAnnotationKey])
						return err == nil, nil
					})
					Expect(err).NotTo(HaveOccurred(), "timed out while waiting for the edgelb api server to acknowledge the pool's creation")

					// TODO (@bcustodio) Wait for the pool's IP(s) to be reported.

					// Wait for the Ingress to be reachable at "http://<public-ip>:18080/foo".
					err = retry.WithTimeout(framework.DefaultRetryTimeout, framework.DefaultRetryInterval, func() (bool, error) {
						url := fmt.Sprintf("http://%s:%s/foo", publicIP, ingress1.Annotations[constants.EdgeLBPoolPortAnnotationKey])
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

					// Create an Ingress resource targeting the "http-echo-2" service above, annotating it to be provisioned by EdgeLB.
					// The Ingress is configured to direct all traffic under "/foo" to "http-echo-2".
					ingress2, err = f.CreateEdgeLBIngress(namespace2.Name, "http-echo", func(ingress *extsv1beta1.Ingress) {
						ingress.Annotations = map[string]string{
							// Request for the EdgeLB pool to be called "<namespace-name>".
							constants.EdgeLBPoolNameAnnotationKey: pool.Name,
							// Request for the EdgeLB pool to be deployed to an agent with the "slave_public" role.
							constants.EdgeLBPoolRoleAnnotationKey: constants.EdgeLBRolePublic,
							// Request for the EdgeLB pool to use the "28080" frontend bind port.
							constants.EdgeLBPoolPortAnnotationKey: "28080",
						}
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

					// Wait for the Ingress to be reachable at "http://<public-ip>:28080/foo".
					err = retry.WithTimeout(framework.DefaultRetryTimeout, framework.DefaultRetryInterval, func() (bool, error) {
						url := fmt.Sprintf("http://%s:%s/foo", publicIP, ingress2.Annotations[constants.EdgeLBPoolPortAnnotationKey])
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
						port              string
						path              string
						expectedNamespace string
						expectedPod       string
					}{
						{
							port:              ingress1.Annotations[constants.EdgeLBPoolPortAnnotationKey],
							path:              "/foo",
							expectedNamespace: echoPod1.Namespace,
							expectedPod:       echoPod1.Name,
						},
						{
							port:              ingress2.Annotations[constants.EdgeLBPoolPortAnnotationKey],
							path:              "/foo",
							expectedNamespace: echoPod2.Namespace,
							expectedPod:       echoPod2.Name,
						},
					}
					for _, test := range tests {
						log.Debugf("test case: request to port %s and path %q is directed towards %q", test.port, test.path, test.expectedPod)
						res, err := f.EchoRequest("GET", fmt.Sprintf("%s:%s", publicIP, test.port), test.path, nil)
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
	})
})
