package l2discovery

import (




	l2 "github.com/test-network-function/l2discovery/export"
	//"github.com/test-network-function/l2discovery/l2lib"

	"github.com/sirupsen/logrus"
	"github.com/yourbasic/graph"
	v1core "k8s.io/api/core/v1"
)

const (
	
	FirstSolution                  = 0
)

type PtpAlgo int64

const NumAlgos = 8
const (

	// AlgoOC OrdinaryClock algorithm
	AlgoOC PtpAlgo = iota
	// AlgoBC Boundary Clock algorithm
	AlgoBC
	// AlgoBCWithSlaves Boundary Clock with slaves algorithm
	AlgoBCWithSlaves
	// AlgoBCWithSlaves Dual NIC Boundary Clock algorithm
	AlgoDualNicBC
	// AlgoBCWithSlaves Dual NIC Boundary Clock with slaves algorithm
	AlgoDualNicBCWithSlaves
	// AlgoOC SNO OrdinaryClock algorithm
	AlgoSNOOC
	// AlgoBC SNO Boundary Clock algorithm
	AlgoSNOBC
	// AlgoBCWithSlaves SNO Dual NIC Boundary Clock algorithm
	AlgoSNODualNicBC
)

const (
	AlgoOCString                  = "OC"
	AlgoBCString                  = "BC"
	AlgoBCWithSlavesString        = "BCWithSlaves"
	AlgoDualNicBCString           = "DualNicBC"
	AlgoDualNicBCWithSlavesString = "DualNicBCWithSlaves"
	AlgoSNOOCString               = "SNOOC"
	AlgoSNOBCString               = "SNOBC"
	AlgoSNODualNicBCString        = "SNODualNicBC"
)

func (algo PtpAlgo) String() string {
	switch algo {
	case AlgoOC:
		return AlgoOCString
	case AlgoBC:
		return AlgoBCString
	case AlgoBCWithSlaves:
		return AlgoBCWithSlavesString
	case AlgoDualNicBC:
		return AlgoDualNicBCString
	case AlgoDualNicBCWithSlaves:
		return AlgoDualNicBCWithSlavesString
	case AlgoSNOOC:
		return AlgoSNOOCString
	case AlgoSNOBC:
		return AlgoSNOBCString
	case AlgoSNODualNicBC:
		return AlgoSNODualNicBCString

	default:
		return ""
	}
}

type L2DaemonsetMode int64

const (
	// In managed mode, the L2 Topology discovery Daemonset is created by the conformance suite
	Managed L2DaemonsetMode = iota
	// In pre-configured mode, the L2 topology daemonset is pre-configured by the user in the cluster
	PreConfigured
)

var GlobalL2DiscoveryConfig L2DiscoveryConfig

// Object used to index interfaces in a cluster
type IfClusterIndex struct {
	// interface name
	IfName string
	// node name
	NodeName string
}

// Object representing a ptp interface within a cluster.
type PtpIf struct {
	// Mac address of the Ethernet interface
	MacAddress string
	// Index of the interface in the cluster (node/interface name)
	IfClusterIndex
	// PCI address
	IfPci l2.PCIAddress
}

// Object representing the calculated clock configuration
type ClockConfig struct {
	// Grandmaster selected cluster interface
	Grandmaster *PtpIf
	// Slave selected cluster interface (OC)
	Slave []*PtpIf
	// BC Master selected interface (BC)
	BcMaster []*PtpIf
	// BC slave selected interface (BC)
	BcSlave []*PtpIf
}

