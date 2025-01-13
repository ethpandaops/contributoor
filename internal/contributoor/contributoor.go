package contributoor

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/ethpandaops/xatu/pkg/proto/xatu"
)

var (
	Release        = "dev"
	GitCommit      = "dev"
	Implementation = "Contributoor"
	GOOS           = runtime.GOOS
	GOARCH         = runtime.GOARCH
	Module         = xatu.ModuleName_SENTRY
)

func Full() string {
	return fmt.Sprintf("%s/%s", Implementation, Short())
}

func Short() string {
	return fmt.Sprintf("%s-%s", Release, GitCommit)
}

func FullVWithGOOS() string {
	return fmt.Sprintf("%s/%s", Full(), GOOS)
}

func FullVWithPlatform() string {
	return fmt.Sprintf("%s/%s/%s", Full(), GOOS, GOARCH)
}

func ImplementationLower() string {
	return strings.ToLower(Implementation)
}
