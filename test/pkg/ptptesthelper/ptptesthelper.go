package ptptesthelper

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ptpv1 "github.com/openshift/ptp-operator/api/v1"
	"github.com/openshift/ptp-operator/test/pkg"
	"github.com/openshift/ptp-operator/test/pkg/client"
	"github.com/openshift/ptp-operator/test/pkg/metrics"
	nodeshelper "github.com/openshift/ptp-operator/test/pkg/nodes"
	"github.com/openshift/ptp-operator/test/pkg/pods"
	"github.com/openshift/ptp-operator/test/pkg/ptphelper"
	"github.com/openshift/ptp-operator/test/pkg/testconfig"
	"github.com/pkg/errors"

	"github.com/redhat-cne/sdk-go/pkg/event"
	ptpEvent "github.com/redhat-cne/sdk-go/pkg/event/ptp"
	httpevents "github.com/redhat-cne/sdk-go/pkg/protocol/http"
	"github.com/redhat-cne/sdk-go/pkg/pubsub"
	"github.com/redhat-cne/sdk-go/pkg/subscriber"
	"github.com/redhat-cne/sdk-go/pkg/types"
	"github.com/sirupsen/logrus"
	k8sPriviledgedDs "github.com/test-network-function/privileged-daemonset"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	//"github.com/redhat-cne/sdk-go/pkg/subscriber"
	"github.com/google/uuid"
	"log"
	"net"
)

// helper function for old interface discovery test
func TestPtpRunningPods(ptpPods *corev1.PodList) (ptpRunningPods []*corev1.Pod, err error) {
	ptpSlaveRunningPods := []*corev1.Pod{}
	ptpMasterRunningPods := []*corev1.Pod{}
	for podIndex := range ptpPods.Items {
		isClockUnderTestPod, err := pods.PodRole(&ptpPods.Items[podIndex], pkg.PtpClockUnderTestNodeLabel)
		if err != nil {
			logrus.Errorf("could not check clock under test pod role, err: %s", err)
			return ptpRunningPods, errors.Errorf("could not check clock under test pod role, err: %s", err)
		}

		isGrandmaster, err := pods.PodRole(&ptpPods.Items[podIndex], pkg.PtpGrandmasterNodeLabel)
		if err != nil {
			logrus.Errorf("could not check Grandmaster pod role, err: %s", err)
			return ptpRunningPods, errors.Errorf("could not check Grandmaster pod role, err: %s", err)
		}

		if isClockUnderTestPod {
			pods.WaitUntilLogIsDetected(&ptpPods.Items[podIndex], pkg.TimeoutIn3Minutes, "Profile Name:")
			ptpSlaveRunningPods = append(ptpSlaveRunningPods, &ptpPods.Items[podIndex])
		} else if isGrandmaster {
			pods.WaitUntilLogIsDetected(&ptpPods.Items[podIndex], pkg.TimeoutIn3Minutes, "Profile Name:")
			ptpMasterRunningPods = append(ptpMasterRunningPods, &ptpPods.Items[podIndex])
		}
	}
	if testconfig.GlobalConfig.DiscoveredGrandMasterPtpConfig != nil {
		if len(ptpMasterRunningPods) == 0 {
			return ptpRunningPods, errors.Errorf("Fail to detect PTP master pods on Cluster")
		}
		if len(ptpSlaveRunningPods) == 0 {
			return ptpRunningPods, errors.Errorf("Fail to detect PTP slave pods on Cluster")
		}

	} else {
		if len(ptpSlaveRunningPods) == 0 {
			return ptpRunningPods, errors.Errorf("Fail to detect PTP slave pods on Cluster")
		}
	}
	ptpRunningPods = append(ptpRunningPods, ptpSlaveRunningPods...)
	ptpRunningPods = append(ptpRunningPods, ptpMasterRunningPods...)
	return ptpRunningPods, nil
}

