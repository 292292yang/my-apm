package infra

import "google.golang.org/grpc/metadata"

type metadataSupplier struct {
	metadata metadata.MD
}

func (m *metadataSupplier) Get(key string) string {
	values := m.metadata.Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (m *metadataSupplier) Set(key, value string) {
	m.metadata.Set(key, value)
}

func (m *metadataSupplier) Keys() []string {
	res := make([]string, 0)
	for key := range m.metadata {
		res = append(res, key)
	}
	return res
}
