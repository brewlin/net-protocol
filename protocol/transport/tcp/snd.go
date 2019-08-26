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
	"math"
	"sync"
	"time"

	"github.com/brewlin/net-protocol/pkg/sleep"
	tcpip "github.com/brewlin/net-protocol/protocol"
	"github.com/brewlin/net-protocol/pkg/buffer"
	"github.com/brewlin/net-protocol/protocol/header"
	"github.com/brewlin/net-protocol/pkg/seqnum"
)

const (
	// minRTO is the minimum allowed value for the retransmit timeout.
	minRTO = 200 * time.Millisecond

	// InitialCwnd is the initial congestion window.
	// 初始拥塞窗口大小
	InitialCwnd = 10

	// nDupAckThreshold is the number of duplicate ACK's required
	// before fast-retransmit is entered.
	nDupAckThreshold = 3
)

// congestionControl is an interface that must be implemented by any supported
// congestion control algorithm.
// tcp拥塞控制：拥塞控制算法的接口
type congestionControl interface {
	// HandleNDupAcks is invoked when sender.dupAckCount >= nDupAckThreshold
	// just before entering fast retransmit.
	// 在进入快速重新传输之前，当 sender.dupAckCount> = nDupAckThreshold 时调用HandleNDupAcks。
	HandleNDupAcks()

	// HandleRTOExpired is invoked when the retransmit timer expires.
	// 当重新传输计时器到期时调用HandleRTOExpired。
	HandleRTOExpired()

	// Update is invoked when processing inbound acks. It's passed the
	// number of packet's that were acked by the most recent cumulative
	// acknowledgement.
	// 已经有数据包被确认时调用 Update。它传递了最近累积确认所确认的数据包数。
	Update(packetsAcked int)

	// PostRecovery is invoked when the sender is exiting a fast retransmit/
	// recovery phase. This provides congestion control algorithms a way
	// to adjust their state when exiting recovery.
	// 当发送方退出快速重新传输/恢复阶段时，将调用PostRecovery。
	// 这为拥塞控制算法提供了一种在退出恢复时调整其状态的方法。
	PostRecovery()
}

/*
					 +-------> sndwnd <-------+
					 |						  |
---------------------+-------------+----------+--------------------
|      acked		 | * * * * * * | # # # # #|   unable send
---------------------+-------------+----------+--------------------
					 ^             ^
					 |			   |
				   sndUna        sndNxt
***** in flight data
##### able send date
*/

// sender holds the state necessary to send TCP segments.
//
// +stateify savable
// tcp发送器，它维护了tcp必要的状态
type sender struct {
	ep *endpoint

	// lastSendTime is the timestamp when the last packet was sent.
	// lastSendTime 是发送最后一个数据包的时间戳。
	lastSendTime time.Time

	// dupAckCount is the number of duplicated acks received. It is used for
	// fast retransmit.
	// dupAckCount 是收到的重复ack数。它用于快速重传。
	dupAckCount int

	// fr holds state related to fast recovery.
	// fr 持有与快速恢复有关的状态。
	fr fastRecovery

	// sndCwnd is the congestion window, in packets.
	// sndCwnd 是拥塞窗口，单位是包
	sndCwnd int

	// sndSsthresh is the threshold between slow start and congestion
	// avoidance.
	// sndSsthresh 是慢启动和拥塞避免之间的阈值。
	sndSsthresh int

	// sndCAAckCount is the number of packets acknowledged during congestion
	// avoidance. When enough packets have been ack'd (typically cwnd
	// packets), the congestion window is incremented by one.
	// sndCAAckCount 是拥塞避免期间确认的数据包数。当已经确认了足够的分组（通常是cwnd分组）时，拥塞窗口增加1。
	sndCAAckCount int

	// outstanding is the number of outstanding packets, that is, packets
	// that have been sent but not yet acknowledged.
	// outstanding 是正在发送的数据包的数量，即已发送但尚未确认的数据包。
	outstanding int

	// sndWnd is the send window size.
	// 发送窗口大小，单位是字节
	sndWnd seqnum.Size

	// sndUna is the next unacknowledged sequence number.
	// sndUna 是下一个未确认的序列号
	sndUna seqnum.Value

	// sndNxt is the sequence number of the next segment to be sent.
	// sndNxt 是要发送的下一个段的序列号。
	sndNxt seqnum.Value

	// sndNxtList is the sequence number of the next segment to be added to
	// the send list.
	// sndNxtList 是要添加到发送列表的下一个段的序列号。
	sndNxtList seqnum.Value

	// rttMeasureSeqNum is the sequence number being used for the latest RTT
	// measurement.
	rttMeasureSeqNum seqnum.Value

	// rttMeasureTime is the time when the rttMeasureSeqNum was sent.
	rttMeasureTime time.Time

	closed    bool
	writeNext *segment
	// 发送链表
	writeList   segmentList
	resendTimer timer
	resendWaker sleep.Waker

	// rtt.srtt, rtt.rttvar, and rto are the "smoothed round-trip time",
	// "round-trip time variation" and "retransmit timeout", as defined in
	// section 2 of RFC 6298.
	rtt        rtt
	rto        time.Duration
	srttInited bool

	// maxPayloadSize is the maximum size of the payload of a given segment.
	// It is initialized on demand.
	maxPayloadSize int

	// sndWndScale is the number of bits to shift left when reading the send
	// window size from a segment.
	sndWndScale uint8

	// maxSentAck is the maxium acknowledgement actually sent.
	maxSentAck seqnum.Value

	// cc is the congestion control algorithm in use for this sender.
	// cc 是实现拥塞控制算法的接口
	cc congestionControl
}

