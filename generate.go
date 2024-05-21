//go:generate go run pkg/codegen/cleanup/main.go
//go:generate go run pkg/codegen/main.go
//go:generate go run ./pkg/codegen crds ./charts/templates/crds.yaml
//go:generate go run pkg/util/gousb/codegen/main.go -template pkg/util/gousb/codegen/load_data.go.tpl -o pkg/util/gousb/usbid/load_data.go

package main
