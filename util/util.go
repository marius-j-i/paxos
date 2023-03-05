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

/* Return format `http://<addr>/<endpoint>/args...`
 */
func HttpUrl(addr, endpoint string, args ...interface{}) string {
	url := fmt.Sprintf("http://%s/%s", addr, endpoint)
	for a := range args {
		url = fmt.Sprintf("%s/%s", url, fmt.Sprint(a))
	}
	return url
}
