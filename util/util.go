package util

import (
	"errors"
	"fmt"
)

var (
	ErrImplMe = errors.New("implement me")
)

/* Return new error with var args formatted into error-string.
 * Note: assumes existing error-string already has format syntax.
 */
func ErrorFormat(err error, formats ...interface{}) error {
	return fmt.Errorf(err.Error(), formats...)
}