package util

import (
	"errors"
	"fmt"
	"math/rand"
	"time"
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
	for _, a := range args {
		url = fmt.Sprintf("%s/%v", url, a)
	}
	return url
}

/* Select a random timeout from interval to wait for; then return.
 */
func RandTimeout(lower, upper int, unit time.Duration) {

	t := rand.Intn(upper-lower) + lower
	d := time.Duration(t) * unit

	wait := time.NewTimer(d)
	<-wait.C
}