// waits for the foreign master to appear in the logs and checks the clock accuracy
func BasicClockSyncCheck(fullConfig testconfig.TestConfig, ptpConfig *ptpv1.PtpConfig, gmID *string) error {
	if gmID != nil {
		logrus.Infof("expected master=%s", *gmID)
	}
	profileName, errProfile := ptphelper.GetProfileName(ptpConfig)

	if fullConfig.PtpModeDesired == testconfig.Discovery {
		// Only for ptp mode == discovery, if errProfile is not nil just log a info message
		if errProfile != nil {
			logrus.Infof("profile name not detected in log (probably because of log rollover)). Remote clock ID will not be printed")
		}
	} else if errProfile != nil {
		// Otherwise, for other non-discovery modes, report an error
		return errors.Errorf("expects errProfile to be nil, errProfile=%s", errProfile)
	}

	label, err := ptphelper.GetLabel(ptpConfig)
	if err != nil {
		logrus.Debugf("could not get label because of err: %s", err)
	}
	nodeName, err := ptphelper.GetFirstNode(ptpConfig)
	if err != nil {
		logrus.Debugf("could not get nodeName because of err: %s", err)
	}
	slaveMaster, err := ptphelper.GetClockIDForeign(profileName, label, nodeName)
	if errProfile == nil {
		if fullConfig.PtpModeDesired == testconfig.Discovery {
			if err != nil {
				logrus.Infof("slave's Master not detected in log (probably because of log rollover))")
			} else {
				logrus.Infof("slave's Master=%s", slaveMaster)
			}
		} else {
			if err != nil {
				return errors.Errorf("expects err to be nil, err=%s", err)
			}
			if slaveMaster == "" {
				return errors.Errorf("expects slaveMaster to not be empty, slaveMaster=%s", slaveMaster)
			}
			logrus.Infof("slave's Master=%s", slaveMaster)
		}
	}
	if gmID != nil {
		if !strings.HasPrefix(slaveMaster, *gmID) {
			return errors.Errorf("Slave connected to another (incorrect) Master, slaveMaster=%s, gmID=%s", slaveMaster, *gmID)
		}
	}

	Eventually(func() error {
		err = metrics.CheckClockRoleAndOffset(ptpConfig, label, nodeName)
		if err != nil {
			logrus.Infof(fmt.Sprintf("CheckClockRoleAndOffset Failed because of err: %s", err))
		}
		return err
	}, pkg.TimeoutIn10Minutes, pkg.Timeout10Seconds).Should(BeNil(), fmt.Sprintf("Timeout to detect metrics for ptpconfig %s", ptpConfig.Name))
	return nil
}

func VerifyAfterRebootState(rebootedNodes []string, fullConfig testconfig.TestConfig) {
	By("Getting ptp operator config")
	ptpConfig, err := client.Client.PtpV1Interface.PtpOperatorConfigs(pkg.PtpLinuxDaemonNamespace).Get(context.Background(), pkg.PtpConfigOperatorName, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())
	listOptions := metav1.ListOptions{}
	if ptpConfig.Spec.DaemonNodeSelector != nil && len(ptpConfig.Spec.DaemonNodeSelector) != 0 {
		listOptions = metav1.ListOptions{LabelSelector: metav1.FormatLabelSelector(&metav1.LabelSelector{MatchLabels: ptpConfig.Spec.DaemonNodeSelector})}
	}

	By("Getting list of nodes")
	nodes, err := client.Client.CoreV1().Nodes().List(context.Background(), listOptions)
	Expect(err).NotTo(HaveOccurred())
	By("Checking number of nodes")
	Expect(len(nodes.Items)).To(BeNumerically(">", 0), "number of nodes should be more than 0")

	By("Get daemonsets collection for the namespace " + pkg.PtpLinuxDaemonNamespace)
	ds, err := client.Client.DaemonSets(pkg.PtpLinuxDaemonNamespace).List(context.Background(), metav1.ListOptions{})
	Expect(err).ToNot(HaveOccurred())
	Expect(len(ds.Items)).To(BeNumerically(">", 0), "no damonsets found in the namespace "+pkg.PtpLinuxDaemonNamespace)

	By("Checking number of scheduled instances")
	Expect(ds.Items[0].Status.CurrentNumberScheduled).To(BeNumerically("==", len(nodes.Items)), "should be one instance per node")

	By("Checking if the ptp offset metric is present")
	for _, slaveNode := range rebootedNodes {

		runningPods := pods.GetRebootDaemonsetPodsAt(slaveNode)

		// Testing for one pod is sufficient as these pods are running on the same node that restarted
		for _, pod := range runningPods.Items {
			Expect(ptphelper.IsClockUnderTestPod(&pod)).To(BeTrue())

			logrus.Printf("Calling metrics endpoint for pod %s with status %s", pod.Name, pod.Status.Phase)

			time.Sleep(pkg.TimeoutIn3Minutes)

			Eventually(func() string {
				commands := []string{
					"curl", "-s", pkg.MetricsEndPoint,
				}
				buf, err := pods.ExecCommand(client.Client, &pod, pkg.RebootDaemonSetContainerName, commands)
				Expect(err).NotTo(HaveOccurred())

				scanner := bufio.NewScanner(strings.NewReader(buf.String()))
				var lines []string = make([]string, 5)
				for scanner.Scan() {
					text := scanner.Text()
					if strings.Contains(text, metrics.OpenshiftPtpOffsetNs+"{from=\"master\"") {
						logrus.Printf("Line obtained is %s", text)
						lines = append(lines, text)
					}
				}
				var offset string
				var offsetVal int
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line == "" {
						continue
					}
					tokens := strings.Fields(line)
					len := len(tokens)

					if len > 0 {
						offset = tokens[len-1]
						if offset != "" {
							if val, err := strconv.Atoi(offset); err == nil {
								offsetVal = val
								logrus.Println("Offset value obtained", offsetVal)
								break
							}
						}
					}
				}
				Expect(buf.String()).NotTo(BeEmpty())
				Expect(offsetVal >= pkg.MasterOffsetLowerBound && offsetVal < pkg.MasterOffsetHigherBound).To(BeTrue())
				return buf.String()
			}, pkg.TimeoutIn5Minutes, 5*time.Second).Should(ContainSubstring(metrics.OpenshiftPtpOffsetNs),
				"Time metrics are not detected")
			break
		}
	}
}

