//go:build !unittests
// +build !unittests

package test

import (
	"context"
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"

	v1core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ptptestconfig "github.com/openshift/ptp-operator/test/conformance/config"
	"github.com/openshift/ptp-operator/test/pkg"
	"github.com/openshift/ptp-operator/test/pkg/client"
	testclient "github.com/openshift/ptp-operator/test/pkg/client"
	"github.com/openshift/ptp-operator/test/pkg/execute"
	"github.com/openshift/ptp-operator/test/pkg/pods"
	"github.com/openshift/ptp-operator/test/pkg/ptptesthelper"
	"github.com/openshift/ptp-operator/test/pkg/testconfig"

	_ "embed"

	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	"github.com/anthhub/forwarder"
)

var _ = Describe("[ptp-long-running]", func() {
	var fullConfig testconfig.TestConfig
	var testParameters ptptestconfig.PtpTestConfig

	execute.BeforeAll(func() {
		testParameters = ptptestconfig.GetPtpTestConfig()
		testclient.Client = testclient.New("")
		Expect(testclient.Client).NotTo(BeNil())
		testconfig.CreatePtpConfigurations()
		fullConfig = testconfig.GetFullDiscoveredConfig(pkg.PtpLinuxDaemonNamespace, false)
	})

	Context("Soak testing", func() {

		BeforeEach(func() {
			if fullConfig.Status == testconfig.DiscoveryFailureStatus {
				Skip("Failed to find a valid ptp slave configuration")
			}

			ptpPods, err := client.Client.CoreV1().Pods(pkg.PtpLinuxDaemonNamespace).List(context.Background(), metav1.ListOptions{LabelSelector: "app=linuxptp-daemon"})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(ptpPods.Items)).To(BeNumerically(">", 0), "linuxptp-daemon is not deployed on cluster")

			ptpSlaveRunningPods := []v1core.Pod{}
			ptpMasterRunningPods := []v1core.Pod{}

			for podIndex := range ptpPods.Items {
				if role, _ := pods.PodRole(&ptpPods.Items[podIndex], pkg.PtpClockUnderTestNodeLabel); role {
					pods.WaitUntilLogIsDetected(&ptpPods.Items[podIndex], pkg.TimeoutIn3Minutes, "Profile Name:")
					ptpSlaveRunningPods = append(ptpSlaveRunningPods, ptpPods.Items[podIndex])
				} else if role, _ := pods.PodRole(&ptpPods.Items[podIndex], pkg.PtpGrandmasterNodeLabel); role {
					pods.WaitUntilLogIsDetected(&ptpPods.Items[podIndex], pkg.TimeoutIn3Minutes, "Profile Name:")
					ptpMasterRunningPods = append(ptpMasterRunningPods, ptpPods.Items[podIndex])
				}
			}

			if testconfig.GlobalConfig.DiscoveredGrandMasterPtpConfig != nil {
				Expect(len(ptpMasterRunningPods)).To(BeNumerically(">=", 1), "Fail to detect PTP master pods on Cluster")
				Expect(len(ptpSlaveRunningPods)).To(BeNumerically(">=", 1), "Fail to detect PTP slave pods on Cluster")
			} else {
				Expect(len(ptpSlaveRunningPods)).To(BeNumerically(">=", 1), "Fail to detect PTP slave pods on Cluster")
			}
			//ptpRunningPods = append(ptpMasterRunningPods, ptpSlaveRunningPods...)
		})

		It("Execute TestCase A", func() {
			done := make(chan interface{})
			go func() {
				logrus.Info("start: TestCase A")
				time.Sleep(100 * time.Millisecond)
				logrus.Info("end: TestCase A")
				close(done)
			}()
			Eventually(done, pkg.TimeoutIn5Minutes).Should(BeClosed())
		})
		It("Execute TestCase B", func() {
			done := make(chan interface{})
			go func() {
				logrus.Info("start: Testing TestCase B")
				time.Sleep(500 * time.Millisecond)
				logrus.Info("end: Testing TestCase B")
				close(done)
			}()
			Eventually(done, pkg.TimeoutIn10Minutes).Should(BeClosed())
		})

		It("Execute TestCase C", func() {
			done := make(chan interface{})
			go func() {
				logrus.Info("start: Testing TestCase C")
				time.Sleep(500 * time.Millisecond)
				logrus.Info("end: Testing TestCase C")
				close(done)
			}()
			Eventually(done, pkg.TimeoutIn3Minutes).Should(BeClosed())
		})

		It("PTP Offset testing", func() {
			a := testParameters
			logrus.Info(a)

			//logrus.Info(fullConfig)
			options := []*forwarder.Option{
				{
					LocalPort:  9043,
					RemotePort: 9043,
					Namespace:  "openshift-ptp",
					//ServiceName: "ptp-event-publisher-service",
					PodName: fullConfig.DiscoveredClockUnderTestPod.Name,
				},
			}
			logrus.Infof("Forwarding to pod: %s node: %s", fullConfig.DiscoveredClockUnderTestPod.Name, fullConfig.DiscoveredClockUnderTestPod.Spec.NodeName)

			ret, err := forwarder.WithForwarders(context.Background(), options, client.Client.KubeConfigPath)
			if err != nil {
				logrus.Error("WithForwarders err: ", err)
				os.Exit(1)
			}
			// remember to close the forwarding
			defer ret.Close()
			// wait forwarding ready
			// the remote and local ports are listed
			ports, err := ret.Ready()
			if err != nil {
				logrus.Error(err)
				os.Exit(1)
			}
			fmt.Printf("ports: %+v\n", ports)
			// ...
			ptptesthelper.RegisterAnWaitForEvents(fullConfig.DiscoveredClockUnderTestPod.Spec.NodeName, "localhost:9043")
			// if you want to block the goroutine and listen IOStreams close signal, you can do as following:
			ret.Wait()
		})

	})
})
