package porcelain_test

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/porcelain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sectorbuilderTestPlumbing struct {
	sectorID abi.Value
}

func (stp *sectorbuilderTestPlumbing) MessageQuery(
	ctx context.Context,
	optFrom,
	to address.Address,
	method string,
	params ...interface{},
) ([][]byte, *exec.FunctionSignature, error) {
	signature := &exec.FunctionSignature{
		Params: nil,
		Return: []abi.Type{abi.SectorID},
	}
	ret, _ := stp.sectorID.Serialize()
	return [][]byte{ret}, signature, nil
}

func TestSectorBuilderGetLastUsedID(t *testing.T) {
	t.Run("Returns the correct value for wallet balance", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)
		ctx := context.Background()

		expectedSectorID := abi.Value{
			Val: uint64(5),
		}
		plumbing := &sectorbuilderTestPlumbing{
			sectorID: expectedSectorID,
		}
		sectorID, err := porcelain.SectorBuilderGetLastUsedID(ctx, plumbing, address.Address{})
		require.NoError(err)

		assert.Equal(expectedSectorID, sectorID)
	})
}
