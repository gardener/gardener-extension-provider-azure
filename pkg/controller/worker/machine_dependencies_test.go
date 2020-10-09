package worker_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener-extension-provider-azure/pkg/controller/worker"

	"github.com/gardener/gardener/extensions/pkg/controller/common"
	"github.com/gardener/gardener/extensions/pkg/controller/worker/genericactuator"
)

var _ = Describe("MachinesDependencies", func() {
	var workerDelegate genericactuator.WorkerDelegate

	BeforeEach(func() {
		workerDelegate, _ = NewWorkerDelegate(common.NewClientContext(nil, nil, nil), nil, "", nil, nil)
	})

	Context("#DeployMachineDependencies", func() {
		It("should return no error", func() {
			err := workerDelegate.DeployMachineDependencies(context.TODO())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("#CleanupMachineDependencies", func() {
		It("should return no error", func() {
			err := workerDelegate.CleanupMachineDependencies(context.TODO())
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
