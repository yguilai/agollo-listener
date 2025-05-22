package apollox

import (
	"github.com/apolloconfig/agollo/v4/storage"
	"github.com/stretchr/testify/assert"
	"testing"
)

type TestConfig struct {
	Server struct {
		Port    int `apollo:"port"`
		Servlet struct {
			ContextPath string `apollo:"contextPath"`
		}
	}

	SimpleSlice  []int
	Slice        []Req
	SlicePointer []*Req
}

type Req struct {
	Method  string
	Uri     string
	Timeout int `apollo:"time-out"`
}

func (t *TestConfig) Prefix() string {
	return "app"
}

var testConfigV2 TestConfig

func TestNewConfigListener2(t *testing.T) {
	var c TestConfig
	listener, err := NewConfigListener(&c, WithExtraNamespace("app.yml"))
	if err != nil {
		t.Fatalf("apollo config listener error: %v", err)
	}
	r := listener.contains("app.server.servlet.contextPath")
	assert.True(t, r)
	r = listener.contains("app.server.servlet.contextpath")
	assert.True(t, r)
}

func TestConfigListenerV2_OnChange_Simple(t *testing.T) {
	listener := getConfigListenerV2(t)
	event := getDefaultEvent()
	event.Changes = map[string]*storage.ConfigChange{
		"app.server.port": {
			OldValue:   nil,
			NewValue:   8888,
			ChangeType: storage.ADDED,
		},
	}
	listener.OnChange(event)
	assert.Equal(t, 8888, testConfigV2.Server.Port)
}

func TestConfigListenerV2_OnChange_SimpleSlice(t *testing.T) {
	listener := getConfigListenerV2(t)
	event := getDefaultEvent()
	event.Changes = map[string]*storage.ConfigChange{
		"app.simpleSlice": {
			OldValue:   nil,
			NewValue:   []interface{}{1, 2, 3},
			ChangeType: storage.ADDED,
		},
	}
	listener.OnChange(event)
	assert.Equal(t, []int{1, 2, 3}, testConfigV2.SimpleSlice)
}

func TestConfigListenerV2_OnChange_Slice(t *testing.T) {
	listener := getConfigListenerV2(t)
	event := getDefaultEvent()
	event.Changes = map[string]*storage.ConfigChange{
		"app.slice": {
			OldValue:   nil,
			NewValue:   []interface{}{map[string]interface{}{"method": "GET", "uri": "/foo"}},
			ChangeType: storage.ADDED,
		},
	}
	listener.OnChange(event)
	assert.Equal(t, []Req{{Method: "GET", Uri: "/foo"}}, testConfigV2.Slice)
}

func TestConfigListenerV2_OnChange_SlicePointer(t *testing.T) {
	listener := getConfigListenerV2(t)
	event := getDefaultEvent()
	event.Changes = map[string]*storage.ConfigChange{
		"app.slicePointer": {
			OldValue:   nil,
			NewValue:   []interface{}{map[string]interface{}{"method": "GET", "uri": "/foo", "time-out": 10}},
			ChangeType: storage.ADDED,
		},
	}
	listener.OnChange(event)
	assert.Equal(t, []*Req{{Method: "GET", Uri: "/foo", Timeout: 10}}, testConfigV2.SlicePointer)
}

func getConfigListenerV2(t *testing.T) *ConfigListener {
	listener, err := NewConfigListener(&testConfigV2, WithExtraNamespace("app.yml"))
	if err != nil {
		t.Fatalf("apollo config listener error: %v", err)
	}
	return listener
}

func getDefaultEvent() *storage.ChangeEvent {
	var event storage.ChangeEvent
	event.Namespace = defaultNamespace
	return &event
}
