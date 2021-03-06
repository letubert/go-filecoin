package core

import (
	"context"
	"fmt"
	"sync"
	"testing"

	hamt "gx/ipfs/QmNf3wujpV2Y7Lnj2hy2UrmuX8bhMDStRHbnSLh7Ypf36h/go-hamt-ipld"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
)

var seed = types.GenerateKeyInfoSeed()
var ki = types.MustGenerateKeyInfo(10, seed)
var mockSigner = types.NewMockSigner(ki)
var newSignedMessage = types.NewSignedMessageForTestGetter(mockSigner)

func TestMessagePoolAddRemove(t *testing.T) {
	assert := assert.New(t)

	pool := NewMessagePool()
	msg1 := newSignedMessage()
	msg2 := newSignedMessage()

	c1, err := msg1.Cid()
	assert.NoError(err)
	c2, err := msg2.Cid()
	assert.NoError(err)

	assert.Len(pool.Pending(), 0)
	_, err = pool.Add(msg1)
	assert.NoError(err)
	assert.Len(pool.Pending(), 1)
	_, err = pool.Add(msg2)
	assert.NoError(err)
	assert.Len(pool.Pending(), 2)

	pool.Remove(c1)
	assert.Len(pool.Pending(), 1)
	pool.Remove(c2)
	assert.Len(pool.Pending(), 0)
}

func TestMessagePoolAddBadSignature(t *testing.T) {
	assert := assert.New(t)

	pool := NewMessagePool()
	smsg := newSignedMessage()
	smsg.Message.Nonce = types.Uint64(uint64(smsg.Message.Nonce) + uint64(1)) // invalidate message

	c, err := pool.Add(smsg)
	assert.False(c.Defined())
	assert.Error(err)
}

func TestMessagePoolDedup(t *testing.T) {
	assert := assert.New(t)

	pool := NewMessagePool()
	msg1 := newSignedMessage()

	assert.Len(pool.Pending(), 0)
	_, err := pool.Add(msg1)
	assert.NoError(err)
	assert.Len(pool.Pending(), 1)

	_, err = pool.Add(msg1)
	assert.NoError(err)
	assert.Len(pool.Pending(), 1)
}

func TestMessagePoolAsync(t *testing.T) {
	assert := assert.New(t)

	count := 400
	msgs := types.NewSignedMsgs(count, mockSigner)

	pool := NewMessagePool()
	var wg sync.WaitGroup

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			for j := 0; j < count/4; j++ {
				_, err := pool.Add(msgs[j+(count/4)*i])
				assert.NoError(err)
			}
			wg.Done()
		}(i)
	}

	wg.Wait()
	assert.Len(pool.Pending(), count)
}

func msgAsString(msg *types.SignedMessage) string {
	// When using NewMessageForTestGetter msg.Method is set
	// to "msgN" so we print that (it will correspond
	// to a variable of the same name in the tests
	// below).
	return msg.Message.Method
}

func msgsAsString(msgs []*types.SignedMessage) string {
	s := ""
	for _, m := range msgs {
		s = fmt.Sprintf("%s%s ", s, msgAsString(m))
	}
	return "[" + s + "]"
}

// assertPoolEquals returns true if p contains exactly the expected messages.
func assertPoolEquals(assert *assert.Assertions, p *MessagePool, expMsgs ...*types.SignedMessage) {
	msgs := p.Pending()
	if len(msgs) != len(expMsgs) {
		assert.Failf("wrong messages in pool", "expMsgs %v, got msgs %v", msgsAsString(expMsgs), msgsAsString(msgs))

	}
	for _, m1 := range expMsgs {
		found := false
		for _, m2 := range msgs {
			if types.SmsgCidsEqual(m1, m2) {
				found = true
				break
			}
		}
		if !found {
			assert.Failf("wrong messages in pool", "expMsgs %v, got msgs %v (msgs doesn't contain %v)", msgsAsString(expMsgs), msgsAsString(msgs), msgAsString(m1))
		}
	}
}

func headOf(chain []types.TipSet) types.TipSet {
	return chain[len(chain)-1]
}