type L2DiscoveryConfig struct {
	// Map of L2 topology as discovered by L2 discovery mechanism
	DiscoveryMap map[string]map[string]map[string]*l2.Neighbors
	// L2 topology graph created from discovery map. This is the main internal graph
	L2ConnectivityMap *graph.Mutable
	// Max size of graph
	MaxL2GraphSize int
	// list of cluster interfaces indexed with a simple integer (X) for readability in the graph
	PtpIfList []*PtpIf
	// list of L2discovery daemonset pods
	L2DiscoveryPods map[string]*v1core.Pod
	// Mapping between clusterwide interface index and Mac address
	ClusterMacs map[IfClusterIndex]string
	// Mapping between clusterwide interface index and a simple integer (X) for readability in the graph
	ClusterIndexToInt map[IfClusterIndex]int
	// Mapping between a cluster wide MAC address and a simple integer (X) for readability in the graph
	ClusterMacToInt map[string]int
	// Mapping between a Mac address and a cluster wide interface index
	ClusterIndexes map[string]IfClusterIndex
	// 2D Map holding the valid ptp interfaces as reported by the ptp-operator api. map[ <node name>]map[<interface name>]
	ptpInterfaces map[string]map[string]bool
	// indicates whether the L2discovery daemonset is created by the test suite (managed) or not
	L2DsMode L2DaemonsetMode
	// islands identified in the graph
	Islands *[][]int
	// List of port receiving PTP frames (assuming valid GM signal received)
	PortsGettingPTP []*PtpIf
	// map storing solutions
	Solutions [NumAlgos][][]int
	// Mapping between clock role and port depending on the algo
	TestClockRolesAlgoMapping [NumAlgos][NumTestClockRoles]int
	// interfaces to avoid when running the tests
	SkippedInterfaces []string
	// Indicates that the L2 configuration must be refreshed
	refresh bool
}

// indicates the clock roles in the algotithms
type TestIfClockRoles int

const NumTestClockRoles = 7
const (
	Grandmaster TestIfClockRoles = iota
	Slave1
	Slave2
	BC1Master
	BC1Slave
	BC2Master
	BC2Slave
)

// list of Algorithm functions with zero params
type AlgoFunction0 int

// See applyStep
const (
	// same node
	StepNil AlgoFunction0 = iota
)

// list of Algorithm function with 1 params
type AlgoFunction1 int

// See applyStep
const (
	// same node
	StepIsPTP AlgoFunction1 = iota
)

// list of Algorithm function with 2 params
type AlgoFunction2 int

// See applyStep
const (
	StepSameIsland2 AlgoFunction2 = iota
	StepSameNic
	StepSameNode
	StepDifferentNode
	StepDifferentNic
)

// list of Algorithm function with 3 params
type AlgoFunction3 int

// See applyStep
const (
	StepSameIsland3 AlgoFunction3 = iota
)

// Signature for algorithm functions with 0 params
type ConfigFunc0 func() bool

// Signature for algorithm functions with 1 params
type ConfigFunc1 func(*L2DiscoveryConfig, int) bool

// Signature for algorithm functions with 2 params
type ConfigFunc2 func(*L2DiscoveryConfig, int, int) bool

// Signature for algorithm functions with 3 params
type ConfigFunc3 func(*L2DiscoveryConfig, int, int, int) bool

type ConfigFunc func(*L2DiscoveryConfig, []int) bool
type Algorithm struct {
	// number of interfaces to solve
	IfCount int
	// Function to run algo
	TestSolution ConfigFunc
}

