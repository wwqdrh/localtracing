package driver

import (
	"fmt"
	"testing"
)

func TestLeveldbAllKey(t *testing.T) {
	d, err := NewLevelDBDriver("./temp/heap")
	if err != nil {
		t.Error(err)
	}
	length, err := d.IterAllLen("apitime")
	if err != nil {
		t.Error(err)
	}
	fmt.Println(length)
}
