package mining

import (
	"bytes"
	"container/heap"
	"sort"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
)

// MessageQueue is a priority queue of messages from different actors. Messages are ordered
// by decreasing gas price, subject to the constraint that messages from a single actor are
// always in increasing nonce order.
// All messages for a queue are inserted at construction, after which messages may only
// be popped.
// Potential improvements include:
// - deprioritising messages after a gap in nonce value, which can never be mined (see Ethereum)
// - attempting to pack messages into a fixed gas limit (i.e. 0/1 knapsack subject to nonce ordering)
type MessageQueue struct {
	bySender queueHeap
}

// NewMessageQueue allocates and initializes a message queue.
func NewMessageQueue(msgs []*types.SignedMessage) MessageQueue {
	// Group messages by sender.
	bySender := make(map[address.Address]nonceQueue)
	for _, m := range msgs {
		bySender[m.From] = append(bySender[m.From], m)
	}

	// Order each sender queue by nonce and initialize heap structure.
	addrHeap := make(queueHeap, len(bySender))
	heapIdx := 0
	for _, nq := range bySender {
		sort.Slice(nq, func(i, j int) bool { return nq[i].Nonce < nq[j].Nonce })
		addrHeap[heapIdx] = nq
		heapIdx++
	}
	heap.Init(&addrHeap)

	return MessageQueue{addrHeap}
}

// Empty tests whether the queue is empty.
func (mq *MessageQueue) Empty() bool {
	return len(mq.bySender) == 0
}

// Pop removes and returns the next message from the queue, returning (nil, false) if none remain.
func (mq *MessageQueue) Pop() (*types.SignedMessage, bool) {
	if len(mq.bySender) == 0 {
		return nil, false
	}
	// Select actor with best gas price.
	bestQueue := &mq.bySender[0]

	// Pop first message off that actor's queue
	msg := (*bestQueue)[0]
	if len(*bestQueue) == 1 {
		// If the queue will become empty, remove it from the heap.
		heap.Pop(&mq.bySender)
	} else {
		*bestQueue = (*bestQueue)[1:]
		heap.Fix(&mq.bySender, 0)
	}
	return msg, true
}

// Drain removes and returns all messages in a slice.
func (mq *MessageQueue) Drain() []*types.SignedMessage {
	var out []*types.SignedMessage
	for msg, hasMore := mq.Pop(); hasMore; msg, hasMore = mq.Pop() {
		out = append(out, msg)
	}
	return out
}

// A slice of messages ordered by Nonce (for a single sender).
type nonceQueue []*types.SignedMessage

// Implements heap.Interface to hold a priority queue of queues per sender.
// Heap priority is given by the gas price of the first (lowest nonce) message for each queue.
type queueHeap []nonceQueue

func (aq queueHeap) Len() int { return len(aq) }

// Len implements Heap.Interface.Less to compare items on gas price and sender address.
func (aq queueHeap) Less(i, j int) bool {
	delta := aq[i][0].MeteredMessage.GasPrice.Sub(&aq[j][0].MeteredMessage.GasPrice)
	if !delta.Equal(types.ZeroAttoFIL) {
		// We want Pop to give us the highest gas price, so use GreaterThan.
		return delta.GreaterThan(types.ZeroAttoFIL)
	}
	// Secondarily order by address to give a stable ordering.
	return bytes.Compare(aq[i][0].From[:], aq[j][0].From[:]) < 0
}

func (aq queueHeap) Swap(i, j int) {
	aq[i], aq[j] = aq[j], aq[i]
}

func (aq *queueHeap) Push(x interface{}) {
	item := x.(nonceQueue)
	*aq = append(*aq, item)
}

func (aq *queueHeap) Pop() interface{} {
	old := *aq
	n := len(old)
	item := old[n-1]
	*aq = old[0 : n-1]
	return item
}
