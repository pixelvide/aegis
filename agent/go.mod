module github.com/pixelvide/aegis/agent

go 1.25.0

require (
	github.com/google/uuid v1.6.0
	github.com/pixelvide/localharness v0.9.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/gorilla/websocket v1.5.3 // indirect
	google.golang.org/protobuf v1.36.5 // indirect
)

// Local development: use local checkout of localharness.
replace github.com/pixelvide/localharness => ../../local-harness
