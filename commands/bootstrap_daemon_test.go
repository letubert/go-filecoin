package commands_test

import (
	"testing"

	th "github.com/filecoin-project/go-filecoin/testhelpers"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
)

func TestBootstrapList(t *testing.T) {
	t.Parallel()

	d := th.NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	bs := d.RunSuccess("bootstrap ls")

	assert.Equal(t, "&{[]}\n", bs.ReadStdout())
}