func CheckSlaveSyncWithMaster(fullConfig testconfig.TestConfig) {
	By("Checking if slave nodes can sync with the master")

	ptpPods, err := client.Client.CoreV1().Pods(pkg.PtpLinuxDaemonNamespace).List(context.Background(), metav1.ListOptions{LabelSelector: "app=linuxptp-daemon"})
	Expect(err).NotTo(HaveOccurred())
	Expect(len(ptpPods.Items)).To(BeNumerically(">", 0), "linuxptp-daemon is not deployed on cluster")

	ptpSlaveRunningPods := []corev1.Pod{}
	ptpMasterRunningPods := []corev1.Pod{}

	for _, pod := range ptpPods.Items {
		if ptphelper.IsClockUnderTestPod(&pod) {
			pods.WaitUntilLogIsDetected(&pod, pkg.TimeoutIn5Minutes, "Profile Name:")
			ptpSlaveRunningPods = append(ptpSlaveRunningPods, pod)
		} else if ptphelper.IsGrandMasterPod(&pod) {
			pods.WaitUntilLogIsDetected(&pod, pkg.TimeoutIn5Minutes, "Profile Name:")
			ptpMasterRunningPods = append(ptpMasterRunningPods, pod)
		}
	}
	if testconfig.GlobalConfig.DiscoveredGrandMasterPtpConfig != nil {
		Expect(len(ptpMasterRunningPods)).To(BeNumerically(">=", 1), "Fail to detect PTP master pods on Cluster")
		Expect(len(ptpSlaveRunningPods)).To(BeNumerically(">=", 1), "Fail to detect PTP slave pods on Cluster")
	} else {
		Expect(len(ptpSlaveRunningPods)).To(BeNumerically(">=", 1), "Fail to detect PTP slave pods on Cluster")
	}

	var masterID string
	var slaveMasterID string
	grandMaster := "assuming the grand master role"

	for _, pod := range ptpPods.Items {
		if pkg.PtpGrandmasterNodeLabel != "" &&
			ptphelper.IsGrandMasterPod(&pod) {
			podLogs, err := pods.GetLog(&pod, pkg.PtpContainerName)
			Expect(err).NotTo(HaveOccurred(), "Error to find needed log due to %s", err)
			Expect(podLogs).Should(ContainSubstring(grandMaster),
				fmt.Sprintf("Log message %q not found in pod's log %s", grandMaster, pod.Name))
			for _, line := range strings.Split(podLogs, "\n") {
				if strings.Contains(line, "selected local clock") && strings.Contains(line, "as best master") {
					// Log example: ptp4l[10731.364]: [eno1] selected local clock 3448ed.fffe.f38e00 as best master
					masterID = strings.Split(line, " ")[5]
				}
			}
		}
		if ptphelper.IsClockUnderTestPod(&pod) {
			podLogs, err := pods.GetLog(&pod, pkg.PtpContainerName)
			Expect(err).NotTo(HaveOccurred(), "Error to find needed log due to %s", err)

			for _, line := range strings.Split(podLogs, "\n") {
				if strings.Contains(line, "new foreign master") {
					// Log example: ptp4l[11292.467]: [eno1] port 1: new foreign master 3448ed.fffe.f38e00-1
					slaveMasterID = strings.Split(line, " ")[7]
				}
			}
		}
	}
	Expect(masterID).NotTo(BeNil())
	Expect(slaveMasterID).NotTo(BeNil())
	Expect(slaveMasterID).Should(HavePrefix(masterID), "Error match MasterID with the SlaveID. Slave connected to another Master")
}

