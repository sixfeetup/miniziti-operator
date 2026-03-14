//go:build tools

package hack

import (
	_ "github.com/golangci/golangci-lint/v2/cmd/golangci-lint"
	_ "sigs.k8s.io/controller-runtime/tools/setup-envtest"
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
)