// Discovers the L2 connectivity using l2discovery daemonset
/*func (config *L2DiscoveryConfig) DiscoverL2Connectivity(client *client.ClientSet) error {
	GlobalL2DiscoveryConfig.reset()
	GlobalL2DiscoveryConfig.InitSkippedInterfaces()
	nodeDevicesList, err := client.NodePtpDevices(utils.PtpLinuxDaemonNamespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	if len(nodeDevicesList.Items) == 0 {
		return fmt.Errorf("Zero nodes found")
	}

	// initializes clusterwide ptp interfaces
	config.ptpInterfaces, err = buildPtpIfaces(client)
	if err != nil {
		logrus.Errorf("could not retrieve ptp interface list")
	}

	// Create L2 discovery daemonset
	config.L2DsMode = StringToL2Mode(os.Getenv("L2_DAEMONSET"))
	if config.L2DsMode == Managed {
		_, err = daemonsets.CreateDaemonSet(L2DiscoveryDsName, L2DiscoveryNsName, L2DiscoveryContainerName, l2DiscoveryImage, timeoutDaemon)
		if err != nil {
			logrus.Errorf("error creating l2 discovery daemonset, err=%s", err)
		}
	}

	isSingleNode, err := nodes.IsSingleNodeCluster()
	if err != nil {
		return err
	}

	if isSingleNode {
		// Sleep a short time to allow discovery to happen (first report after 5s)
		time.Sleep(L2DiscoveryDurationSNO)
	} else {
		// Sleep a short time to allow discovery to happen (first report after 5s)
		time.Sleep(L2DiscoveryDuration)
	}

	// Get the L2 topology pods
	err = GlobalL2DiscoveryConfig.getL2TopologyDiscoveryPods()
	if err != nil {
		return fmt.Errorf("could not get linkloop pods, err=%s", err)
	}

	err = config.getL2Disc()
	if err != nil {
		logrus.Errorf("error getting l2 discovery data, err=%s", err)
	}

	// Delete L2 discovery daemonset
	if config.L2DsMode == Managed {
		err = daemonsets.DeleteDaemonSet(L2DiscoveryDsName, L2DiscoveryNsName)
		if err != nil {
			logrus.Errorf("error deleting l2 discovery daemonset, err=%s", err)
		}
	}
	// Create a graph from the discovered data
	err = config.createL2InternalGraph()
	if err != nil {
		return err
	}
	config.SolveConfig()

	return nil
}
*/

// Print a single solution
func (config *L2DiscoveryConfig) PrintSolution(p []int) {
	i := 0
	for _, aIf := range p {
		logrus.Infof("p%d= %s", i, config.PtpIfList[aIf])
		i++
	}
}

// Prints the selected solution for each scenario, if found
func (config *L2DiscoveryConfig) PrintOneSolutionPerScenario() {
	for scenario, solutions := range config.Solutions {

		if len(solutions) == 0 {
			logrus.Infof("Solution for %s scenario does not exists", PtpAlgo(scenario))
			continue
		}
		logrus.Infof("Solution for %s scenario", PtpAlgo(scenario))
		config.PrintSolution(solutions[FirstSolution])

	}
}

// Recursive solver function. Creates a set of permutations and applies contraints at each step to
// reduce the solution graph and speed up execution
func permutationsWithConstraints(config *L2DiscoveryConfig, algo [][][]int, l []int, s, e, n int, result bool, solutions *[][]int) {
	if !result {
		return
	}
	if s == e {
		temp := make([]int, 0)
		temp = append(temp, l...)
		temp = temp[0:e]
		logrus.Debugf("%v --  %v", temp, result)
		// config.PrintSolution(temp)
		*solutions = append(*solutions, temp)
	} else {
		// Backtracking loop
		for i := s; i < n; i++ {
			l[i], l[s] = l[s], l[i]
			result = applyStep(config, algo[s], l[0:e])
			permutationsWithConstraints(config, algo, l, s+1, e, n, result, solutions)
			l[i], l[s] = l[s], l[i]
		}
	}
}

// check if an interface is receiving GM
func (config *L2DiscoveryConfig) IsPTP(aInterface *PtpIf) bool {
	for _, aIf := range config.PortsGettingPTP {
		if aInterface.IfClusterIndex == aIf.IfClusterIndex {
			return true
		}
	}
	return false
}

// Checks that an if an interface receives ptp frames
func IsPTPWrapper(config *L2DiscoveryConfig, if1 int) bool {
	return config.IsPTP(config.PtpIfList[if1])
}

// Checks if 2 interfaces are on the same node
func SameNode(if1, if2 *PtpIf) bool {
	return if1.NodeName == if2.NodeName
}

// algo Wrapper for SameNode
func SameNodeWrapper(config *L2DiscoveryConfig, if1, if2 int) bool {
	return SameNode(config.PtpIfList[if1], config.PtpIfList[if2])
}

