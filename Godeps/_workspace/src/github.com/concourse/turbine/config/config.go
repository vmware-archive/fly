package config

type Config struct {
	ResourceTypes ResourceTypes
}

type ResourceTypes []ResourceType

type ResourceType struct {
	Name  string
	Image string
}

func (types ResourceTypes) Lookup(name string) (ResourceType, bool) {
	for _, rt := range types {
		if rt.Name == name {
			return rt, true
		}
	}

	return ResourceType{}, false
}
