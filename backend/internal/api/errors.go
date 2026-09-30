package api

import "errors"

// ErrNotImplemented kennzeichnet Operationen, die in der aktuellen Iteration
// noch nicht umgesetzt sind. Der Server antwortet mit 501 (Problem Details).
var ErrNotImplemented = errors.New("operation not implemented")