func TestUpdateMessagePool(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	type msgs []*types.SignedMessage
	type msgsSet [][]*types.SignedMessage

	t.Run("Replace head", func(t *testing.T) {
		// Msg pool: [m0, m1], Chain: b[]
		// to
		// Msg pool: [m0],     Chain: b[m1]
		store := hamt.NewCborStore()
		p := NewMessagePool()

		m := types.NewSignedMsgs(2, mockSigner)
		MustAdd(p, m[0], m[1])

		oldChain := NewChainWithMessages(store, types.TipSet{}, msgsSet{})
		oldTipSet := headOf(oldChain)

		newChain := NewChainWithMessages(store, types.TipSet{}, msgsSet{msgs{m[1]}})
		newTipSet := headOf(newChain)

		assert.NoError(UpdateMessagePool(ctx, p, store, oldTipSet, newTipSet))
		assertPoolEquals(assert, p, m[0])
	})

	t.Run("Replace head with self", func(t *testing.T) {
		// Msg pool: [m0, m1], Chain: b[m2]
		// to
		// Msg pool: [m0, m1], Chain: b[m2]
		store := hamt.NewCborStore()
		p := NewMessagePool()

		m := types.NewSignedMsgs(3, mockSigner)
		MustAdd(p, m[0], m[1])

		oldChain := NewChainWithMessages(store, types.TipSet{}, msgsSet{msgs{m[2]}})
		oldTipSet := headOf(oldChain)

		UpdateMessagePool(ctx, p, store, oldTipSet, oldTipSet) // sic
		assertPoolEquals(assert, p, m[0], m[1])
	})

	t.Run("Replace head with a long chain", func(t *testing.T) {
		// Msg pool: [m2, m5],     Chain: b[m0, m1]
		// to
		// Msg pool: [m1],         Chain: b[m2, m3] -> b[m4] -> b[m0] -> b[] -> b[m5, m6]
		store := hamt.NewCborStore()
		p := NewMessagePool()

		m := types.NewSignedMsgs(7, mockSigner)
		MustAdd(p, m[2], m[5])

		oldChain := NewChainWithMessages(store, types.TipSet{}, msgsSet{msgs{m[0], m[1]}})
		oldTipSet := headOf(oldChain)

		newChain := NewChainWithMessages(store, types.TipSet{},
			msgsSet{msgs{m[2], m[3]}},
			msgsSet{msgs{m[4]}},
			msgsSet{msgs{m[0]}},
			msgsSet{msgs{}},
			msgsSet{msgs{m[5], m[6]}},
		)
		newTipSet := headOf(newChain)

		UpdateMessagePool(ctx, p, store, oldTipSet, newTipSet)
		assertPoolEquals(assert, p, m[1])
	})

	t.Run("Replace head with multi-block tipset chains", func(t *testing.T) {
		// Msg pool: [m2, m5],     Chain: {b[m0], b[m1]}
		// to
		// Msg pool: [m1],         Chain: b[m2, m3] -> {b[m4], b[m0], b[], b[]} -> {b[], b[m6,m5]}
		store := hamt.NewCborStore()
		p := NewMessagePool()

		m := types.NewSignedMsgs(7, mockSigner)
		MustAdd(p, m[2], m[5])

		oldChain := NewChainWithMessages(store, types.TipSet{}, msgsSet{msgs{m[0]}, msgs{m[1]}})
		oldTipSet := headOf(oldChain)

		newChain := NewChainWithMessages(store, types.TipSet{},
			msgsSet{msgs{m[2], m[3]}},
			msgsSet{msgs{m[4]}, msgs{m[0]}, msgs{}, msgs{}},
			msgsSet{msgs{}, msgs{m[5], m[6]}},
		)
		newTipSet := headOf(newChain)

		UpdateMessagePool(ctx, p, store, oldTipSet, newTipSet)
		assertPoolEquals(assert, p, m[1])
	})

	t.Run("Replace internal node (second one)", func(t *testing.T) {
		// Msg pool: [m3, m5],     Chain: b[m0] -> b[m1] -> b[m2]
		// to
		// Msg pool: [m1, m2],     Chain: b[m0] -> b[m3] -> b[m4, m5]
		store := hamt.NewCborStore()
		p := NewMessagePool()

		m := types.NewSignedMsgs(6, mockSigner)
		MustAdd(p, m[3], m[5])

		oldChain := NewChainWithMessages(store, types.TipSet{}, msgsSet{msgs{m[0]}}, msgsSet{msgs{m[1]}}, msgsSet{msgs{m[2]}})
		oldTipSet := headOf(oldChain)

		newChain := NewChainWithMessages(store, oldChain[0], msgsSet{msgs{m[3]}}, msgsSet{msgs{m[4], m[5]}})
		newTipSet := headOf(newChain)

		UpdateMessagePool(ctx, p, store, oldTipSet, newTipSet)
		assertPoolEquals(assert, p, m[1], m[2])
	})

	t.Run("Replace internal node (second one) with a long chain", func(t *testing.T) {
		// Msg pool: [m6],         Chain: b[m0] -> b[m1] -> b[m2]
		// to
		// Msg pool: [m6],         Chain: b[m0] -> b[m3] -> b[m4] -> b[m5] -> b[m1, m2]
		store := hamt.NewCborStore()
		p := NewMessagePool()

		m := types.NewSignedMsgs(7, mockSigner)
		MustAdd(p, m[6])

		oldChain := NewChainWithMessages(store, types.TipSet{},
			msgsSet{msgs{m[0]}},
			msgsSet{msgs{m[1]}},
			msgsSet{msgs{m[2]}},
		)
		oldTipSet := headOf(oldChain)

		newChain := NewChainWithMessages(store, oldChain[0],
			msgsSet{msgs{m[3]}},
			msgsSet{msgs{m[4]}},
			msgsSet{msgs{m[5]}},
			msgsSet{msgs{m[1], m[2]}},
		)
		newTipSet := headOf(newChain)

		UpdateMessagePool(ctx, p, store, oldTipSet, newTipSet)
		assertPoolEquals(assert, p, m[6])
	})

	t.Run("Replace internal node with multi-block tipset chains", func(t *testing.T) {
		// Msg pool: [m6],         Chain: {b[m0], b[m1]} -> b[m2]
		// to
		// Msg pool: [m6],         Chain: {b[m0], b[m1]} -> b[m3] -> b[m4] -> {b[m5], b[m1, m2]}
		store := hamt.NewCborStore()
		p := NewMessagePool()

		m := types.NewSignedMsgs(7, mockSigner)
		MustAdd(p, m[6])

		oldChain := NewChainWithMessages(store, types.TipSet{},
			msgsSet{msgs{m[0]}, msgs{m[1]}},
			msgsSet{msgs{m[2]}},
		)
		oldTipSet := headOf(oldChain)

		newChain := NewChainWithMessages(store, oldChain[0],
			msgsSet{msgs{m[3]}},
			msgsSet{msgs{m[4]}},
			msgsSet{msgs{m[5]}, msgs{m[1], m[2]}},
		)
		newTipSet := headOf(newChain)

		UpdateMessagePool(ctx, p, store, oldTipSet, newTipSet)
		assertPoolEquals(assert, p, m[6])
	})

	t.Run("Replace with same messages in different block structure", func(t *testing.T) {
		// Msg pool: [m3, m5],     Chain: b[m0] -> b[m1] -> b[m2]
		// to
		// Msg pool: [m3, m5],     Chain: {b[m0], b[m1], b[m2]}
		store := hamt.NewCborStore()
		p := NewMessagePool()

		m := types.NewSignedMsgs(6, mockSigner)
		MustAdd(p, m[3], m[5])

		oldChain := NewChainWithMessages(store, types.TipSet{},
			msgsSet{msgs{m[0]}},
			msgsSet{msgs{m[1]}},
			msgsSet{msgs{m[2]}},
		)
		oldTipSet := headOf(oldChain)

		newChain := NewChainWithMessages(store, types.TipSet{},
			msgsSet{msgs{m[0]}, msgs{m[1]}, msgs{m[2]}},
		)
		newTipSet := headOf(newChain)

		UpdateMessagePool(ctx, p, store, oldTipSet, newTipSet)
		assertPoolEquals(assert, p, m[3], m[5])
	})

	t.Run("Truncate to internal node", func(t *testing.T) {
		// Msg pool: [],               Chain: b[m0] -> b[m1] -> b[m2] -> b[m3]
		// to
		// Msg pool: [m2, m3],         Chain: b[m0] -> b[m1]
		store := hamt.NewCborStore()
		p := NewMessagePool()
		m := types.NewSignedMsgs(4, mockSigner)

		oldChain := NewChainWithMessages(store, types.TipSet{},
			msgsSet{msgs{m[0]}},
			msgsSet{msgs{m[1]}},
			msgsSet{msgs{m[2]}},
			msgsSet{msgs{m[3]}},
		)
		oldTipSet := headOf(oldChain)

		oldTipSetPrev := oldChain[1]
		UpdateMessagePool(ctx, p, store, oldTipSet, oldTipSetPrev)
		assertPoolEquals(assert, p, m[2], m[3])
	})

	t.Run("Extend head", func(t *testing.T) {
		// Msg pool: [m0, m1], Chain: b[]
		// to
		// Msg pool: [m0],     Chain: b[] -> b[m1, m2]
		store := hamt.NewCborStore()
		p := NewMessagePool()

		m := types.NewSignedMsgs(3, mockSigner)
		MustAdd(p, m[0], m[1])

		oldChain := NewChainWithMessages(store, types.TipSet{}, msgsSet{msgs{}})
		oldTipSet := headOf(oldChain)

		newChain := NewChainWithMessages(store, oldChain[len(oldChain)-1], msgsSet{msgs{m[1], m[2]}})
		newTipSet := headOf(newChain)

		UpdateMessagePool(ctx, p, store, oldTipSet, newTipSet)
		assertPoolEquals(assert, p, m[0])
	})

	t.Run("Extend head with a longer chain and more messages", func(t *testing.T) {
		// Msg pool: [m2, m5],     Chain: b[m0] -> b[m1]
		// to
		// Msg pool: [],           Chain: b[m0] -> b[m1] -> b[m2, m3] -> b[m4] -> b[m5, m6]
		store := hamt.NewCborStore()
		p := NewMessagePool()

		m := types.NewSignedMsgs(7, mockSigner)
		MustAdd(p, m[2], m[5])

		oldChain := NewChainWithMessages(store, types.TipSet{}, msgsSet{msgs{m[0]}}, msgsSet{msgs{m[1]}})
		oldTipSet := headOf(oldChain)

		newChain := NewChainWithMessages(store, oldChain[1],
			msgsSet{msgs{m[2], m[3]}},
			msgsSet{msgs{m[4]}},
			msgsSet{msgs{m[5], m[6]}},
		)
		newTipSet := headOf(newChain)

		UpdateMessagePool(ctx, p, store, oldTipSet, newTipSet)
		assertPoolEquals(assert, p)
	})
}