// rtt is a synchronization wrapper used to appease stateify. See the comment
// in sender, where it is used.
//
// +stateify savable
type rtt struct {
	sync.Mutex

	srtt   time.Duration
	rttvar time.Duration
}

// fastRecovery holds information related to fast recovery from a packet loss.
//
// +stateify savable
// fastRecovery 保存与数据包丢失快速恢复相关的信息
type fastRecovery struct {
	// active whether the endpoint is in fast recovery. The following fields
	// are only meaningful when active is true.
	active bool

	// first and last represent the inclusive sequence number range being
	// recovered.
	first seqnum.Value
	last  seqnum.Value

	// maxCwnd is the maximum value the congestion window may be inflated to
	// due to duplicate acks. This exists to avoid attacks where the
	// receiver intentionally sends duplicate acks to artificially inflate
	// the sender's cwnd.
	maxCwnd int
}

// 新建并初始化发送器
func newSender(ep *endpoint, iss, irs seqnum.Value, sndWnd seqnum.Size, mss uint16, sndWndScale int) *sender {
	s := &sender{
		ep:               ep,
		sndCwnd:          InitialCwnd,
		sndSsthresh:      math.MaxInt64,
		sndWnd:           sndWnd,
		sndUna:           iss + 1,
		sndNxt:           iss + 1,
		sndNxtList:       iss + 1,
		rto:              1 * time.Second,
		rttMeasureSeqNum: iss + 1,
		lastSendTime:     time.Now(),
		maxPayloadSize:   int(mss),
		maxSentAck:       irs + 1,
		fr: fastRecovery{
			// See: https://tools.ietf.org/html/rfc6582#section-3.2 Step 1.
			last: iss,
		},
	}

	// 拥塞控制算法的初始化
	s.cc = s.initCongestionControl(ep.cc)

	// A negative sndWndScale means that no scaling is in use, otherwise we
	// store the scaling value.
	if sndWndScale > 0 {
		s.sndWndScale = uint8(sndWndScale)
	}

	s.updateMaxPayloadSize(int(ep.route.MTU()), 0)

	s.resendTimer.init(&s.resendWaker)

	return s
}

// tcp拥塞控制：根据算法名，新建拥塞控制算法和初始化
func (s *sender) initCongestionControl(congestionControlName CongestionControlOption) congestionControl {
	switch congestionControlName {
	case ccCubic:
		return newCubicCC(s)
	case ccReno:
		fallthrough
	default:
		return newRenoCC(s)
	}
}

