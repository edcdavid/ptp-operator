package nodes

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientsholder "github.com/openshift/ptp-operator/test/utils/client"
	"github.com/pkg/errors"
	ptpv1 "github.com/openshift/ptp-operator/api/v1"
)

// NodesSelector represent the label selector used to filter impacted nodes.
var NodesSelector string

func init() {
	NodesSelector = os.Getenv("NODES_SELECTOR")
}

type NodeTopology struct {
	NodeName      string
	InterfaceList []string
	NodeObject    *corev1.Node
}

func LabelNode(nodeName, key, value string) (*corev1.Node, error) {
	NodeObject, err := clientsholder.Client.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	NodeObject.Labels[key] = value
	NodeObject, err = clientsholder.Client.CoreV1().Nodes().Update(context.Background(), NodeObject, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}

	return NodeObject, nil
}

// MatchingOptionalSelectorPTP filter the given slice with only the nodes matching the optional selector.
// If no selector is set, it returns the same list.
// The NODES_SELECTOR must be set with a labelselector expression.
// For example: NODES_SELECTOR="sctp=true"
func MatchingOptionalSelectorPTP(toFilter []ptpv1.NodePtpDevice) ([]ptpv1.NodePtpDevice, error) {
	if NodesSelector == "" {
		return toFilter, nil
	}
	toMatch, err := clientsholder.Client.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
		LabelSelector: NodesSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("Error in getting nodes matching the %s label selector, %v", NodesSelector, err)
	}
	if len(toMatch.Items) == 0 {
		return nil, fmt.Errorf("Failed to get nodes matching %s label selector", NodesSelector)
	}

	res := make([]ptpv1.NodePtpDevice, 0)
	for _, n := range toFilter {
		for _, m := range toMatch.Items {
			if n.Name == m.Name {
				res = append(res, n)
				break
			}
		}
	}
	if len(res) == 0 {
		return nil, fmt.Errorf("Failed to find matching nodes with %s label selector", NodesSelector)
	}
	return res, nil
}

func IsSingleNodeCluster() (bool, error) {

	nodes, err := clientsholder.Client.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return false, err
	}
	return len(nodes.Items) == 1, nil
}

// expectedReachabilityStatus true means test if the node is reachable, false means test if the node is unreachable
func WaitForNodeReachability(node *corev1.Node, timeout time.Duration, expectedReachabilityStatus bool) {
	isCurrentlyReachable := false

	for start := time.Now(); time.Since(start) < timeout; {
		isCurrentlyReachable = IsNodeReachable(node)

		if isCurrentlyReachable == expectedReachabilityStatus {
			break
		}
		if isCurrentlyReachable {
			logrus.Printf("The node %s is reachable via ping", node.Name)
		} else {
			logrus.Printf("The node %s is unreachable via ping", node.Name)
		}
		time.Sleep(time.Second)
	}
	if expectedReachabilityStatus {
		logrus.Printf("The node %s is reachable via ping", node.Name)
	} else {
		logrus.Printf("The node %s is unreachable via ping", node.Name)
	}
}

func IsNodeReachable(node *corev1.Node) bool {
	_, err := ExecAndLogCommand(false, 20*time.Second, "ping", "-c", "3", "-W", "10", node.Name)

	return err == nil
}

func ExecAndLogCommand(logCommand bool, timeout time.Duration, name string, arg ...string) ([]byte, error) {
	// Create a new context and add a timeout to it
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}

	ctx, cancel := context.WithTimeout(context.TODO(), timeout)

	defer cancel() // The cancel should be deferred so resources are cleaned up

	if logCommand {
		logrus.Printf("run command '%s %v'", name, arg)
	}

	out, err := exec.CommandContext(ctx, name, arg...).Output()

	// We want to check the context error to see if the timeout was executed.
	// The error returned by cmd.Output() will be OS specific based on what
	// happens when a process is killed.
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return nil, fmt.Errorf("command '%s %v' failed because of the timeout", name, arg)
	}

	if logCommand {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			log.Printf("err=%v:\n  stderr=%s\n  output=%s\n", err, exitError.Stderr, string(out))
		}
	}

	return out, err
}
