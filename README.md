# agollo-listener

implement structured dynamic configuration through listener of agollo, that is a apollo client for go

## Usage

```bash
go get -u github.com/yguilai/agollo-listener
```

### Simple Case

```go
package main

import (
	"fmt"
	"github.com/apolloconfig/agollo/v4"
	apolloconfig "github.com/apolloconfig/agollo/v4/env/config"
	agollolistener "github.com/yguilai/agollo-listener"
)

type (
	AppConfig struct {
		Server struct {
			Port    int `apollo:"port"`
			Servlet struct {
				ContextPath string `apollo:"contextPath,default:/foo"`
			}
		}

		SimpleSlice  []int
		Slice        []Req
		SlicePointer []*Req
	}

	Req struct {
		Method  string
		Uri     string
		Timeout int `apollo:"time-out"`
	}
)

// Prefix implement agollolistener.Configuration
func (c *AppConfig) Prefix() string {
	return "app"
}

func main() {
	apolloClient, err := agollo.StartWithConfig(func() (*apolloconfig.AppConfig, error) {
		// fill your apollo configs
		return &apolloconfig.AppConfig{}, nil
	})
	if err != nil {
		fmt.Printf("agollo start error: %v\n", err)
		return
	}

	var appConfig AppConfig
	// bind a listener for appConfig with default options
	listener, err := agollolistener.NewConfigListener(&appConfig)
	// register listener to apollo client
	apolloClient.AddChangeListener(listener)

	var appConfigCustom AppConfig
	// also you can given some custom options
	listenerForCustom, err := agollolistener.NewConfigListener(
		&appConfigCustom,
		agollolistener.WithNamespaces("application", "application.yml"),
		agollolistener.WithReplaceEnv(),
	)
	apolloClient.AddChangeListener(listenerForCustom)

}
```