// updateMaxPayloadSize updates the maximum payload size based on the given
// MTU. If this is in response to "packet too big" control packets (indicated
// by the count argument), it also reduces the number of outstanding packets and
// attempts to retransmit the first packet above the MTU size.
func (s *sender) updateMaxPayloadSize(mtu, count int) {
	m := mtu - header.TCPMinimumSize

	// Calculate the maximum option size.
	// 计算MSS的大小
	var maxSackBlocks [header.TCPMaxSACKBlocks]header.SACKBlock
	options := s.ep.makeOptions(maxSackBlocks[:])
	m -= len(options)
	putOptions(options)

	// We don't adjust up for now.
	if m >= s.maxPayloadSize {
		return
	}

	// Make sure we can transmit at least one byte.
	if m <= 0 {
		m = 1
	}

	s.maxPayloadSize = m

	s.outstanding -= count
	if s.outstanding < 0 {
		s.outstanding = 0
	}

	// Rewind writeNext to the first segment exceeding the MTU. Do nothing
	// if it is already before such a packet.
	for seg := s.writeList.Front(); seg != nil; seg = seg.Next() {
		if seg == s.writeNext {
			// We got to writeNext before we could find a segment
			// exceeding the MTU.
			break
		}

		if seg.data.Size() > m {
			// We found a segment exceeding the MTU. Rewind
			// writeNext and try to retransmit it.
			s.writeNext = seg
			break
		}
	}

	// Since we likely reduced the number of outstanding packets, we may be
	// ready to send some more.
	s.sendData()
}

// sendAck sends an ACK segment.
// 发送一个ack tcp段
func (s *sender) sendAck() {
	s.sendSegment(buffer.VectorisedView{}, flagAck, s.sndNxt)
}

// updateRTO updates the retransmit timeout when a new roud-trip time is
// available. This is done in accordance with section 2 of RFC 6298.
// 根据rtt来更新计算rto
func (s *sender) updateRTO(rtt time.Duration) {
	s.rtt.Lock()
	// 第一次计算
	if !s.srttInited {
		s.rtt.rttvar = rtt / 2
		s.rtt.srtt = rtt
		s.srttInited = true
	} else {
		diff := s.rtt.srtt - rtt
		if diff < 0 {
			diff = -diff
		}
		// Use RFC6298 standard algorithm to update rttvar and srtt when
		// no timestamps are available.
		if !s.ep.sendTSOk {
			s.rtt.rttvar = (3*s.rtt.rttvar + diff) / 4
			s.rtt.srtt = (7*s.rtt.srtt + rtt) / 8
		} else {
			// When we are taking RTT measurements of every ACK then
			// we need to use a modified method as specified in
			// https://tools.ietf.org/html/rfc7323#appendix-G
			if s.outstanding == 0 {
				s.rtt.Unlock()
				return
			}
			// github.com/brewlin/net-protocol measures congestion window/inflight all in
			// terms of packets and not bytes. This is similar to
			// how linux also does cwnd and inflight. In practice
			// this approximation works as expected.
			expectedSamples := math.Ceil(float64(s.outstanding) / 2)

			// alpha & beta values are the original values as recommended in
			// https://tools.ietf.org/html/rfc6298#section-2.3.
			const alpha = 0.125
			const beta = 0.25

			alphaPrime := alpha / expectedSamples
			betaPrime := beta / expectedSamples
			rttVar := (1-betaPrime)*s.rtt.rttvar.Seconds() + betaPrime*diff.Seconds()
			srtt := (1-alphaPrime)*s.rtt.srtt.Seconds() + alphaPrime*rtt.Seconds()
			s.rtt.rttvar = time.Duration(rttVar * float64(time.Second))
			s.rtt.srtt = time.Duration(srtt * float64(time.Second))
		}
	}

	// 更新 rto 值
	s.rto = s.rtt.srtt + 4*s.rtt.rttvar
	s.rtt.Unlock()
	if s.rto < minRTO {
		s.rto = minRTO
	}
}

// resendSegment resends the first unacknowledged segment.
// tcp的拥塞控制：快速重传
func (s *sender) resendSegment() {
	// Don't use any segments we already sent to measure RTT as they may
	// have been affected by packets being lost.
	s.rttMeasureSeqNum = s.sndNxt

	// Resend the segment.
	if seg := s.writeList.Front(); seg != nil {
		s.sendSegment(seg.data, seg.flags, seg.sequenceNumber)
	}
}

