//go:generate go run pkg/codegen/cleanup/main.go
//go:generate go run pkg/codegen/main.go
//go:generate go run ./pkg/codegen crds ./charts/pcidevices/templates/crds.yaml

package main