// algo wrapper for !SameNode
func DifferentNodeWrapper(config *L2DiscoveryConfig, if1, if2 int) bool {
	return !SameNode(config.PtpIfList[if1], config.PtpIfList[if2])
}

// Algo wrapper for !SameNic
func DifferentNicWrapper(config *L2DiscoveryConfig, if1, if2 int) bool {
	return !SameNic(config.PtpIfList[if1], config.PtpIfList[if2])
}

// Checks if 3 interfaces are connected to the same LAN
func SameIsland3(config *L2DiscoveryConfig, if1, if2, if3 int, islands *[][]int) bool {
	if SameNode(config.PtpIfList[if1], config.PtpIfList[if2]) ||
		SameNode(config.PtpIfList[if1], config.PtpIfList[if3]) {
		return false
	}
	for _, island := range *islands {
		if1Present := false
		if2Present := false
		if3Present := false
		for _, aIf := range island {
			if aIf == if1 {
				if1Present = true
			}
			if aIf == if2 {
				if2Present = true
			}
			if aIf == if3 {
				if3Present = true
			}
		}
		if if1Present && if2Present && if3Present {
			return true
		}
	}
	return false
}

// algo wrapper for SameIsland3
func SameIsland3Wrapper(config *L2DiscoveryConfig, if1, if2, if3 int) bool {
	return SameIsland3(config, if1, if2, if3, config.Islands)
}

// Checks if 2 interfaces are connected to the same LAN
func SameIsland2(config *L2DiscoveryConfig, if1, if2 int, islands *[][]int) bool {
	if SameNode(config.PtpIfList[if1], config.PtpIfList[if2]) {
		return false
	}
	for _, island := range *islands {
		if1Present := false
		if2Present := false
		for _, aIf := range island {
			if aIf == if1 {
				if1Present = true
			}
			if aIf == if2 {
				if2Present = true
			}
		}
		if if1Present && if2Present {
			return true
		}
	}
	return false
}

// wrapper for SameIsland2
func SameIsland2Wrapper(config *L2DiscoveryConfig, if1, if2 int) bool {
	return SameIsland2(config, if1, if2, config.Islands)
}

// Determines if 2 interfaces (ports) belong to the same NIC
func SameNic(ifaceName1, ifaceName2 *PtpIf) bool {
	if ifaceName1.IfClusterIndex.NodeName != ifaceName2.IfClusterIndex.NodeName {
		return false
	}
	return ifaceName1.IfPci.Device != "" && ifaceName1.IfPci.Device == ifaceName2.IfPci.Device
}

// wrapper for SameNic
func SameNicWrapper(config *L2DiscoveryConfig, if1, if2 int) bool {
	return SameNic(config.PtpIfList[if1], config.PtpIfList[if2])
}

// wrapper for nil algo function
func NilWrapper() bool {
	return true
}

// Applies a single step (constraint) in the backtracking algorithm
func applyStep(config *L2DiscoveryConfig, step [][]int, combinations []int) bool {
	type paramNum int

	const (
		NoParam paramNum = iota
		OneParam
		TwoParams
		ThreeParams
		FourParams
	)
	// mapping table between :
	// AlgoFunction0, AlgoFunction1, AlgoFunction2, AlgoFunction3 and
	// function wrappers

	var AlgoCode0 [1]ConfigFunc0
	AlgoCode0[StepNil] = NilWrapper

	var AlgoCode1 [1]ConfigFunc1
	AlgoCode1[StepIsPTP] = IsPTPWrapper

	var AlgoCode2 [5]ConfigFunc2
	AlgoCode2[StepSameIsland2] = SameIsland2Wrapper
	AlgoCode2[StepSameNic] = SameNicWrapper
	AlgoCode2[StepSameNode] = SameNodeWrapper
	AlgoCode2[StepDifferentNode] = DifferentNodeWrapper
	AlgoCode2[StepDifferentNic] = DifferentNicWrapper

	var AlgoCode3 [1]ConfigFunc3
	AlgoCode3[StepSameIsland3] = SameIsland3Wrapper

	result := true
	for _, test := range step {
		switch test[1] {
		case int(NoParam):
			result = result && AlgoCode0[test[0]]()
		case int(OneParam):
			result = result && AlgoCode1[test[0]](config, combinations[test[2]])
		case int(TwoParams):
			result = result && AlgoCode2[test[0]](config, combinations[test[2]], combinations[test[3]])
		case int(ThreeParams):
			result = result && AlgoCode3[test[0]](config, combinations[test[2]], combinations[test[3]], combinations[test[4]])
		}
	}
	return result
}

