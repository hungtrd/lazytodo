package task

import (
	"fmt"
	"time"
)

func newID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