// retransmitTimerExpired is called when the retransmit timer expires, and
// unacknowledged segments are assumed lost, and thus need to be resent.
// Returns true if the connection is still usable, or false if the connection
// is deemed lost.
// tcp的可靠性：重传定时器触发的时候调用这个函数，也就是超时重传
// tcp的拥塞控制：发生重传即认为发送丢包，拥塞控制需要对丢包进行相应的处理。
func (s *sender) retransmitTimerExpired() bool {
	// Check if the timer actually expired or if it's a spurious wake due
	// to a previously orphaned runtime timer.
	// 检查计时器是否真的到期
	if !s.resendTimer.checkExpiration() {
		return true
	}

	// Give up if we've waited more than a minute since the last resend.
	// 如果rto已经超过了1分钟，直接放弃发送，返回错误
	if s.rto >= 60*time.Second {
		return false
	}

	// Set new timeout. The timer will be restarted by the call to sendData
	// below.
	// 每次超时，rto都变成原来的2倍
	s.rto *= 2

	// tcp的拥塞控制：如果已经是快速恢复阶段，那么退出，因为现在丢包了
	if s.fr.active {
		// We were attempting fast recovery but were not successful.
		// Leave the state. We don't need to update ssthresh because it
		// has already been updated when entered fast-recovery.
		// 我们试图快速恢复，但没有成功，退出这个状态。
		// 我们不需要更新ssthresh，因为它在进入快速恢复时已经更新。
		s.leaveFastRecovery()
	}

	// See: https://tools.ietf.org/html/rfc6582#section-3.2 Step 4.
	// We store the highest sequence number transmitted in cases where
	// we were not in fast recovery.
	s.fr.last = s.sndNxt - 1

	// tcp的拥塞控制：处理丢包的情况
	s.cc.HandleRTOExpired()

	// Mark the next segment to be sent as the first unacknowledged one and
	// start sending again. Set the number of outstanding packets to 0 so
	// that we'll be able to retransmit.
	//
	// We'll keep on transmitting (or retransmitting) as we get acks for
	// the data we transmit.
	// tcp可靠性：将下一个段标记为第一个未确认的段，然后再次开始发送。将未完成的数据包数设置为0，以便我们能够重新传输。
	// 当我们收到我们传输的数据时，我们将继续传输（或重新传输）。
	s.outstanding = 0
	s.writeNext = s.writeList.Front()
	// 重新发送数据包
	s.sendData()

	return true
}