func RebootSlaveNode(fullConfig testconfig.TestConfig) {
	logrus.Info("Rebooting system starts ..............")

	const (
		imageWithVersion = "quay.io/testnetworkfunction/debug-partner:latest"
	)

	// Create the client of Priviledged Daemonset
	k8sPriviledgedDs.SetDaemonSetClient(client.Client.Interface)
	// 1. create a daemon set for the node reboot
	dummyLabels := map[string]string{}
	rebootDaemonSetRunningPods, err := k8sPriviledgedDs.CreateDaemonSet(pkg.RebootDaemonSetName, pkg.RebootDaemonSetNamespace, pkg.RebootDaemonSetContainerName, imageWithVersion, dummyLabels, pkg.TimeoutIn5Minutes)
	if err != nil {
		logrus.Errorf("error : +%v\n", err.Error())
	}
	nodeToPodMapping := make(map[string]corev1.Pod)
	for _, dsPod := range rebootDaemonSetRunningPods.Items {
		nodeToPodMapping[dsPod.Spec.NodeName] = dsPod
	}

	// 2. Get the slave config, restart the slave node
	slavePtpConfig := fullConfig.DiscoveredClockUnderTestPtpConfig
	restartedNodes := make([]string, len(rebootDaemonSetRunningPods.Items))

	for _, recommend := range slavePtpConfig.Spec.Recommend {
		matches := recommend.Match

		for _, match := range matches {
			nodeLabel := *match.NodeLabel
			// Get all nodes with this node label
			nodes, err := client.Client.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{LabelSelector: nodeLabel})
			Expect(err).NotTo(HaveOccurred())

			// Restart the node
			for _, node := range nodes.Items {
				nodeshelper.RebootNode(nodeToPodMapping[node.Name], node)
				restartedNodes = append(restartedNodes, node.Name)
			}
		}
	}
	logrus.Printf("Restarted nodes %v", restartedNodes)

	// 3. Verify the setup of PTP
	VerifyAfterRebootState(restartedNodes, fullConfig)

	// 4. Slave nodes can sync to master
	CheckSlaveSyncWithMaster(fullConfig)

	logrus.Info("Rebooting system ends ..............")
}

func initSubscribers() map[string]string {
	subscribeTo := make(map[string]string)
	subscribeTo[string(ptpEvent.OsClockSyncStateChange)] = string(ptpEvent.OsClockSyncState)
	//subscribeTo[string(ptpEvent.PtpClockClassChange)] = string(ptpEvent.PtpClockClass)
	//subscribeTo[string(ptpEvent.PtpStateChange)] = string(ptpEvent.PtpLockState)
	return subscribeTo
}

// Consumer webserver
func server(localListeningEndpoint string) {

	http.HandleFunc("/event", getEvent)
	http.HandleFunc("/health", health)
	http.HandleFunc("/ack/event", ackEvent)
	err := http.ListenAndServe(localListeningEndpoint, nil)
	logrus.Infof("ListenAndServe returns with err= %s", err)
}