func TestLargestNonce(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	t.Run("No matches", func(t *testing.T) {
		p := NewMessagePool()

		m := types.NewSignedMsgs(2, mockSigner)
		MustAdd(p, m[0], m[1])

		_, found := LargestNonce(p, address.NewForTestGetter()())
		assert.False(found)
	})

	t.Run("Match, largest is zero", func(t *testing.T) {
		p := NewMessagePool()

		m := types.NewMsgsWithAddrs(1, mockSigner.Addresses)
		m[0].Nonce = 0

		sm, err := types.SignMsgs(mockSigner, m)
		require.NoError(err)

		MustAdd(p, sm...)

		largest, found := LargestNonce(p, m[0].From)
		assert.True(found)
		assert.Equal(uint64(0), largest)
	})

	t.Run("Match", func(t *testing.T) {
		p := NewMessagePool()

		m := types.NewMsgsWithAddrs(3, mockSigner.Addresses)
		m[1].Nonce = 1
		m[2].Nonce = 2
		m[2].From = m[1].From

		sm, err := types.SignMsgs(mockSigner, m)
		require.NoError(err)

		MustAdd(p, sm...)

		largest, found := LargestNonce(p, m[2].From)
		assert.True(found)
		assert.Equal(uint64(2), largest)
	})
}