// sendData sends new data segments. It is called when data becomes available or
// when the send window opens up.
// 发送数据段，最终掉用 sendSegment 来发送
func (s *sender) sendData() {
	limit := s.maxPayloadSize

	// Reduce the congestion window to min(IW, cwnd) per RFC 5681, page 10.
	// "A TCP SHOULD set cwnd to no more than RW before beginning
	// transmission if the TCP has not sent data in the interval exceeding
	// the retrasmission timeout."
	// 根据RFC 5681，第10页将拥塞窗口减少到min（IW，cwnd）。
	// 如果TCP在超过重新传输超时的时间间隔内没有发送数据，TCP应该在开始传输之前将cwnd设置为不超过RW。
	if !s.fr.active && time.Now().Sub(s.lastSendTime) > s.rto {
		if s.sndCwnd > InitialCwnd {
			s.sndCwnd = InitialCwnd
		}
	}

	// TODO: We currently don't merge multiple send buffers
	// into one segment if they happen to fit. We should do that
	// eventually.
	var seg *segment
	end := s.sndUna.Add(s.sndWnd)
	var dataSent bool
	// 遍历发送链表，发送数据
	// tcp拥塞控制：s.outstanding < s.sndCwnd 判断正在发送的数据量不能超过拥塞窗口。
	for seg = s.writeNext; seg != nil && s.outstanding < s.sndCwnd; seg = seg.Next() {
		// We abuse the flags field to determine if we have already
		// assigned a sequence number to this segment.
		// 如果seg的flags是0，将flags改为psh|ack
		if seg.flags == 0 {
			seg.sequenceNumber = s.sndNxt
			seg.flags = flagAck | flagPsh
		}

		var segEnd seqnum.Value
		if seg.data.Size() == 0 { // 数据段没有负载，表示要结束连接
			if s.writeList.Back() != seg {
				panic("FIN segments must be the final segment in the write list.")
			}
			// 发送 fin 报文
			seg.flags = flagAck | flagFin
			// fin 报文需要确认，且消耗一个字节序列号
			segEnd = seg.sequenceNumber.Add(1)
		} else {
			// We're sending a non-FIN segment.
			if seg.flags&flagFin != 0 {
				panic("github.com/brewlin/net-protocol queues FIN segments without data.")
			}

			if !seg.sequenceNumber.LessThan(end) {
				break
			}

			// tcp流量控制：计算最多一次发送多大数据，
			available := int(seg.sequenceNumber.Size(end))
			if available > limit {
				available = limit
			}

			// 如果seg的payload字节数大于available
			// 将seg进行分段，并且插入到该seg的后面
			if seg.data.Size() > available {
				// Split this segment up.
				nSeg := seg.clone()
				nSeg.data.TrimFront(available)
				nSeg.sequenceNumber.UpdateForward(seqnum.Size(available))
				s.writeList.InsertAfter(seg, nSeg)
				seg.data.CapLength(available)
			}

			s.outstanding++
			segEnd = seg.sequenceNumber.Add(seqnum.Size(seg.data.Size()))
		}

		if !dataSent {
			dataSent = true
			// We are sending data, so we should stop the keepalive timer to
			// ensure that no keepalives are sent while there is pending data.
			s.ep.disableKeepaliveTimer()
		}
		s.sendSegment(seg.data, seg.flags, seg.sequenceNumber)

		// Update sndNxt if we actually sent new data (as opposed to
		// retransmitting some previously sent data).
		// 发送一个数据段后，更新sndNxt
		if s.sndNxt.LessThan(segEnd) {
			s.sndNxt = segEnd
		}
	}

	// Remember the next segment we'll write.
	s.writeNext = seg

	// Enable the timer if we have pending data and it's not enabled yet.
	// tcp的可靠性：如果重发定时器没有启动 且 snduna 不等于 sndNxt，启动定时器
	if !s.resendTimer.enabled() && s.sndUna != s.sndNxt {
		// 启动定时器，并且设定定时器的间隔为s.rto
		s.resendTimer.enable(s.rto)
	}
	// If we have no more pending data, start the keepalive timer.
	if s.sndUna == s.sndNxt {
		s.ep.resetKeepaliveTimer(false)
	}
}

// tcp拥塞控制：进入快速恢复状态和相应的处理
func (s *sender) enterFastRecovery() {
	s.fr.active = true
	// Save state to reflect we're now in fast recovery.
	// See : https://tools.ietf.org/html/rfc5681#section-3.2 Step 3.
	// We inflat the cwnd by 3 to account for the 3 packets which triggered
	// the 3 duplicate ACKs and are now not in flight.
	s.sndCwnd = s.sndSsthresh + 3
	s.fr.first = s.sndUna
	s.fr.last = s.sndNxt - 1
	s.fr.maxCwnd = s.sndCwnd + s.outstanding
}

// tcp拥塞控制：退出快速恢复状态和相应的处理
func (s *sender) leaveFastRecovery() {
	s.fr.active = false
	s.fr.first = 0
	s.fr.last = s.sndNxt - 1
	s.fr.maxCwnd = 0
	s.dupAckCount = 0

	// Deflate cwnd. It had been artificially inflated when new dups arrived.
	s.sndCwnd = s.sndSsthresh
	s.cc.PostRecovery()
}

