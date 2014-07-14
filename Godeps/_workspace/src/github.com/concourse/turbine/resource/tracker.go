package resource

import (
	"errors"
	"io"
	"sync"

	"github.com/cloudfoundry-incubator/garden/warden"
	"github.com/concourse/turbine/config"
)

type Tracker interface {
	Init(typ string, logs io.Writer, abort <-chan struct{}) (Resource, error)
	Release(Resource) error
}

type tracker struct {
	resourceTypes config.ResourceTypes
	wardenClient  warden.Client

	containers  map[Resource]warden.Container
	containersL *sync.Mutex
}

var ErrUnknownResourceType = errors.New("unknown resource type")

func NewTracker(resourceTypes config.ResourceTypes, wardenClient warden.Client) Tracker {
	return &tracker{
		resourceTypes: resourceTypes,
		wardenClient:  wardenClient,

		containers:  make(map[Resource]warden.Container),
		containersL: new(sync.Mutex),
	}
}

func (tracker *tracker) Init(typ string, logs io.Writer, abort <-chan struct{}) (Resource, error) {
	resourceType, found := tracker.resourceTypes.Lookup(typ)
	if !found {
		return nil, ErrUnknownResourceType
	}

	container, err := tracker.wardenClient.Create(warden.ContainerSpec{
		RootFSPath: resourceType.Image,
	})
	if err != nil {
		return nil, err
	}

	resource := NewResource(container, logs, abort)

	tracker.containersL.Lock()
	tracker.containers[resource] = container
	tracker.containersL.Unlock()

	return resource, nil
}

func (tracker *tracker) Release(resource Resource) error {
	tracker.containersL.Lock()
	container, found := tracker.containers[resource]
	delete(tracker.containers, resource)
	tracker.containersL.Unlock()

	if !found {
		return nil
	}

	return tracker.wardenClient.Destroy(container.Handle())
}
