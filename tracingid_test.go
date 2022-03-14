package localtracing

import (
	"fmt"
	"testing"
)

func TestGenTracingID(t *testing.T) {
	fmt.Println(GenTracingID(4))
}