func health(w http.ResponseWriter, req *http.Request) {
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

// net/http.Header ["User-Agent": ["Go-http-client/1.1"],
// "Ce-Subject": ["/cluster/node/master2/sync/ptp-status/ptp-clock-class-change"],
// "Content-Type": ["application/json"],
// "Accept-Encoding": ["gzip"],
// "Content-Length": ["138"],
// "Ce-Id": ["4eff05f8-493a-4382-8d89-209dc2179041"],
// "Ce-Source": ["/cluster/node/master2/sync/ptp-status/ptp-clock-class-change"],
// "Ce-Specversion": ["0.3"], "Ce-Time": ["2022-12-16T14:26:47.167232673Z"],
// "Ce-Type": ["event.sync.ptp-status.ptp-clock-class-change"], ]

func getEvent(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	aSource := req.Header.Get("Ce-Source")
	aType := toEventType[req.Header.Get("Ce-Type")]
	aTime, err := types.ParseTimestamp(req.Header.Get("Ce-Time"))
	if err != nil {
		logrus.Error(err)
	}
	logrus.Infof(aTime.String())
	logrus.Infof(aSource)

	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		logrus.Errorf("error reading event %v", err)
	}
	e := string(bodyBytes)

	if e != "" {
		logrus.Infof("received event %s", string(bodyBytes))
		switch aType {
		case LockState:
			processEventLockState(aSource, aType, aTime.Time, bodyBytes)
		}

	} else {
		w.WriteHeader(http.StatusNoContent)
	}
}

type EventType int64

const (
	LockState EventType = iota
)

type LockStateValue int64

const (
	AcquiringSync LockStateValue = iota
	AntennaDisconnected
	AntennaShortCircuit
	Booting
	Freerun
	Holdover
	Locked
	Synchronized
	Unlocked
)

type StoredEvent struct {
	TimeStamp time.Time
	Source    string
	Type      EventType
	Value     int64
}

var (
	mu               sync.Mutex
	events           []StoredEvent
	toEventType      = map[string]EventType{string(ptpEvent.OsClockSyncStateChange): LockState}
	toLockStateValue = map[string]LockStateValue{
		string(ptpEvent.ACQUIRING_SYNC):        AcquiringSync,
		string(ptpEvent.ANTENNA_DISCONNECTED):  AntennaDisconnected,
		string(ptpEvent.ANTENNA_SHORT_CIRCUIT): AntennaShortCircuit,
		string(ptpEvent.BOOTING):               Booting,
		string(ptpEvent.FREERUN):               Freerun,
		string(ptpEvent.HOLDOVER):              Holdover,
		string(ptpEvent.LOCKED):                Locked,
		string(ptpEvent.SYNCHRONIZED):          Synchronized,
		string(ptpEvent.UNLOCKED):              Unlocked,
	}
)

func processEventLockState(source string, eventType EventType, eventTime time.Time, data []byte) {
	var e event.Data
	json.Unmarshal(data, &e)
	logrus.Info(e)

	// Note that there is no UnixMillis, so to get the
	// milliseconds since epoch you'll need to manually
	// divide from nanoseconds.
	latency := (time.Now().UnixNano() - eventTime.UnixNano()) / 1000000
	// set log to Info level for performance measurement
	logrus.Infof("Latency for the event: %v ms\n", latency)

	var value LockStateValue
	for _, v := range e.Values {
		if v.ValueType == event.ENUMERATION {
			if str, ok := v.Value.(string); ok {
				value = toLockStateValue[str]
			} else {
				logrus.Error("could not extract Lockstate value")
			}
		}
	}
	aEvent := StoredEvent{TimeStamp: eventTime, Source: source, Type: eventType, Value: int64(value)}
	mu.Lock()
	events = append(events, aEvent)
	mu.Unlock()
	logrus.Info(events)
}

func ackEvent(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		logrus.Errorf("error reading acknowledgment  %v", err)
	}
	e := string(bodyBytes)
	if e != "" {
		logrus.Infof("received ack %s", string(bodyBytes))
	} else {
		w.WriteHeader(http.StatusNoContent)
	}
}

const (
	resourcePrefix     string = "/cluster/node/%s%s"
	localAPIAddr       string = "localhost:9085"
	localListeningPort string = ":8989"
)

func RegisterAnWaitForEvents(nodeName, apiAddr string) {

	subscribeTo := initSubscribers()
	var wg sync.WaitGroup
	wg.Add(1)
	localListeningEndpoint := GetOutboundIP(client.Client.Config.Host).String() + localListeningPort
	go server(localListeningPort) // spin local api
	time.Sleep(5 * time.Second)

	var subs []pubsub.PubSub
	for _, resource := range subscribeTo {
		subs = append(subs, pubsub.PubSub{
			ID:       uuid.New().String(),
			Resource: fmt.Sprintf(resourcePrefix, nodeName, resource),
		})
	}

	// if AMQ enabled the subscription will create an AMQ listener client
	// IF HTTP enabled, the subscription will post a subscription  requested to all
	// publishers that are defined in http-event-publisher variable

	e := createSubscription(subs, apiAddr, localListeningEndpoint)
	if e != nil {
		logrus.Error(e)
	}
	logrus.Info("waiting for events")
	wg.Wait()
}

