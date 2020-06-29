// Copyright (c) 2020 Red Hat, Inc.

// +build e2e

package e2e

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	libgooptions "github.com/open-cluster-management/library-e2e-go/pkg/options"
	libgoapplier "github.com/open-cluster-management/library-go/pkg/applier"
	libgoclient "github.com/open-cluster-management/library-go/pkg/client"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
)

const MANAGEDCLUSTERS_KUBECONFIGS_DIR = "test/e2e/resources/clusters"
const HUBCLUSTER_KUBECONFIG_DIR = "test/e2e/resources/hubs"
const NumberOfPods = 13

const (
	//	MANUAL_IMPORT_IMAGE_PULL_SECRET = "image-pull-secret"
	MANUAL_IMPORT_CLUSTER_SCENARIO = "manual-import"
)

// list of manifestwork name for addon crs
var addonCRs = map[string]string{
	"appmgr":         "applicationmanagers",
	"certpolicyctrl": "certpolicycontrollers",
	"cispolicyctrl":  "cispolicycontrollers",
	"iampolicyctrl":  "iampolicycontrollers",
	"policyctrl":     "policycontrollers",
	"search":         "searchcollectors",
	"workmgr":        "workmanagers",
}

// list of regex we will use to validate json from the manifestwork
var validations = map[string][]string{
	"appmgr": []string{
		`"kind":"ApplicationManager"`,
		`"name":"klusterlet-addon-appmgr"`,
		`"kubeconfig":`,
		`"name":"appmgr-hub-kubeconfig"`,
	},
	"certpolicyctrl": []string{
		`"kind":"CertPolicyController"`,
		`"name":"klusterlet-addon-certpolicyctrl"`,
		`"kubeconfig":`,
		`"name":"certpolicyctrl-hub-kubeconfig"`,
	},
	"cisctrl": []string{
		`"kind":"CISController"`,
		`"name":"klusterlet-addon-cisctrl"`,
		`"kubeconfig":`,
		`"name":"cisctrl-hub-kubeconfig"`,
	},
	"iampolicyctrl": []string{
		`"kind":"IAMPolicyController"`,
		`"name":"klusterlet-addon-iampolicyctrl"`,
		`"kubeconfig":`,
		`"name":"iampolicyctrl-hub-kubeconfig"`,
	},
	"policyctrl": []string{
		`"kind":"PolicyController"`,
		`"name":"klusterlet-addon-policyctrl"`,
		`"kubeconfig":`,
		`"name":"policyctrl-hub-kubeconfig"`,
	},
	"search": []string{
		`"kind":"SearchCollector"`,
		`"name":"klusterlet-addon-search"`,
		`"kubeconfig":`,
		`"name":"search-hub-kubeconfig"`,
	},
	"workmgr": []string{
		`"kind":"WorkManager"`,
		`"name":"klusterlet-addon-workmgr"`,
		`"kubeconfig":`,
		`"name":"workmgr-hub-kubeconfig"`,
	},
	"addon-operator": []string{
		`"kind":"Deployment"`,
		`"name":"klusterlet-addon-operator"`,
		`"kind":"Secret"`,
		`"kubernetes.io/dockerconfigjson"`,
	},
	"crds": []string{
		`"name":"applicationmanagers.agent.open-cluster-management.io"`,
		`"name":"certpolicycontrollers.agent.open-cluster-management.io"`,
		`"name":"cispolicycontrollers.agent.open-cluster-management.io"`,
		`"name":"iampolicycontrollers.agent.open-cluster-management.io"`,
		`"name":"policycontrollers.agent.open-cluster-management.io"`,
		`"name":"searchcollectors.agent.open-cluster-management.io"`,
		`"name":"workmanagers.agent.open-cluster-management.io"`,
		`"rbac.authorization.k8s.io/aggregate-to-admin":"true"`,
	},
}