// Runs Solver to find optimal configurations
func (config *L2DiscoveryConfig) SolveConfig() {

	// Initializing Algorithms

	OCAlgo := [][][]int{
		{{int(StepNil), 0, 0}},            // step1
		{{int(StepSameIsland2), 2, 0, 1}}, // step2
	}
	BCAlgo := [][][]int{
		{{int(StepNil), 0, 0}},            // step1
		{{int(StepSameNic), 2, 0, 1}},     // step2
		{{int(StepSameIsland2), 2, 1, 2}}, // step3

	}
	BCAlgoWithSlaves := [][][]int{
		{{int(StepNil), 0, 0}},            // step1
		{{int(StepSameIsland2), 2, 0, 1}}, // step2
		{{int(StepSameNic), 2, 1, 2}},     // step3
		{{int(StepSameIsland2), 2, 2, 3}}, // step4
	}
	DualNicBCAlgo := [][][]int{
		{{int(StepNil), 0, 0}},            // step1
		{{int(StepSameNic), 2, 0, 1}},     // step2
		{{int(StepSameIsland2), 2, 1, 2}}, // step3
		{{int(StepSameNode), 2, 1, 3}, // step4
			{int(StepSameIsland2), 2, 2, 3}}, // step4
		{{int(StepSameNic), 2, 3, 4}}, // step5
	}
	DualNicBCAlgoWithSlaves := [][][]int{
		{{int(StepNil), 0, 0}},            // step1
		{{int(StepSameIsland2), 2, 0, 1}}, // step2
		{{int(StepSameNic), 2, 1, 2}},     // step3
		{{int(StepSameIsland2), 2, 2, 3}}, // step4
		{{int(StepSameNode), 2, 2, 4}, // step5
			{int(StepSameIsland2), 2, 3, 4}}, // step5
		{{int(StepSameNic), 2, 4, 5}},     // step6
		{{int(StepSameIsland2), 2, 5, 6}}, // step7
	}
	SNOOCAlgo := [][][]int{
		{{int(StepIsPTP), 1, 0}}, // step1
	}
	SNOBCAlgo := [][][]int{
		{{int(StepIsPTP), 1, 0}},      // step1
		{{int(StepSameNic), 2, 0, 1}}, // step2
	}
	SNODualNicBCAlgo := [][][]int{
		{{int(StepIsPTP), 1, 0}},      // step1
		{{int(StepSameNic), 2, 0, 1}}, // step2
		{{int(StepIsPTP), 1, 2}, // step3
			{int(StepSameNode), 2, 0, 2}}, // step3
		{{int(StepSameNic), 2, 2, 3}}, // step4

	}

	// Initializing Solution decoding and mapping

	// OC
	config.TestClockRolesAlgoMapping[AlgoOC][Slave1] = 0
	config.TestClockRolesAlgoMapping[AlgoOC][Grandmaster] = 1

	// BC

	config.TestClockRolesAlgoMapping[AlgoBC][BC1Slave] = 0
	config.TestClockRolesAlgoMapping[AlgoBC][BC1Master] = 1
	config.TestClockRolesAlgoMapping[AlgoBC][Grandmaster] = 2

	// BC with slaves

	config.TestClockRolesAlgoMapping[AlgoBCWithSlaves][Slave1] = 0
	config.TestClockRolesAlgoMapping[AlgoBCWithSlaves][BC1Master] = 1
	config.TestClockRolesAlgoMapping[AlgoBCWithSlaves][BC1Slave] = 2
	config.TestClockRolesAlgoMapping[AlgoBCWithSlaves][Grandmaster] = 3

	// Dual NIC BC
	config.TestClockRolesAlgoMapping[AlgoDualNicBC][BC1Slave] = 0
	config.TestClockRolesAlgoMapping[AlgoDualNicBC][BC1Master] = 1
	config.TestClockRolesAlgoMapping[AlgoDualNicBC][Grandmaster] = 2
	config.TestClockRolesAlgoMapping[AlgoDualNicBC][BC2Master] = 3
	config.TestClockRolesAlgoMapping[AlgoDualNicBC][BC2Slave] = 4

	// Dual NIC BC with slaves
	config.TestClockRolesAlgoMapping[AlgoDualNicBCWithSlaves][Slave1] = 0
	config.TestClockRolesAlgoMapping[AlgoDualNicBCWithSlaves][BC1Master] = 1
	config.TestClockRolesAlgoMapping[AlgoDualNicBCWithSlaves][BC1Slave] = 2
	config.TestClockRolesAlgoMapping[AlgoDualNicBCWithSlaves][Grandmaster] = 3
	config.TestClockRolesAlgoMapping[AlgoDualNicBCWithSlaves][BC2Slave] = 4
	config.TestClockRolesAlgoMapping[AlgoDualNicBCWithSlaves][BC2Master] = 5
	config.TestClockRolesAlgoMapping[AlgoDualNicBCWithSlaves][Slave2] = 6

	// SNO OC
	config.TestClockRolesAlgoMapping[AlgoSNOOC][Slave1] = 0

	// SNO BC
	config.TestClockRolesAlgoMapping[AlgoSNOBC][BC1Slave] = 0
	config.TestClockRolesAlgoMapping[AlgoSNOBC][BC1Master] = 1

	// SNO Dual NIC BC

	config.TestClockRolesAlgoMapping[AlgoSNODualNicBC][BC1Slave] = 0
	config.TestClockRolesAlgoMapping[AlgoSNODualNicBC][BC1Master] = 1
	config.TestClockRolesAlgoMapping[AlgoSNODualNicBC][BC2Slave] = 2
	config.TestClockRolesAlgoMapping[AlgoSNODualNicBC][BC2Master] = 3

	// Initializing solution slice
	algos := [][][][]int{
		OCAlgo,
		BCAlgo,
		BCAlgoWithSlaves,
		DualNicBCAlgo,
		DualNicBCAlgoWithSlaves,
		SNOOCAlgo,
		SNOBCAlgo,
		SNODualNicBCAlgo,
	}

	L := []int{}
	for i := 0; i < config.MaxL2GraphSize; i++ {
		L = append(L, i)
	}

	// Running solver per Algorithm
	//Multi node
	permutationsWithConstraints(config, algos[0], L, 0, len(algos[0]), len(L), true, &config.Solutions[AlgoOC])
	permutationsWithConstraints(config, algos[1], L, 0, len(algos[1]), len(L), true, &config.Solutions[AlgoBC])
	permutationsWithConstraints(config, algos[2], L, 0, len(algos[2]), len(L), true, &config.Solutions[AlgoBCWithSlaves])
	permutationsWithConstraints(config, algos[3], L, 0, len(algos[3]), len(L), true, &config.Solutions[AlgoDualNicBC])
	permutationsWithConstraints(config, algos[4], L, 0, len(algos[4]), len(L), true, &config.Solutions[AlgoDualNicBCWithSlaves])
	// SNO
	permutationsWithConstraints(config, algos[5], L, 0, len(algos[5]), len(L), true, &config.Solutions[AlgoSNOOC])
	permutationsWithConstraints(config, algos[6], L, 0, len(algos[6]), len(L), true, &config.Solutions[AlgoSNOBC])
	permutationsWithConstraints(config, algos[7], L, 0, len(algos[7]), len(L), true, &config.Solutions[AlgoSNODualNicBC])

	// print selected solutions
	config.PrintOneSolutionPerScenario()
}