// checkDuplicateAck is called when an ack is received. It manages the state
// related to duplicate acks and determines if a retransmit is needed according
// to the rules in RFC 6582 (NewReno).
// tcp拥塞控制：收到确认时调用 checkDuplicateAck。它管理与重复确认相关的状态，
// 并根据RFC 6582（NewReno）中的规则确定是否需要重新传输
func (s *sender) checkDuplicateAck(seg *segment) (rtx bool) {
	ack := seg.ackNumber
	// 已经启动了快速恢复
	if s.fr.active {
		// We are in fast recovery mode. Ignore the ack if it's out of
		// range.
		if !ack.InRange(s.sndUna, s.sndNxt+1) {
			return false
		}

		// Leave fast recovery if it acknowledges all the data covered by
		// this fast recovery session.
		// 如果它确认此快速恢复涵盖的所有数据，退出快速恢复。
		if s.fr.last.LessThan(ack) {
			s.leaveFastRecovery()
			return false
		}

		// Don't count this as a duplicate if it is carrying data or
		// updating the window.
		// 如果该tcp段有负载或者正在更新窗口，那么不计算这个ack
		if seg.logicalLen() != 0 || s.sndWnd != seg.window {
			return false
		}

		// Inflate the congestion window if we're getting duplicate acks
		// for the packet we retransmitted.
		if ack == s.fr.first {
			// We received a dup, inflate the congestion window by 1
			// packet if we're not at the max yet.
			if s.sndCwnd < s.fr.maxCwnd {
				s.sndCwnd++
			}
			return false
		}

		// A partial ack was received. Retransmit this packet and
		// remember it so that we don't retransmit it again. We don't
		// inflate the window because we're putting the same packet back
		// onto the wire.
		//
		// N.B. The retransmit timer will be reset by the caller.
		s.fr.first = ack
		s.dupAckCount = 0
		return true
	}

	// We're not in fast recovery yet. A segment is considered a duplicate
	// only if it doesn't carry any data and doesn't update the send window,
	// because if it does, it wasn't sent in response to an out-of-order
	// segment.
	// 我们还没有进入快速恢复状态，只有当段不携带任何数据并且不更新发送窗口时，才认为该段是重复的。
	if ack != s.sndUna || seg.logicalLen() != 0 || s.sndWnd != seg.window || ack == s.sndNxt {
		s.dupAckCount = 0
		return false
	}

	// 到这表示收到一个重复的ack
	s.dupAckCount++

	// 收到三次的重复ack才会进入快速恢复。
	if s.dupAckCount < nDupAckThreshold {
		return false
	}

	// See: https://tools.ietf.org/html/rfc6582#section-3.2 Step 2
	//
	// We only do the check here, the incrementing of last to the highest
	// sequence number transmitted till now is done when enterFastRecovery
	// is invoked.
	// 我们只在这里进行检查，当调用enterFastRecovery时，最后一次到最高序列号的递增完成。
	if !s.fr.last.LessThan(seg.ackNumber) {
		s.dupAckCount = 0
		return false
	}

	// 调用拥塞控制的 HandleNDupAcks 处理三次重复ack
	s.cc.HandleNDupAcks()
	// 进入快速恢复状态
	s.enterFastRecovery()
	s.dupAckCount = 0
	return true
}