var _ = Describe("Manual import cluster", func() {

	var err error
	var managedClustersForManualImport map[string]string
	var hubClient client.Client
	var clientHubDynamic dynamic.Interface
	var clientHubClientset clientset.Interface
	var templateProcessor *libgoapplier.TemplateProcessor
	var hubApplier *libgoapplier.Applier
	var clientManagedCluster kubernetes.Interface
	var clientManagedDynamic dynamic.Interface

	BeforeEach(func() {
		managedClustersForManualImport, err = libgooptions.GetManagedClusterKubeConfigs(testOptions.ManagedClusters.ConfigDir, MANUAL_IMPORT_CLUSTER_SCENARIO)
		Expect(err).To(BeNil())
		if len(managedClustersForManualImport) == 0 {
			Skip("Manual import not executed because no managed cluster defined for import")
		}
		SetDefaultEventuallyTimeout(10 * time.Minute)
		SetDefaultEventuallyPollingInterval(10 * time.Second)
		kubeconfig := libgooptions.GetHubKubeConfig(testOptions.Hub.ConfigDir)
		clientHubDynamic, err = libgoclient.NewDefaultKubeClientDynamic(kubeconfig)
		Expect(err).To(BeNil())
		clientHubClientset, err = libgoclient.NewDefaultKubeClientAPIExtension(kubeconfig)
		Expect(err).To(BeNil())
		yamlReader := libgoapplier.NewYamlFileReader("resources")
		templateProcessor, err = libgoapplier.NewTemplateProcessor(yamlReader, &libgoapplier.Options{})
		Expect(err).To(BeNil())
		hubClient, err = libgoclient.NewDefaultClient(kubeconfig, client.Options{})
		Expect(err).To(BeNil())
		hubApplier, err = libgoapplier.NewApplier(templateProcessor, hubClient, nil, nil, nil)
		Expect(err).To(BeNil())
	})

	It("Given a list of clusters to import (cluster/g0/manual-import-service-resources)", func() {
		for clusterName, clusterKubeconfig := range managedClustersForManualImport {
			klog.V(1).Infof("kubeconfigpath: %s", clusterKubeconfig)
			klog.V(1).Infof("========================= Test cluster import cluster %s ===============================", clusterName)
			clientManagedCluster, err = libgoclient.NewDefaultKubeClient(clusterKubeconfig)
			//fmt.Println("clientManagedCluster", clientManagedCluster)
			Expect(err).To(BeNil())
			clientManagedDynamic, err = libgoclient.NewDefaultKubeClientDynamic(clusterKubeconfig)
			Expect(err).To(BeNil())
			Eventually(func() error {
				klog.V(1).Info("Check CRDs")
				return libgoclient.HaveCRDs(clientHubClientset,
					[]string{
						"klusterletaddonconfigs.agent.open-cluster-management.io",
					})
			}).Should(BeNil())
			// Eventually(func() error {
			// 	return libgoclient.HaveDeploymentsInNamespace(testOptions.HubCluster, testOptions.KubeConfig,
			// 		"open-cluster-management",
			// 		[]string{"cert-manager-webhook",
			// 			"console-header",
			// 			"etcd-operator",
			// 			"mcm-apiserver",
			// 			"mcm-controller",
			// 			"mcm-webhook",
			// 			"multicluster-operators-application",
			// 			"multicluster-operators-hub-subscription",
			// 			"multicluster-operators-standalone-subscription",
			// 			"multiclusterhub-repo",
			// 			"multiclusterhub-operator",
			// 			"rcm-controller",
			// 			"search-operator",
			// 			"mcm-controller",
			// 		})
			// }).Should(BeNil())

			By("creating the klusterletaddonconfig", func() {
				klog.V(1).Info("Creating the klusterletaddonconfig")
				values := struct {
					ManagedClusterName string
					//ImagePullSecret    string
				}{
					ManagedClusterName: clusterName,
					//ImagePullSecret:    MANUAL_IMPORT_IMAGE_PULL_SECRET,
				}
				names, err := templateProcessor.AssetNamesInPath("./klusterletaddonconfig_cr.yaml", nil, false)
				Expect(err).To(BeNil())
				klog.V(1).Infof("names: %s", names)
				Expect(hubApplier.CreateOrUpdateAsset("klusterletaddonconfig_cr.yaml", values)).To(BeNil())
				gvr := schema.GroupVersionResource{Group: "agent.open-cluster-management.io", Version: "v1", Resource: "klusterletaddonconfigs"}
				Expect(clientHubDynamic.Resource(gvr).Namespace(clusterName).Get(context.TODO(), clusterName, metav1.GetOptions{})).NotTo(BeNil())
			})

			When("the klusterletaddonconfig is created, wait for manifestwork for crds", func() {
				gvrManifestwork := schema.GroupVersionResource{Group: "work.open-cluster-management.io", Version: "v1", Resource: "manifestworks"}
				var crds *unstructured.Unstructured
				Eventually(func() error {
					klog.V(1).Infof("Wait ManifestWork %s...", clusterName+"-crds")
					crds, err = clientHubDynamic.Resource(gvrManifestwork).Namespace(clusterName).Get(context.TODO(), clusterName+"-crds", metav1.GetOptions{})
					if err != nil {
						klog.V(1).Info(err)
					}
					return err
				}).Should(BeNil())
				validateUnstructured(crds, validations["crds"])
				klog.V(1).Infof("ManifestWork %s created", clusterName+"-crds")
			})

			When("the klusterletaddonconfig is created, wait for manifestwork for addon operator", func() {
				gvrManifestwork := schema.GroupVersionResource{Group: "work.open-cluster-management.io", Version: "v1", Resource: "manifestworks"}
				var addonOperator *unstructured.Unstructured
				Eventually(func() error {
					klog.V(1).Infof("Wait ManifestWork %s...", clusterName+"-addon-operator")
					addonOperator, err = clientHubDynamic.Resource(gvrManifestwork).Namespace(clusterName).Get(context.TODO(), clusterName+"-addon-operator", metav1.GetOptions{})
					if err != nil {
						klog.V(1).Info(err)
					}
					return err
				}).Should(BeNil())

				validateUnstructured(addonOperator, validations["addon-operator"])
				klog.V(1).Infof("ManifestWork %s created", clusterName+"-addon-operator")
			})

			When("the klusterletaddonconfig is created, wait for manifestwork for CRs", func() {
				gvrManifestwork := schema.GroupVersionResource{Group: "work.open-cluster-management.io", Version: "v1", Resource: "manifestworks"}
				for crName, _ := range addonCRs {
					By("Checking " + crName)
					var cr *unstructured.Unstructured
					Eventually(func() error {
						klog.V(1).Infof("Wait ManifestWork CRs %s...", clusterName+"-"+crName)
						cr, err = clientHubDynamic.Resource(gvrManifestwork).Namespace(clusterName).Get(context.TODO(), clusterName+"-"+crName, metav1.GetOptions{})
						if err != nil {
							klog.V(1).Info(err)
						}
						return err
					}).Should(BeNil())
					validateUnstructured(cr, validations[crName])
					klog.V(1).Infof("ManifestWork %s created", clusterName+"-"+crName)
				}
			})

			When("the klusterletaddonconfig is created, wait for namespace creation on managed cluster", func() {
				Eventually(func() error {
					klog.V(1).Infof("Wait namespace open-cluster-management-agent-addon...")
					_, err = clientManagedCluster.CoreV1().Namespaces().Get(context.TODO(), "open-cluster-management-agent-addon", metav1.GetOptions{})
					if err != nil {
						klog.V(1).Info(err)
					}
					return err
				}).Should(BeNil())
				klog.V(1).Infof("Namespace open-cluster-management-agent-addon created")
			})

			When("the klusterletaddonconfig is created, wait for deployment klusterlet-addon-operator creation on managed cluster", func() {
				Eventually(func() error {
					klog.V(1).Infof("Wait deployment klusterlet-addon-operator...")
					_, err = clientManagedCluster.AppsV1().Deployments("open-cluster-management-agent-addon").Get(context.TODO(), "klusterlet-addon-operator", metav1.GetOptions{})
					if err != nil {
						klog.V(1).Info(err)
					}
					return err
				}).Should(BeNil())
				klog.V(1).Infof("Deployment klusterlet-addon-operator created")
			})

			When("the klusterletaddonconfig is created, wait for components creation on managed cluster", func() {
				for crName, crCrd := range addonCRs {
					Eventually(func() error {
						gvr := schema.GroupVersionResource{Group: "agent.open-cluster-management.io", Version: "v1", Resource: crCrd}
						klog.V(1).Infof("Wait component CR klusterlet-addon-%s...", crName)
						_, err = clientManagedDynamic.Resource(gvr).Namespace("open-cluster-management-agent-addon").Get(context.TODO(), "klusterlet-addon-"+crName, metav1.GetOptions{})
						if err != nil {
							klog.V(1).Info(err)
						}
						return err
					}).Should(BeNil())
					klog.V(1).Infof("component CR klusterlet-addon-%s created...", crName)
				}
			})

			When("the klusterletaddonconfig is created, wait for pods Status=Running on managed cluster", func() {
				Eventually(func() error {
					klog.V(1).Infof("Wait for all component pods running...")
					err := waitForPodsRunning(NumberOfPods, clientManagedCluster, "open-cluster-management-agent-addon")
					if err != nil {
						klog.V(1).Info(err)
					}
					return err
				}).Should(BeNil())
				klog.V(1).Infof("Pods in open-cluster-management-agent-addon are running")
			})

			// By(fmt.Sprintf("Deleting the klusterletaddonconfig %s on the hub", clusterName), func() {
			// 	klog.V(1).Infof("Deleting the klusterletaddonconfig %s on the hub", clusterName)
			// 	gvr := schema.GroupVersionResource{Group: "agent.open-cluster-management.io", Version: "v1", Resource: "klusterletaddonconfigs"}
			// 	Expect(clientHubDynamic.Resource(gvr).Namespace(clusterName).Delete(context.TODO(), clusterName, metav1.DeleteOptions{})).NotTo(HaveOccurred())
			// })

			// When("the klusterletaddonconfig is deleted, wait for deletion of components crs on managed cluster", func() {
			// 	for crName, crCrd := range addonCRs {
			// 		Eventually(func() bool {
			// 			gvr := schema.GroupVersionResource{Group: "agent.open-cluster-management.io", Version: "v1", Resource: crCrd}
			// 			klog.V(1).Infof("Wait component CR klusterlet-addon-%s...", crName)
			// 			_, err = clientManagedDynamic.Resource(gvr).Namespace("open-cluster-management-agent-addon").Get(context.TODO(), "klusterlet-addon-"+crName, metav1.GetOptions{})
			// 			if err != nil {
			// 				klog.V(1).Info(err)
			// 				return errors.IsNotFound(err)
			// 			}
			// 			return false
			// 		}).Should(BeTrue())
			// 		klog.V(1).Infof("component CR klusterlet-addon-%s deleted...", crName)
			// 	}
			// })

			// When("the klusterletaddonconfig is deleted, wait for deployment klusterlet-addon-operator deletion on managed cluster", func() {
			// 	Eventually(func() bool {
			// 		klog.V(1).Infof("Wait deployment klusterlet-addon-operator...")
			// 		_, err = clientManagedCluster.AppsV1().Deployments("open-cluster-management-agent-addon").Get(context.TODO(), "klusterlet-addon-operator", metav1.GetOptions{})
			// 		if err != nil {
			// 			klog.V(1).Info(err)
			// 			return errors.IsNotFound(err)
			// 		}
			// 		return false
			// 	}).Should(BeTrue())
			// 	klog.V(1).Infof("Deployment klusterlet-addon-operator deleted")
			// })

			// When("the klusterletaddonconfig is deleted, wait for deletion of manifestwork for CRs", func() {
			// 	gvrManifestwork := schema.GroupVersionResource{Group: "work.open-cluster-management.io", Version: "v1", Resource: "manifestworks"}
			// 	for crName, _ := range addonCRs {
			// 		By("Checking " + crName)
			// 		//var cr *unstructured.Unstructured
			// 		Eventually(func() bool {
			// 			klog.V(1).Infof("Wait ManifestWork CRs %s...", clusterName+"-"+crName)
			// 			_, err = clientHubDynamic.Resource(gvrManifestwork).Namespace(clusterName).Get(context.TODO(), clusterName+"-"+crName, metav1.GetOptions{})
			// 			if err != nil {
			// 				klog.V(1).Info(err)
			// 				return errors.IsNotFound(err)
			// 			}
			// 			return false
			// 		}).Should(BeTrue())
			// 		klog.V(1).Infof("ManifestWork %s deleted", clusterName+"-"+crName)
			// 	}
			// })

			// When("the klusterletaddonconfig is deleted, wait for deletion of manifestwork for addon operator", func() {
			// 	gvrManifestwork := schema.GroupVersionResource{Group: "work.open-cluster-management.io", Version: "v1", Resource: "manifestworks"}
			// 	//var addonOperator *unstructured.Unstructured
			// 	Eventually(func() bool {
			// 		klog.V(1).Infof("Wait ManifestWork %s...", clusterName+"-addon-operator")
			// 		_, err = clientHubDynamic.Resource(gvrManifestwork).Namespace(clusterName).Get(context.TODO(), clusterName+"-addon-operator", metav1.GetOptions{})
			// 		if err != nil {
			// 			klog.V(1).Info(err)
			// 			return errors.IsNotFound(err)
			// 		}
			// 		return false
			// 	}).Should(BeTrue())
			// 	klog.V(1).Infof("ManifestWork %s deleted", clusterName+"-addon-operator")
			// })

			// When("the klusterletaddonconfig is deleted, wait for deletion of manifestwork for crds", func() {
			// 	gvrManifestwork := schema.GroupVersionResource{Group: "work.open-cluster-management.io", Version: "v1", Resource: "manifestworks"}
			// 	Eventually(func() bool {
			// 		klog.V(1).Infof("Wait ManifestWork %s...", clusterName+"-crds")
			// 		_, err = clientHubDynamic.Resource(gvrManifestwork).Namespace(clusterName).Get(context.TODO(), clusterName+"-crds", metav1.GetOptions{})
			// 		if err != nil {
			// 			klog.V(1).Info(err)
			// 			return errors.IsNotFound(err)
			// 		}
			// 		return false
			// 	}).Should(BeTrue())
			// 	klog.V(1).Infof("ManifestWork %s created", clusterName+"-crds")
			// })

			// When("the deletion of the klusterletaddonconfig is requested, wait for the effective deletion", func() {
			// 	By(fmt.Sprintf("Checking the deletion of the klusterletaddonconfig %s on the hub", clusterName), func() {
			// 		klog.V(1).Infof("Checking the deletion of the klusterletaddonconfig %s on the hub", clusterName)
			// 		gvr := schema.GroupVersionResource{Group: "agent.open-cluster-management.io", Version: "v1", Resource: "klusterletaddonconfigs"}
			// 		Eventually(func() bool {
			// 			klog.V(1).Infof("Wait %s CR deletion...", clusterName)
			// 			_, err := clientHubDynamic.Resource(gvr).Namespace(clusterName).Get(context.TODO(), clusterName, metav1.GetOptions{})
			// 			if err != nil {
			// 				klog.V(1).Info(err)
			// 				return errors.IsNotFound(err)
			// 			}
			// 			return false
			// 		}).Should(BeTrue())
			// 		klog.V(1).Infof("%s CR deleted", clusterName)
			// 	})

			// By("Checking the deletion of the namespace open-cluster-management-agent-addon on the managed cluster", func() {
			// 	klog.V(1).Info("Checking the deletion of the namespace open-cluster-management-agent-addon on the managed cluster")
			// 	Eventually(func() bool {
			// 		klog.V(1).Info("Wait namespace open-cluster-management-agent-addon deletion...")
			// 		_, err := clientManagedCluster.CoreV1().Namespaces().Get(context.TODO(), "open-cluster-management-agent-addon", metav1.GetOptions{})
			// 		if err != nil {
			// 			klog.V(1).Info(err)
			// 			return errors.IsNotFound(err)
			// 		}
			// 		return false
			// 	}).Should(BeTrue())
			// 	klog.V(1).Info("namespace open-cluster-management-agent-addon deleted")
			// })
			//})

		}
	})
})

func validateUnstructured(obj *unstructured.Unstructured, regexps []string) error {
	resources, err := obj.MarshalJSON()
	Expect(err).To(BeNil())
	for _, r := range regexps {
		Expect(string(resources)).To(MatchRegexp(r))
	}
	return err
}

func waitForPodsRunning(numPods int, c kubernetes.Interface, namespace string) error {
	pollErr := wait.PollImmediate(time.Second, 120*time.Second,
		func() (bool, error) {
			podList, err := c.CoreV1().Pods("open-cluster-management-agent-addon").List(context.TODO(), metav1.ListOptions{})
			Expect(err).To(BeNil())
			if int(len(podList.Items)) < numPods {
				return false, nil
			}
			if int(len(podList.Items)) > numPods {
				return false, fmt.Errorf("Too many pods scheduled, expected %d got %d", numPods, len(podList.Items))
			}
			for _, pod := range podList.Items {
				if pod.Status.Phase != corev1.PodRunning {
					return false, nil
				}
				klog.V(1).Infof("Pod %s running", pod.Name)
			}
			return true, nil
		})
	Expect(pollErr).NotTo(HaveOccurred())
	return pollErr
}