func createSubscription(subscriptions []pubsub.PubSub, apiAddr, localAPIAddr string) (err error) {
	//var status int
	subURL := &types.URI{URL: url.URL{Scheme: "http",
		Host: apiAddr,
		Path: "subscription"}}
	endpointURL := &types.URI{URL: url.URL{Scheme: "http",
		Host: localAPIAddr,
		Path: ""}}

	subs := subscriber.New(uuid.UUID{})
	//Self URL
	_ = subs.SetEndPointURI(endpointURL.String())

	// create a subscriber model
	subs.AddSubscription(subscriptions...)

	ce, _ := subs.CreateCloudEvents()
	//ce.SetSource(endpointURL.String())
	ce.SetSubject("1")
	ce.SetSource(subscriptions[0].Resource)
	ce.SetDataContentType("application/json")

	//var subB []byte

	logrus.Info(ce)

	if err := httpevents.Post(fmt.Sprintf("%s", subURL.String()), *ce); err != nil {
		logrus.Errorf("(1)error creating: %v at  %s with data %s=%s", err, subURL.String(), ce.String(), ce.Data())
	}

	/*if subB, err = json.Marshal(&ce); err == nil {
		rc := restclient.New()
		if status, subB = rc.PostWithReturn(subURL, subB); status != http.StatusCreated {
			err = fmt.Errorf("error subscription creation api at %s, returned status %d", subURL, status)
		}
	}*/
	return
}

/*func createSubscription2(resourceAddress, apiAddr, localAPIAddr string) (sub pubsub.PubSub, err error) {
	var status int
	subURL := &types.URI{URL: url.URL{Scheme: "http",
		Host: apiAddr,
		Path: fmt.Sprintf("%s%s", apiPath, "subscriptions")}}
	endpointURL := &types.URI{URL: url.URL{Scheme: "http",
		Host: localAPIAddr,
		Path: "event"}}
	aUUID:=uuid.New()
	// Post it to the address that has been specified : to target URL
	subs := subscriber.New(aUUID)
	//Self URL
	_ = subs.SetEndPointURI(localAPIAddr)
	obj := pubsub.PubSub{
		ID:       endpointURL,
		Resource: resourceAddress,
	}
	// create a subscriber model
	subs.AddSubscription(obj)
	subs.Action = d.Status
	ce, _ := subs.CreateCloudEvents()
	ce.SetSubject(d.Status.String())
	ce.SetSource(d.Address)


	var subB []byte

	if subB, err = json.Marshal(&sub); err == nil {
		rc := restclient.New()
		if status, subB = rc.PostWithReturn(subURL, subB); status != http.StatusCreated {
			err = fmt.Errorf("error subscription creation api at %s, returned status %d", subURL, status)
		} else {
			err = json.Unmarshal(subB, &sub)
		}
	} else {
		err = fmt.Errorf("failed to marshal subscription or %s", resourceAddress)
	}
	return



	if len(h.Publishers) > 0 {
		for _, pubURL := range h.Publishers { // if you call
			if err := Post(fmt.Sprintf("%s/subscription", subURL.String()), *ce); err != nil {
				log.Errorf("(1)error creating: %v at  %s with data %s=%s", err, pubURL.String(), ce.String(), ce.Data())
				localmetrics.UpdateSenderCreatedCount(d.Address, localmetrics.ACTIVE, -1)
				d.Status = channel.FAILED
				h.DataOut <- d
			}
		}
	}

}
*/

// Get preferred outbound ip of this machine
func GetOutboundIP(aUrl string) net.IP {
	u, err := url.Parse(aUrl)
	if err != nil {
		log.Fatalf("cannot parse k8s api url, would not receive events, stopping, err = %s", err)
	}

	conn, err := net.Dial("udp", u.Host)
	if err != nil {
		log.Fatalf("error dialing host or address (%s), err = %s", u.Host, err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	logrus.Infof("Outbound IP = %s to reach server: %s", localAddr.IP.String(), client.Client.Config.Host)
	return localAddr.IP
}
