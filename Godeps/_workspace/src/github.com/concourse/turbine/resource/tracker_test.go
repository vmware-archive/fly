package resource_test

import (
	"errors"

	"github.com/cloudfoundry-incubator/garden/client/fake_warden_client"
	"github.com/cloudfoundry-incubator/garden/warden"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/turbine/config"
	. "github.com/concourse/turbine/resource"
)

var _ = Describe("Tracker", func() {
	var (
		resourceTypes config.ResourceTypes
		wardenClient  *fake_warden_client.FakeClient

		tracker Tracker
	)

	BeforeEach(func() {
		resourceTypes = config.ResourceTypes{
			{Name: "type1", Image: "image1"},
			{Name: "type2", Image: "image2"},
		}

		wardenClient = fake_warden_client.New()

		wardenClient.Connection.CreateReturns("some-handle", nil)

		tracker = NewTracker(resourceTypes, wardenClient)
	})

	Describe("Init", func() {
		var (
			initType string

			initResource Resource
			initErr      error
		)

		BeforeEach(func() {
			initType = "type1"
		})

		JustBeforeEach(func() {
			initResource, initErr = tracker.Init(initType, nil, nil)
		})

		It("does not error and returns a resource", func() {
			Ω(initErr).ShouldNot(HaveOccurred())
			Ω(initResource).ShouldNot(BeNil())
		})

		It("creates a container with the resource type's image", func() {
			Ω(wardenClient.Connection.CreateArgsForCall(0)).Should(Equal(warden.ContainerSpec{
				RootFSPath: "image1",
			}))
		})

		Context("when creating the container fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				wardenClient.Connection.CreateReturns("", disaster)
			})

			It("returns the error and no resource", func() {
				Ω(initErr).Should(Equal(disaster))
				Ω(initResource).Should(BeNil())
			})
		})

		Context("with an unknown resource type", func() {
			BeforeEach(func() {
				initType = "bogus-type"
			})

			It("returns ErrUnknownResourceType", func() {
				Ω(initErr).Should(Equal(ErrUnknownResourceType))
			})
		})
	})

	Describe("Release", func() {
		var (
			releaseResource Resource

			releaseErr error
		)

		BeforeEach(func() {
			var err error

			releaseResource, err = tracker.Init("type1", nil, nil)
			Ω(err).ShouldNot(HaveOccurred())
		})

		JustBeforeEach(func() {
			releaseErr = tracker.Release(releaseResource)
		})

		It("does not error", func() {
			Ω(releaseErr).ShouldNot(HaveOccurred())
		})

		It("destroys the container associated with the resource", func() {
			Ω(wardenClient.Connection.DestroyCallCount()).Should(Equal(1))
			Ω(wardenClient.Connection.DestroyArgsForCall(0)).Should(Equal("some-handle"))
		})

		Context("when destroying the container fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				wardenClient.Connection.DestroyReturns(disaster)
			})

			It("returns the error", func() {
				Ω(releaseErr).Should(Equal(disaster))
			})
		})

		Context("with an unknown resource", func() {
			BeforeEach(func() {
				releaseResource = nil
			})

			It("returns no error", func() {
				Ω(releaseErr).ShouldNot(HaveOccurred())
			})

			It("destroys no containers", func() {
				Ω(wardenClient.Connection.DestroyCallCount()).Should(BeZero())
			})
		})
	})
})
