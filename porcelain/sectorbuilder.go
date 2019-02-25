package porcelain

import (
	"context"
	"errors"
	"github.com/filecoin-project/go-filecoin/abi"
)

type sbgluidPlumbing interface {
	MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, *exec.FunctionSignature, error)
}

// SectorBuilderGetLastUsedID determines the current block height
func SectorBuilderGetLastUsedID(ctx context.Context, plumbing sbgluidPlumbing, minerAddr address.Address) (uint64, error) {
  rets, methodSignature, err := plumbing.MessageQuery(
    ctx,
    address.Address{},
    minerAddr,
    "getLastUsedSectorID",
  )
  if err != nil {
    return 0, errors.Wrap(err, "failed to call query method getLastUsedSectorID")
  }

  lastUsedSectorIDVal, err := abi.Deserialize(rets[0], methodSignature.Return[0])
  if err != nil {
    return 0, errors.Wrap(err, "failed to convert returned ABI value")
  }
  lastUsedSectorID, ok := lastUsedSectorIDVal.Val.(uint64)
  if !ok {
    return 0, errors.New("failed to convert returned ABI value to uint64")
  }

  return lastUsedSectorID, nil
}
