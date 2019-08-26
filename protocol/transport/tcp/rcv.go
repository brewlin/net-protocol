// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tcp

import (
	"container/heap"

	"github.com/brewlin/net-protocol/pkg/seqnum"
)

// receiver holds the state necessary to receive TCP segments and turn them
// into a stream of bytes.
//
// +stateify savable
type receiver struct {
	ep *endpoint

	rcvNxt seqnum.Value

	// rcvAcc is one beyond the last acceptable sequence number. That is,
	// the "largest" sequence value that the receiver has announced to the
	// its peer that it's willing to accept. This may be different than
	// rcvNxt + rcvWnd if the receive window is reduced; in that case we
	// have to reduce the window as we receive more data instead of
	// shrinking it.
	// rcvAcc 超出了最后一个可接受的序列号。也就是说，接收方向其同行宣布它愿意接受的“最大”序列值。
	// 如果接收窗口减少，这可能与rcvNxt + rcvWnd不同;在这种情况下，我们必须减少窗口，因为我们收到更多数据而不是缩小它。
	rcvAcc seqnum.Value

	rcvWndScale uint8

	closed bool

	pendingRcvdSegments segmentHeap
	pendingBufUsed      seqnum.Size
	pendingBufSize      seqnum.Size
}

// 初始化接收器
func newReceiver(ep *endpoint, irs seqnum.Value, rcvWnd seqnum.Size, rcvWndScale uint8) *receiver {
	return &receiver{
		ep:             ep,
		rcvNxt:         irs + 1,
		rcvAcc:         irs.Add(rcvWnd + 1),
		rcvWndScale:    rcvWndScale,
		pendingBufSize: rcvWnd,
	}
}

// acceptable checks if the segment sequence number range is acceptable
// according to the table on page 26 of RFC 793.
// tcp流量控制：判断 segSeq 在窗口內
func (r *receiver) acceptable(segSeq seqnum.Value, segLen seqnum.Size) bool {
	rcvWnd := r.rcvNxt.Size(r.rcvAcc)
	if rcvWnd == 0 {
		return segLen == 0 && segSeq == r.rcvNxt
	}

	return segSeq.InWindow(r.rcvNxt, rcvWnd) ||
		seqnum.Overlap(r.rcvNxt, rcvWnd, segSeq, segLen)
}

// getSendParams returns the parameters needed by the sender when building
// segments to send.
// getSendParams 在构建要发送的段时，返回发送方所需的参数。
// 并且更新接收窗口的指标 rcvAcc
func (r *receiver) getSendParams() (rcvNxt seqnum.Value, rcvWnd seqnum.Size) {
	// Calculate the window size based on the current buffer size.
	n := r.ep.receiveBufferAvailable()
	acc := r.rcvNxt.Add(seqnum.Size(n))
	if r.rcvAcc.LessThan(acc) {
		r.rcvAcc = acc
	}

	return r.rcvNxt, r.rcvNxt.Size(r.rcvAcc) >> r.rcvWndScale
}

// nonZeroWindow is called when the receive window grows from zero to nonzero;
// in such cases we may need to send an ack to indicate to our peer that it can
// resume sending data.
// tcp流量控制：当接收窗口从零增长到非零时，调用 nonZeroWindow;在这种情况下，
// 我们可能需要发送一个 ack，以便向对端表明它可以恢复发送数据。
func (r *receiver) nonZeroWindow() {
	if (r.rcvAcc-r.rcvNxt)>>r.rcvWndScale != 0 {
		// We never got around to announcing a zero window size, so we
		// don't need to immediately announce a nonzero one.
		return
	}

	// Immediately send an ack.
	r.ep.snd.sendAck()
}

