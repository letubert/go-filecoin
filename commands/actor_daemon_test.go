package commands_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/filecoin-project/go-filecoin/api"
	th "github.com/filecoin-project/go-filecoin/testhelpers"

	ast "gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	req "gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
)

func TestActorDaemon(t *testing.T) {
	t.Parallel()
	t.Run("actor ls --enc json returns NDJSON containing all actors in the state tree", func(t *testing.T) {
		require := req.New(t)
		assert := ast.New(t)

		d := th.NewDaemon(t).Start()
		defer d.ShutdownSuccess()

		op1 := d.RunSuccess("actor", "ls", "--enc", "json")
		result1 := op1.ReadStdoutTrimNewlines()

		var avs []api.ActorView
		for _, line := range bytes.Split([]byte(result1), []byte{'\n'}) {
			requireSchemaConformance(t, line, "actor_ls")

			// unmarshall JSON to actor view an add to slice
			var av api.ActorView
			err := json.Unmarshal(line, &av)
			require.NoError(err)
			avs = append(avs, av)
		}

		assert.NotZero(len(avs))

		// The order of actors is consistent, but only within builds of genesis.car.
		// We just want to make sure the views have something valid in them.
		for _, av := range avs {
			assert.Contains([]string{"StoragemarketActor", "AccountActor", "PaymentbrokerActor", "MinerActor", "BootstrapMinerActor"}, av.ActorType)
			if av.ActorType == "AccountActor" {
				assert.Zero(len(av.Exports))
			} else {
				assert.NotZero(len(av.Exports))
			}
		}
	})
}
