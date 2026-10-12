module github.com/alexj11324/orvilo-gateway

go 1.26

require (
	github.com/alexj11324/orvilo-gateway/agent-gateway-go v0.0.0-00010101000000-000000000000
	github.com/alexj11324/orvilo-gateway/device-gateway-go v0.0.0-00010101000000-000000000000
)

replace (
	github.com/alexj11324/orvilo-gateway/agent-gateway-go => ./agent-gateway-go
	github.com/alexj11324/orvilo-gateway/device-gateway-go => ./device-gateway-go
)