// consumeSegment attempts to consume a segment that was received by r. The
// segment may have just been received or may have been received earlier but
// wasn't ready to be consumed then.
//
// Returns true if the segment was consumed, false if it cannot be consumed
// yet because of a missing segment.
// tcp可靠性：consumeSegment 尝试使用r接收tcp段。该数据段可能刚刚收到或可能已经收到，但尚未准备好被消费。
// 如果数据段被消耗则返回true，如果由于缺少段而无法消耗，则返回false。
func (r *receiver) consumeSegment(s *segment, segSeq seqnum.Value, segLen seqnum.Size) bool {
	if segLen > 0 {
		// If the segment doesn't include the seqnum we're expecting to
		// consume now, we're missing a segment. We cannot proceed until
		// we receive that segment though.
		// 我们期望接收到的序列号范围应该是 seqStart <= rcvNxt < seqEnd，
		// 如果不在这个范围内说明我们少了数据段，返回false，表示不能立马消费
		if !r.rcvNxt.InWindow(segSeq, segLen) {
			return false
		}

		// Trim segment to eliminate already acknowledged data.
		// 尝试去除已经确认过的数据
		if segSeq.LessThan(r.rcvNxt) {
			diff := segSeq.Size(r.rcvNxt)
			segLen -= diff
			segSeq.UpdateForward(diff)
			s.sequenceNumber.UpdateForward(diff)
			s.data.TrimFront(int(diff))
		}

		// Move segment to ready-to-deliver list. Wakeup any waiters.
		// 将tcp段插入接收链表，并通知应用层用数据来了
		r.ep.readyToRead(s)

	} else if segSeq != r.rcvNxt {
		return false
	}

	// Update the segment that we're expecting to consume.
	// 因为前面已经收到正确按序到达的数据，那么我们应该更新一下我们期望下次收到的序列号了
	r.rcvNxt = segSeq.Add(segLen)

	// Trim SACK Blocks to remove any SACK information that covers
	// sequence numbers that have been consumed.
	// 修剪SACK块以删除任何涵盖已消耗序列号的SACK信息。
	TrimSACKBlockList(&r.ep.sack, r.rcvNxt)

	// 如果收到 fin 报文
	if s.flagIsSet(flagFin) {
		// 控制报文消耗一个字节的序列号，因此这边期望下次收到的序列号加1
		r.rcvNxt++

		// 收到 fin，立即回复 ack
		r.ep.snd.sendAck()

		// Tell any readers that no more data will come.
		// 标记接收器关闭
		// 触发上层应用可以读取
		r.closed = true
		r.ep.readyToRead(nil)

		// Flush out any pending segments, except the very first one if
		// it happens to be the one we're handling now because the
		// caller is using it.
		first := 0
		if len(r.pendingRcvdSegments) != 0 && r.pendingRcvdSegments[0] == s {
			first = 1
		}

		for i := first; i < len(r.pendingRcvdSegments); i++ {
			r.pendingRcvdSegments[i].decRef()
		}
		r.pendingRcvdSegments = r.pendingRcvdSegments[:first]
	}

	return true
}

// handleRcvdSegment handles TCP segments directed at the connection managed by
// r as they arrive. It is called by the protocol main loop.
// 从 handleSegments 接收到tcp段，然后进行处理消费，所谓的消费就是将负载内容插入到接收队列中
func (r *receiver) handleRcvdSegment(s *segment) {
	// We don't care about receive processing anymore if the receive side
	// is closed.
	if r.closed {
		return
	}

	segLen := seqnum.Size(s.data.Size())
	segSeq := s.sequenceNumber

	// If the sequence number range is outside the acceptable range, just
	// send an ACK. This is according to RFC 793, page 37.
	// tcp流量控制：判断该数据段的序列号是否在接收窗口内，如果不在，立即返回ack给对端。
	if !r.acceptable(segSeq, segLen) {
		r.ep.snd.sendAck()
		return
	}

	// Defer segment processing if it can't be consumed now.
	// tcp可靠性：r.consumeSegment 返回值是个bool类型，如果是true，表示已经消费该数据段，
	// 如果不是，那么进行下面的处理，插入到 pendingRcvdSegments，且进行堆排序。
	if !r.consumeSegment(s, segSeq, segLen) {
		// 如果有负载数据或者是 fin 报文，立即回复一个 ack 报文
		if segLen > 0 || s.flagIsSet(flagFin) {
			// We only store the segment if it's within our buffer
			// size limit.
			// tcp可靠性：对于乱序的tcp段，应该在等待处理段中缓存
			if r.pendingBufUsed < r.pendingBufSize {
				r.pendingBufUsed += s.logicalLen()
				s.incRef()
				// 插入堆中，且进行排序
				heap.Push(&r.pendingRcvdSegments, s)
			}

			// tcp的可靠性：更新 sack 块信息
			UpdateSACKBlocks(&r.ep.sack, segSeq, segSeq.Add(segLen), r.rcvNxt)

			// Immediately send an ack so that the peer knows it may
			// have to retransmit.
			r.ep.snd.sendAck()
		}
		return
	}

	// By consuming the current segment, we may have filled a gap in the
	// sequence number domain that allows pending segments to be consumed
	// now. So try to do it.
	// tcp的可靠性：通过使用当前段，我们可能填补了序列号域中的间隙，该间隙允许现在使用待处理段。
	// 所以试着去消费等待处理段。
	for !r.closed && r.pendingRcvdSegments.Len() > 0 {
		s := r.pendingRcvdSegments[0]
		segLen := seqnum.Size(s.data.Size())
		segSeq := s.sequenceNumber

		// Skip segment altogether if it has already been acknowledged.
		if !segSeq.Add(segLen-1).LessThan(r.rcvNxt) &&
			!r.consumeSegment(s, segSeq, segLen) {
			break
		}

		// 如果该tcp段，已经被正确消费，那么中等待处理段中删除
		heap.Pop(&r.pendingRcvdSegments)
		r.pendingBufUsed -= s.logicalLen()
		s.decRef()
	}
}