// handleRcvdSegment is called when a segment is received; it is responsible for
// updating the send-related state.
//收到段时调用 handleRcvdSegment; 它负责更新与发送相关的状态。
func (s *sender) handleRcvdSegment(seg *segment) {
	// Check if we can extract an RTT measurement from this ack.
	// 如果rtt测量seq小于ack num，更新rto
	if !s.ep.sendTSOk && s.rttMeasureSeqNum.LessThan(seg.ackNumber) {
		s.updateRTO(time.Now().Sub(s.rttMeasureTime))
		s.rttMeasureSeqNum = s.sndNxt
	}

	// Update Timestamp if required. See RFC7323, section-4.3.
	s.ep.updateRecentTimestamp(seg.parsedOptions.TSVal, s.maxSentAck, seg.sequenceNumber)

	// Count the duplicates and do the fast retransmit if needed.
	// tcp的拥塞控制：检查是否有重复的ack，是否进入快速重传和快速恢复状态
	rtx := s.checkDuplicateAck(seg)

	// Stash away the current window size.
	// 存放当前窗口大小。
	s.sndWnd = seg.window

	// Ignore ack if it doesn't acknowledge any new data.
	// 获取确认号
	ack := seg.ackNumber
	// 如果ack在最小未确认的seq和下一seg的seq之间
	if (ack - 1).InRange(s.sndUna, s.sndNxt) {
		s.dupAckCount = 0
		// When an ack is received we must reset the timer. We stop it
		// here and it will be restarted later if needed.
		s.resendTimer.disable()

		// See : https://tools.ietf.org/html/rfc1323#section-3.3.
		// Specifically we should only update the RTO using TSEcr if the
		// following condition holds:
		//
		//    A TSecr value received in a segment is used to update the
		//    averaged RTT measurement only if the segment acknowledges
		//    some new data, i.e., only if it advances the left edge of
		//    the send window.
		if s.ep.sendTSOk && seg.parsedOptions.TSEcr != 0 {
			// TSVal/Ecr values sent by github.com/brewlin/net-protocol are at a millisecond
			// granularity.
			elapsed := time.Duration(s.ep.timestamp()-seg.parsedOptions.TSEcr) * time.Millisecond
			s.updateRTO(elapsed)
		}

		// 获取这次确认的字节数，即 ack - snaUna
		acked := s.sndUna.Size(ack)
		// 更新下一个未确认的序列号
		s.sndUna = ack

		ackLeft := acked
		originalOutstanding := s.outstanding
		// 从发送链表中删除已经确认的数据，发送窗口的滑动。
		for ackLeft > 0 {
			// We use logicalLen here because we can have FIN
			// segments (which are always at the end of list) that
			// have no data, but do consume a sequence number.
			seg := s.writeList.Front()
			datalen := seg.logicalLen()

			if datalen > ackLeft {
				seg.data.TrimFront(int(ackLeft))
				break
			}

			if s.writeNext == seg {
				s.writeNext = seg.Next()
			}
			// 从发送链表中删除已确认的tcp段。
			s.writeList.Remove(seg)
			// 因为有一个tcp段确认了，所以 outstanding 减1
			s.outstanding--
			seg.decRef()
			ackLeft -= datalen
		}

		// Update the send buffer usage and notify potential waiters.
		// 当收到ack确认时，需要更新发送缓冲占用
		s.ep.updateSndBufferUsage(int(acked))

		// If we are not in fast recovery then update the congestion
		// window based on the number of acknowledged packets.
		// tcp拥塞控制：如果没有进入快速恢复状态，那么根据确认的数据包的数量更新拥塞窗口。
		if !s.fr.active {
			// 调用相应拥塞控制算法的 Update
			s.cc.Update(originalOutstanding - s.outstanding)
		}

		// It is possible for s.outstanding to drop below zero if we get
		// a retransmit timeout, reset outstanding to zero but later
		// get an ack that cover previously sent data.
		// 如果发生超时重传时，s.outstanding可能会降到零以下，
		// 重置为零但后来得到一个覆盖先前发送数据的确认。
		if s.outstanding < 0 {
			s.outstanding = 0
		}
	}

	// Now that we've popped all acknowledged data from the retransmit
	// queue, retransmit if needed.
	// tcp拥塞控制：如果需要快速重传，则重传数据，但是只是重传下一个数据
	if rtx {
		// tcp拥塞控制：快速重传
		s.resendSegment()
	}

	// Send more data now that some of the pending data has been ack'd, or
	// that the window opened up, or the congestion window was inflated due
	// to a duplicate ack during fast recovery. This will also re-enable
	// the retransmit timer if needed.
	// 现在某些待处理数据已被确认，或者窗口打开，或者由于快速恢复期间出现重复的ack而导致拥塞窗口膨胀，
	// 因此发送更多数据。如果需要，这也将重新启用重传计时器。
	s.sendData()
}

// sendSegment sends a new segment containing the given payload, flags and
// sequence number.
// 根据给定的参数，负载数据、flags标记和序列号来发送数据
func (s *sender) sendSegment(data buffer.VectorisedView, flags byte, seq seqnum.Value) *tcpip.Error {
	s.lastSendTime = time.Now()
	if seq == s.rttMeasureSeqNum {
		s.rttMeasureTime = s.lastSendTime
	}

	rcvNxt, rcvWnd := s.ep.rcv.getSendParams()

	// Remember the max sent ack.
	s.maxSentAck = rcvNxt

	return s.ep.sendRaw(data, flags, seq, rcvNxt, rcvWnd)
}
