//go:build !unittests
// +build !unittests

package test_test

import (
	"flag"
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"

	_ "github.com/openshift/ptp-operator/test/conformance/ptp"
	testclient "github.com/openshift/ptp-operator/test/utils/client"
)

// TODO: we should refactor tests to use client from controller-runtime package
// see - https://github.com/openshift/cluster-api-actuator-pkg/blob/master/pkg/e2e/framework/framework.go

var junitPath *string

func init() {
	junitPath = flag.String("junit", "junit.xml", "the path for the junit format report")
}

func TestTest(t *testing.T) {
	RegisterFailHandler(Fail)

	rr := []Reporter{}
	if junitPath != nil {
		rr = append(rr, reporters.NewJUnitReporter(*junitPath))
	}
	RunSpecsWithDefaultAndCustomReporters(t, "PTP e2e integration tests", rr)
}

var _ = BeforeSuite(func() {
	Expect(testclient.Client).NotTo(BeNil())
})

var _ = AfterSuite(func() {
})
