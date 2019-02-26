package crypto_test

import (
	"testing"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"

	"github.com/filecoin-project/go-filecoin/crypto"
)

func TestGenerateKey(t *testing.T) {
	assert := assert.New(t)

	key, err := crypto.GenerateKey()
	assert.NoError(err)

	keyBytes := key.Serialize()

	assert.Equal(len(keyBytes), 32)
	assert.NotEqual(keyBytes[0], 0)

	msg := make([]byte, 32)
	msg[0] = 1

	digest, err := key.Sign(msg)
	assert.NoError(err)
	assert.Equal(len(digest), 65)

	// assert.True(key.PublicKey().Verify(msg, digest))

	recovered, err := crypto.EcRecover(msg, digest)
	assert.NoError(err)
	assert.Equal(recovered, key.PublicKey())
}
