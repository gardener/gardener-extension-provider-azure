package backupbucket_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestBackupbucket(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Backupbucket Suite")
}
