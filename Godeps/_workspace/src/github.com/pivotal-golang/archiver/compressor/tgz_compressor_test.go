package compressor_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/pivotal-golang/archiver/compressor"
	"github.com/pivotal-golang/archiver/extractor"
)

func retrieveFilePaths(dir string) (results []string) {
	err := filepath.Walk(dir, func(singlePath string, info os.FileInfo, err error) error {
		relative, err := filepath.Rel(dir, singlePath)
		Ω(err).ShouldNot(HaveOccurred())

		results = append(results, relative)
		return nil
	})

	Ω(err).NotTo(HaveOccurred())

	return results
}

var _ = Describe("Tgz Compressor", func() {
	var compressor Compressor
	var destDir string
	var extracticator extractor.Extractor
	var victimFile *os.File
	var victimDir string

	BeforeEach(func() {
		var err error

		compressor = NewTgz()
		extracticator = extractor.NewDetectable()

		destDir, err = ioutil.TempDir("", "")
		Ω(err).ShouldNot(HaveOccurred())

		victimDir, err = ioutil.TempDir("", "")
		Ω(err).ShouldNot(HaveOccurred())

		victimFile, err = ioutil.TempFile("", "")
		Ω(err).ShouldNot(HaveOccurred())

		err = os.Mkdir(filepath.Join(victimDir, "empty"), 0755)
		Ω(err).ShouldNot(HaveOccurred())

		notEmptyDirPath := filepath.Join(victimDir, "not_empty")

		err = os.Mkdir(notEmptyDirPath, 0755)
		Ω(err).ShouldNot(HaveOccurred())

		err = ioutil.WriteFile(filepath.Join(notEmptyDirPath, "some_file"), []byte("stuff"), 0644)
		Ω(err).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(destDir)
		os.RemoveAll(victimDir)
		os.Remove(victimFile.Name())
	})

	It("compresses the src file to dest file", func() {
		srcFile := victimFile.Name()

		destFile := filepath.Join(destDir, "compress-dst.tgz")

		err := compressor.Compress(srcFile, destFile)
		Ω(err).NotTo(HaveOccurred())

		finalReadingDir, err := ioutil.TempDir(destDir, "final")
		Ω(err).NotTo(HaveOccurred())

		defer os.RemoveAll(finalReadingDir)

		err = extracticator.Extract(destFile, finalReadingDir)
		Ω(err).NotTo(HaveOccurred())

		expectedContent, err := ioutil.ReadFile(srcFile)
		Ω(err).NotTo(HaveOccurred())

		actualContent, err := ioutil.ReadFile(filepath.Join(finalReadingDir, filepath.Base(srcFile)))
		Ω(err).NotTo(HaveOccurred())
		Ω(actualContent).Should(Equal(expectedContent))
	})

	It("compresses the src path recursively to dest file", func() {
		srcDir := victimDir

		destFile := filepath.Join(destDir, "compress-dst.tgz")

		err := compressor.Compress(srcDir, destFile)
		Ω(err).ShouldNot(HaveOccurred())

		finalReadingDir, err := ioutil.TempDir(destDir, "final")
		Ω(err).ShouldNot(HaveOccurred())

		err = extracticator.Extract(destFile, finalReadingDir)
		Ω(err).ShouldNot(HaveOccurred())

		expectedFilePaths := retrieveFilePaths(srcDir)
		actualFilePaths := retrieveFilePaths(finalReadingDir)

		Ω(actualFilePaths).To(Equal(expectedFilePaths))

		emptyDir, err := os.Open(filepath.Join(finalReadingDir, "empty"))
		Ω(err).NotTo(HaveOccurred())

		emptyDirInfo, err := emptyDir.Stat()
		Ω(err).NotTo(HaveOccurred())

		Ω(emptyDirInfo.IsDir()).To(BeTrue())
	})
})
