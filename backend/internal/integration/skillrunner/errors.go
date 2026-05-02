package skillrunner

import "errors"

var (
	ErrReviewRequired     = errors.New("script skill version must be approved before test/use/publish")
	ErrUnsupportedRuntime = errors.New("unsupported script runtime")
)
