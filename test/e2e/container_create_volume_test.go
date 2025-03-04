package integration

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

func buildDataVolumeImage(pTest *PodmanTestIntegration, image, data, dest string) {
	// Create a dummy file for data volume
	dummyFile := filepath.Join(pTest.TempDir, data)
	err := os.WriteFile(dummyFile, []byte(data), 0644)
	Expect(err).To(BeNil())

	// Create a data volume container image but no CMD binary in it
	containerFile := fmt.Sprintf(`FROM scratch
CMD doesnotexist.sh
ADD %s %s/
VOLUME %s/`, data, dest, dest)
	pTest.BuildImage(containerFile, image, "false")
}

func createContainersConfFile(pTest *PodmanTestIntegration) {
	configPath := filepath.Join(pTest.TempDir, "containers.conf")
	containersConf := []byte("[containers]\nprepare_volume_on_create = true\n")
	err := os.WriteFile(configPath, containersConf, os.ModePerm)
	Expect(err).To(BeNil())

	// Set custom containers.conf file
	os.Setenv("CONTAINERS_CONF", configPath)
	if IsRemote() {
		pTest.RestartRemoteService()
	}
}

func checkDataVolumeContainer(pTest *PodmanTestIntegration, image, cont, dest, data string) {
	create := pTest.Podman([]string{"create", "--name", cont, image})
	create.WaitWithDefaultTimeout()
	Expect(create).Should(Exit(0))

	inspect := pTest.InspectContainer(cont)
	Expect(inspect).To(HaveLen(1))
	Expect(inspect[0].Mounts).To(HaveLen(1))
	Expect(inspect[0].Mounts[0]).To(HaveField("Destination", dest))

	mntName, mntSource := inspect[0].Mounts[0].Name, inspect[0].Mounts[0].Source

	volList := pTest.Podman([]string{"volume", "list", "--quiet"})
	volList.WaitWithDefaultTimeout()
	Expect(volList).Should(Exit(0))
	Expect(volList.OutputToStringArray()).To(HaveLen(1))
	Expect(volList.OutputToStringArray()[0]).To(Equal(mntName))

	// Check the mount source directory
	files, err := os.ReadDir(mntSource)
	Expect(err).To(BeNil())

	if data == "" {
		Expect(files).To(BeEmpty())
	} else {
		Expect(files).To(HaveLen(1))
		Expect(files[0].Name()).To(Equal(data))
	}
}

var _ = Describe("Podman create data volume", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.Setup()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)
		os.Unsetenv("CONTAINERS_CONF")
	})

	It("podman create with volume data copy turned off", func() {
		imgName, volData, volDest := "dataimg", "dummy", "/test"

		buildDataVolumeImage(podmanTest, imgName, volData, volDest)

		// Create a container with the default containers.conf and
		// check that the volume is not copied from the image.
		checkDataVolumeContainer(podmanTest, imgName, "ctr-nocopy", volDest, "")
	})

	It("podman create with volume data copy turned on", func() {
		imgName, volData, volDest := "dataimg", "dummy", "/test"

		buildDataVolumeImage(podmanTest, imgName, volData, volDest)

		// Create a container with the custom containers.conf and
		// check that the volume is copied from the image.
		createContainersConfFile(podmanTest)

		checkDataVolumeContainer(podmanTest, imgName, "ctr-copy", volDest, volData)
	})

	It("podman run with volume data copy turned on", func() {
		// Create a container with the custom containers.conf and
		// check that the container is run successfully
		createContainersConfFile(podmanTest)

		session := podmanTest.Podman([]string{"run", "--rm", ALPINE, "echo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})
})
