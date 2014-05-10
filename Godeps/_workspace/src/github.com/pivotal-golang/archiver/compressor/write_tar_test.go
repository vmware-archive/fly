package compressor_test

import (
	"archive/tar"
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/pivotal-golang/archiver/compressor"
)

var _ = Describe("WriteTar", func() {
	var srcPath string
	var buffer *bytes.Buffer
	var writeErr error

	BeforeEach(func() {
		dir, err := ioutil.TempDir("", "archive-dir")
		Ω(err).ShouldNot(HaveOccurred())

		err = os.Mkdir(filepath.Join(dir, "outer-dir"), 0755)
		Ω(err).ShouldNot(HaveOccurred())

		err = os.Mkdir(filepath.Join(dir, "outer-dir", "inner-dir"), 0755)
		Ω(err).ShouldNot(HaveOccurred())

		innerFile, err := os.Create(filepath.Join(dir, "outer-dir", "inner-dir", "some-file"))
		Ω(err).ShouldNot(HaveOccurred())

		_, err = innerFile.Write([]byte("sup"))
		Ω(err).ShouldNot(HaveOccurred())

		srcPath = filepath.Join(dir, "outer-dir")
		buffer = new(bytes.Buffer)
	})

	JustBeforeEach(func() {
		writeErr = WriteTar(srcPath, buffer)
	})

	It("returns a reader representing a .tar stream", func() {
		Ω(writeErr).ShouldNot(HaveOccurred())

		reader := tar.NewReader(buffer)

		header, err := reader.Next()
		Ω(err).ShouldNot(HaveOccurred())
		Ω(header.Name).Should(Equal("outer-dir/"))
		Ω(header.FileInfo().IsDir()).Should(BeTrue())

		header, err = reader.Next()
		Ω(err).ShouldNot(HaveOccurred())
		Ω(header.Name).Should(Equal("outer-dir/inner-dir/"))
		Ω(header.FileInfo().IsDir()).Should(BeTrue())

		header, err = reader.Next()
		Ω(err).ShouldNot(HaveOccurred())
		Ω(header.Name).Should(Equal("outer-dir/inner-dir/some-file"))
		Ω(header.FileInfo().IsDir()).Should(BeFalse())

		contents, err := ioutil.ReadAll(reader)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(string(contents)).Should(Equal("sup"))
	})

	Context("with a trailing slash", func() {
		BeforeEach(func() {
			srcPath = srcPath + "/"
		})

		It("archives the directory's contents", func() {
			Ω(writeErr).ShouldNot(HaveOccurred())

			reader := tar.NewReader(buffer)

			header, err := reader.Next()
			Ω(err).ShouldNot(HaveOccurred())
			Ω(header.Name).Should(Equal("./"))
			Ω(header.FileInfo().IsDir()).Should(BeTrue())

			header, err = reader.Next()
			Ω(err).ShouldNot(HaveOccurred())
			Ω(header.Name).Should(Equal("inner-dir/"))
			Ω(header.FileInfo().IsDir()).Should(BeTrue())

			header, err = reader.Next()
			Ω(err).ShouldNot(HaveOccurred())
			Ω(header.Name).Should(Equal("inner-dir/some-file"))
			Ω(header.FileInfo().IsDir()).Should(BeFalse())

			contents, err := ioutil.ReadAll(reader)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(string(contents)).Should(Equal("sup"))
		})
	})

	Context("with a single file", func() {
		BeforeEach(func() {
			srcPath = filepath.Join(srcPath, "inner-dir", "some-file")
		})

		It("archives the single file at the root", func() {
			Ω(writeErr).ShouldNot(HaveOccurred())

			reader := tar.NewReader(buffer)

			header, err := reader.Next()
			Ω(err).ShouldNot(HaveOccurred())
			Ω(header.Name).Should(Equal("some-file"))
			Ω(header.FileInfo().IsDir()).Should(BeFalse())

			contents, err := ioutil.ReadAll(reader)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(string(contents)).Should(Equal("sup"))
		})
	})

	Context("when there is no file at the given path", func() {
		BeforeEach(func() {
			srcPath = filepath.Join(srcPath, "barf")
		})

		It("returns an error", func() {
			Ω(writeErr).Should(BeAssignableToTypeOf(&os.PathError{}))
		})
	})
})
