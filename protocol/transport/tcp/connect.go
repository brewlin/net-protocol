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
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/brewlin/net-protocol/pkg/buffer"
	"github.com/brewlin/net-protocol/pkg/rand"
	"github.com/brewlin/net-protocol/pkg/seqnum"
	"github.com/brewlin/net-protocol/pkg/sleep"
	"github.com/brewlin/net-protocol/pkg/waiter"
	tcpip "github.com/brewlin/net-protocol/protocol"
	"github.com/brewlin/net-protocol/protocol/header"
	"github.com/brewlin/net-protocol/stack"
)

// maxSegmentsPerWake is the maximum number of segments to process in the main
// protocol goroutine per wake-up. Yielding [after this number of segments are
// processed] allows other events to be processed as well (e.g., timeouts,
// resets, etc.).
const maxSegmentsPerWake = 100

type handshakeState int

// The following are the possible states of the TCP connection during a 3-way
// handshake. A depiction of the states and transitions can be found in RFC 793,
// page 23.
const (
	handshakeSynSent handshakeState = iota
	handshakeSynRcvd
	handshakeCompleted
)

// The following are used to set up sleepers.
const (
	wakerForNotification = iota
	wakerForNewSegment
	wakerForResend
	wakerForResolution
)

const (
	// Maximum space available for options.
	// tcp选项的最大长度
	maxOptionSize = 40
)

// handshake holds the state used during a TCP 3-way handshake.
// tcp三次握手时候使用的对象
type handshake struct {
	ep *endpoint
	// 握手的状态
	state  handshakeState
	active bool
	flags  uint8
	ackNum seqnum.Value

	// iss is the initial send sequence number, as defined in RFC 793.
	// 初始序列号
	iss seqnum.Value

	// rcvWnd is the receive window, as defined in RFC 793.
	// 接收窗口
	rcvWnd seqnum.Size

	// sndWnd is the send window, as defined in RFC 793.
	// 发送窗口
	sndWnd seqnum.Size

	// mss is the maximum segment size received from the peer.
	// 最大报文段大小
	mss uint16

	// sndWndScale is the send window scale, as defined in RFC 1323. A
	// negative value means no scaling is supported by the peer.
	// 发送窗口扩展因子
	sndWndScale int

	// rcvWndScale is the receive window scale, as defined in RFC 1323.
	// 接收窗口扩展因子
	rcvWndScale int
}

func newHandshake(ep *endpoint, rcvWnd seqnum.Size) (handshake, *tcpip.Error) {
	h := handshake{
		ep:          ep,
		active:      true,
		rcvWnd:      rcvWnd,
		rcvWndScale: FindWndScale(rcvWnd), // 接收窗口扩展因子
	}
	if err := h.resetState(); err != nil {
		return handshake{}, err
	}

	return h, nil
}

// FindWndScale determines the window scale to use for the given maximum window
// size.
// 因为窗口的大小不能超过序列号范围的一半，即窗口最大2^30,
// so (2^16)*(2^maxWnsScale) < 2^30,get maxWnsScale = 14
func FindWndScale(wnd seqnum.Size) int {
	if wnd < 0x10000 {
		return 0
	}

	max := seqnum.Size(0xffff)
	s := 0
	for wnd > max && s < header.MaxWndScale {
		s++
		max <<= 1
	}

	return s
}

// resetState resets the state of the handshake object such that it becomes
// ready for a new 3-way handshake.
func (h *handshake) resetState() *tcpip.Error {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}

	// 初始化状态为 SynSent
	h.state = handshakeSynSent
	h.flags = flagSyn
	h.ackNum = 0
	h.mss = 0
	h.iss = seqnum.Value(uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24)

	return nil
}

// effectiveRcvWndScale returns the effective receive window scale to be used.
// If the peer doesn't support window scaling, the effective rcv wnd scale is
// zero; otherwise it's the value calculated based on the initial rcv wnd.
func (h *handshake) effectiveRcvWndScale() uint8 {
	if h.sndWndScale < 0 {
		return 0
	}
	return uint8(h.rcvWndScale)
}

// resetToSynRcvd resets the state of the handshake object to the SYN-RCVD
// state.
func (h *handshake) resetToSynRcvd(iss seqnum.Value, irs seqnum.Value, opts *header.TCPSynOptions) {
	h.active = false
	h.state = handshakeSynRcvd
	h.flags = flagSyn | flagAck
	h.iss = iss
	h.ackNum = irs + 1
	h.mss = opts.MSS
	h.sndWndScale = opts.WS
}

// checkAck checks if the ACK number, if present, of a segment received during
// a TCP 3-way handshake is valid. If it's not, a RST segment is sent back in
// response.
func (h *handshake) checkAck(s *segment) bool {
	if s.flagIsSet(flagAck) && s.ackNumber != h.iss+1 {
		// RFC 793, page 36, states that a reset must be generated when
		// the connection is in any non-synchronized state and an
		// incoming segment acknowledges something not yet sent. The
		// connection remains in the same state.
		ack := s.sequenceNumber.Add(s.logicalLen())
		h.ep.sendRaw(buffer.VectorisedView{}, flagRst|flagAck, s.ackNumber, ack, 0)
		return false
	}

	return true
}

// synSentState handles a segment received when the TCP 3-way handshake is in
// the SYN-SENT state.
// synSentState 是客户端或者服务端接收到第一个握手报文的处理
// 正常情况下，如果是客户端，此时应该收到 syn+ack 报文，处理后发送 ack 报文给服务端。
// 如果是服务端，此时接收到syn报文，那么应该回复 syn+ack 报文给客户端，并设置状态为 handshakeSynRcvd。
func (h *handshake) synSentState(s *segment) *tcpip.Error {
	// RFC 793, page 37, states that in the SYN-SENT state, a reset is
	// acceptable if the ack field acknowledges the SYN.
	if s.flagIsSet(flagRst) {
		if s.flagIsSet(flagAck) && s.ackNumber == h.iss+1 {
			return tcpip.ErrConnectionRefused
		}
		return nil
	}

	if !h.checkAck(s) {
		return nil
	}

	// We are in the SYN-SENT state. We only care about segments that have
	// the SYN flag.
	if !s.flagIsSet(flagSyn) {
		return nil
	}

	// Parse the SYN options.
	rcvSynOpts := parseSynSegmentOptions(s)

	// Remember if the Timestamp option was negotiated.
	h.ep.maybeEnableTimestamp(&rcvSynOpts)

	// Remember if the SACKPermitted option was negotiated.
	h.ep.maybeEnableSACKPermitted(&rcvSynOpts)

	// Remember the sequence we'll ack from now on.
	h.ackNum = s.sequenceNumber + 1
	h.flags |= flagAck
	h.mss = rcvSynOpts.MSS
	h.sndWndScale = rcvSynOpts.WS

	// If this is a SYN ACK response, we only need to acknowledge the SYN
	// and the handshake is completed.
	// 客户端接收到了 syn+ack 报文
	if s.flagIsSet(flagAck) {
		// 客户端握手完成，发送 ack 报文给服务端
		h.state = handshakeCompleted
		// 最后依次 ack 报文丢了也没关系，因为后面一但发送任何数据包都是带ack的
		h.ep.sendRaw(buffer.VectorisedView{}, flagAck, h.iss+1, h.ackNum, h.rcvWnd>>h.effectiveRcvWndScale())
		return nil
	}

	// A SYN segment was received, but no ACK in it. We acknowledge the SYN
	// but resend our own SYN and wait for it to be acknowledged in the
	// SYN-RCVD state.
	// 服务端收到了 syn 报文，应该回复客户端 syn+ack 报文，且设置状态为 handshakeSynRcvd
	h.state = handshakeSynRcvd
	synOpts := header.TCPSynOptions{
		WS:    h.rcvWndScale,
		TS:    rcvSynOpts.TS,
		TSVal: h.ep.timestamp(),
		TSEcr: h.ep.recentTS,

		// We only send SACKPermitted if the other side indicated it
		// permits SACK. This is not explicitly defined in the RFC but
		// this is the behaviour implemented by Linux.
		SACKPermitted: rcvSynOpts.SACKPermitted,
	}
	// 发送 syn+ack 报文，如果该报文在链路中丢了，没有关系，客户端会重新发送 syn 报文
	sendSynTCP(&s.route, h.ep.id, h.flags, h.iss, h.ackNum, h.rcvWnd, synOpts)

	return nil
}

// synRcvdState handles a segment received when the TCP 3-way handshake is in
// the SYN-RCVD state.
// 正常情况下，会调用该函数来处理第三次 ack 报文
func (h *handshake) synRcvdState(s *segment) *tcpip.Error {
	if s.flagIsSet(flagRst) {
		// RFC 793, page 37, states that in the SYN-RCVD state, a reset
		// is acceptable if the sequence number is in the window.
		if s.sequenceNumber.InWindow(h.ackNum, h.rcvWnd) {
			return tcpip.ErrConnectionRefused
		}
		return nil
	}

	if !h.checkAck(s) {
		return nil
	}

	// 如果是syn报文，且序列号对应不上，那么返回 rst
	if s.flagIsSet(flagSyn) && s.sequenceNumber != h.ackNum-1 {
		// We received two SYN segments with different sequence
		// numbers, so we reset this and restart the whole
		// process, except that we don't reset the timer.
		ack := s.sequenceNumber.Add(s.logicalLen())
		seq := seqnum.Value(0)
		if s.flagIsSet(flagAck) {
			seq = s.ackNumber
		}
		h.ep.sendRaw(buffer.VectorisedView{}, flagRst|flagAck, seq, ack, 0)

		if !h.active {
			return tcpip.ErrInvalidEndpointState
		}

		if err := h.resetState(); err != nil {
			return err
		}
		synOpts := header.TCPSynOptions{
			WS:            h.rcvWndScale,
			TS:            h.ep.sendTSOk,
			TSVal:         h.ep.timestamp(),
			TSEcr:         h.ep.recentTS,
			SACKPermitted: h.ep.sackPermitted,
		}
		sendSynTCP(&s.route, h.ep.id, h.flags, h.iss, h.ackNum, h.rcvWnd, synOpts)
		return nil
	}

	// We have previously received (and acknowledged) the peer's SYN. If the
	// peer acknowledges our SYN, the handshake is completed.
	// 如果是 ack 报文，表明三次握手已经完成
	if s.flagIsSet(flagAck) {

		// If the timestamp option is negotiated and the segment does
		// not carry a timestamp option then the segment must be dropped
		// as per https://tools.ietf.org/html/rfc7323#section-3.2.
		if h.ep.sendTSOk && !s.parsedOptions.TS {
			h.ep.stack.Stats().DroppedPackets.Increment()
			return nil
		}

		// Update timestamp if required. See RFC7323, section-4.3.
		h.ep.updateRecentTimestamp(s.parsedOptions.TSVal, h.ackNum, s.sequenceNumber)

		h.state = handshakeCompleted
		return nil
	}

	return nil
}

// 握手的时候处理tcp段
func (h *handshake) handleSegment(s *segment) *tcpip.Error {
	h.sndWnd = s.window
	if !s.flagIsSet(flagSyn) && h.sndWndScale > 0 {
		h.sndWnd <<= uint8(h.sndWndScale)
	}

	switch h.state {
	case handshakeSynRcvd:
		// 正常情况下，服务端接收客户端第三次 ack 报文
		return h.synRcvdState(s)
	case handshakeSynSent:
		// 客户端发送了syn报文后的处理
		return h.synSentState(s)
	}
	return nil
}

// processSegments goes through the segment queue and processes up to
// maxSegmentsPerWake (if they're available).
func (h *handshake) processSegments() *tcpip.Error {
	for i := 0; i < maxSegmentsPerWake; i++ {
		// 从队列中取出一个tcp段
		s := h.ep.segmentQueue.dequeue()
		if s == nil {
			return nil
		}

		// 处理tcp段
		err := h.handleSegment(s)
		s.decRef()
		if err != nil {
			return err
		}

		// We stop processing packets once the handshake is completed,
		// otherwise we may process packets meant to be processed by
		// the main protocol goroutine.
		if h.state == handshakeCompleted {
			break
		}
	}

	// If the queue is not empty, make sure we'll wake up in the next
	// iteration.
	if !h.ep.segmentQueue.empty() {
		h.ep.newSegmentWaker.Assert()
	}

	return nil
}

func (h *handshake) resolveRoute() *tcpip.Error {
	log.Printf("tcp resolveRoute")
	// Set up the wakers.
	s := sleep.Sleeper{}
	resolutionWaker := &sleep.Waker{}
	s.AddWaker(resolutionWaker, wakerForResolution)
	s.AddWaker(&h.ep.notificationWaker, wakerForNotification)
	defer s.Done()

	// Initial action is to resolve route.
	index := wakerForResolution
	for {
		switch index {
		case wakerForResolution:
			if _, err := h.ep.route.Resolve(resolutionWaker); err != tcpip.ErrWouldBlock {
				// Either success (err == nil) or failure.
				return err
			}
			// Resolution not completed. Keep trying...

		case wakerForNotification:
			n := h.ep.fetchNotifications()
			if n&notifyClose != 0 {
				h.ep.route.RemoveWaker(resolutionWaker)
				return tcpip.ErrAborted
			}
			if n&notifyDrain != 0 {
				close(h.ep.drainDone)
				<-h.ep.undrain
			}
		}

		// Wait for notification.
		index, _ = s.Fetch(true)
	}
}

// execute executes the TCP 3-way handshake.
// 执行tcp 3次握手，客户端和服务端都是调用该函数来实现三次握手
/*
			c				s
			|				|
   sync_sent|------sync---->|sync_rcvd
			|				|
			|				|
 established|<--sync|ack----|
			|				|
			|				|
			|------ack----->|established
*/
func (h *handshake) execute() *tcpip.Error {
	// 是否需要拿到下一条地址
	if h.ep.route.IsResolutionRequired() {
		if err := h.resolveRoute(); err != nil {
			return err
		}
	}

	// Initialize the resend timer.
	// 初始化重传定时器
	resendWaker := sleep.Waker{}
	// 设置1s超时
	timeOut := time.Duration(time.Second)
	rt := time.AfterFunc(timeOut, func() {
		resendWaker.Assert()
	})
	defer rt.Stop()

	// Set up the wakers.
	s := sleep.Sleeper{}
	s.AddWaker(&resendWaker, wakerForResend)
	s.AddWaker(&h.ep.notificationWaker, wakerForNotification)
	s.AddWaker(&h.ep.newSegmentWaker, wakerForNewSegment)
	defer s.Done()

	var sackEnabled SACKEnabled
	// 是否开启 sack
	if err := h.ep.stack.TransportProtocolOption(ProtocolNumber, &sackEnabled); err != nil {
		// If stack returned an error when checking for SACKEnabled
		// status then just default to switching off SACK negotiation.
		sackEnabled = false
	}

	// Send the initial SYN segment and loop until the handshake is
	// completed.
	// sync报文的选项参数
	synOpts := header.TCPSynOptions{
		WS:            h.rcvWndScale,
		TS:            true,
		TSVal:         h.ep.timestamp(),
		TSEcr:         h.ep.recentTS,
		SACKPermitted: bool(sackEnabled),
	}

	// Execute is also called in a listen context so we want to make sure we
	// only send the TS/SACK option when we received the TS/SACK in the
	// initial SYN.
	// 表示服务端收到了syn报文
	if h.state == handshakeSynRcvd {
		synOpts.TS = h.ep.sendTSOk
		synOpts.SACKPermitted = h.ep.sackPermitted && bool(sackEnabled)
	}
	// 如果是客户端发送 syn 报文，如果是服务端发送 syn+ack 报文

	sendSynTCP(&h.ep.route, h.ep.id, h.flags, h.iss, h.ackNum, h.rcvWnd, synOpts)
	// 判断握手是否结束，没有结束则循环
	for h.state != handshakeCompleted {
		// 获取事件id
		switch index, _ := s.Fetch(true); index {
		case wakerForResend:
			// 如果是客户端当发送 syn 报文，超过一定的时间未收到回包，触发超时重传
			// 如果是服务端当发送 syn+ack 报文，超过一定的时间未收到 ack 回包，触发超时重传
			// 超时时间变为上次的2倍
			timeOut *= 2
			if timeOut > 60*time.Second {
				return tcpip.ErrTimeout
			}
			rt.Reset(timeOut)
			// 重新发送syn报文
			sendSynTCP(&h.ep.route, h.ep.id, h.flags, h.iss, h.ackNum, h.rcvWnd, synOpts)

		case wakerForNotification:
			n := h.ep.fetchNotifications()
			if n&notifyClose != 0 {
				return tcpip.ErrAborted
			}
			if n&notifyDrain != 0 {
				for !h.ep.segmentQueue.empty() {
					s := h.ep.segmentQueue.dequeue()
					err := h.handleSegment(s)
					s.decRef()
					if err != nil {
						return err
					}
					if h.state == handshakeCompleted {
						return nil
					}
				}
				close(h.ep.drainDone)
				<-h.ep.undrain
			}

		case wakerForNewSegment:
			// 处理握手报文
			if err := h.processSegments(); err != nil {
				return err
			}
		}
	}

	return nil
}

func parseSynSegmentOptions(s *segment) header.TCPSynOptions {
	synOpts := header.ParseSynOptions(s.options, s.flagIsSet(flagAck))
	if synOpts.TS {
		s.parsedOptions.TSVal = synOpts.TSVal
		s.parsedOptions.TSEcr = synOpts.TSEcr
	}
	return synOpts
}

var optionPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, maxOptionSize)
	},
}

func getOptions() []byte {
	return optionPool.Get().([]byte)
}

func putOptions(options []byte) {
	// Reslice to full capacity.
	optionPool.Put(options[0:cap(options)])
}

// tcp选项的编码
func makeSynOptions(opts header.TCPSynOptions) []byte {
	// Emulate linux option order. This is as follows:
	//
	// if md5: NOP NOP MD5SIG 18 md5sig(16)
	// if mss: MSS 4 mss(2)
	// if ts and sack_advertise:
	//	SACK 2 TIMESTAMP 2 timestamp(8)
	// elif ts: NOP NOP TIMESTAMP 10 timestamp(8)
	// elif sack: NOP NOP SACK 2
	// if wscale: NOP WINDOW 3 ws(1)
	// if sack_blocks: NOP NOP SACK ((2 + (#blocks * 8))
	//	[for each block] start_seq(4) end_seq(4)
	// if fastopen_cookie:
	//	if exp: EXP (4 + len(cookie)) FASTOPEN_MAGIC(2)
	// 	else: FASTOPEN (2 + len(cookie))
	//	cookie(variable) [padding to four bytes]
	//
	options := getOptions()

	// Always encode the mss.
	offset := header.EncodeMSSOption(uint32(opts.MSS), options)

	// Special ordering is required here. If both TS and SACK are enabled,
	// then the SACK option precedes TS, with no padding. If they are
	// enabled individually, then we see padding before the option.
	if opts.TS && opts.SACKPermitted {
		offset += header.EncodeSACKPermittedOption(options[offset:])
		offset += header.EncodeTSOption(opts.TSVal, opts.TSEcr, options[offset:])
	} else if opts.TS {
		offset += header.EncodeNOP(options[offset:])
		offset += header.EncodeNOP(options[offset:])
		offset += header.EncodeTSOption(opts.TSVal, opts.TSEcr, options[offset:])
	} else if opts.SACKPermitted {
		offset += header.EncodeNOP(options[offset:])
		offset += header.EncodeNOP(options[offset:])
		offset += header.EncodeSACKPermittedOption(options[offset:])
	}

	// Initialize the WS option.
	if opts.WS >= 0 {
		offset += header.EncodeNOP(options[offset:])
		offset += header.EncodeWSOption(opts.WS, options[offset:])
	}

	// Padding to the end; note that this never apply unless we add a
	// fastopen option, we always expect the offset to remain the same.
	if delta := header.AddTCPOptionPadding(options, offset); delta != 0 {
		panic("unexpected option encoding")
	}

	return options[:offset]
}

// 封装 sendTCP ，发送 syn 报文
func sendSynTCP(r *stack.Route, id stack.TransportEndpointID, flags byte, seq, ack seqnum.Value, rcvWnd seqnum.Size, opts header.TCPSynOptions) *tcpip.Error {
	// The MSS in opts is automatically calculated as this function is
	// called from many places and we don't want every call point being
	// embedded with the MSS calculation.

	if opts.MSS == 0 {
		opts.MSS = uint16(r.MTU() - header.TCPMinimumSize)
	}

	options := makeSynOptions(opts)
	err := sendTCP(r, id, buffer.VectorisedView{}, r.DefaultTTL(), flags, seq, ack, rcvWnd, options)
	putOptions(options)
	return err
}

// sendTCP sends a TCP segment with the provided options via the provided
// network endpoint and under the provided identity.
// 发送一个tcp段数据，封装 tcp 首部，并写入网路层
func sendTCP(r *stack.Route, id stack.TransportEndpointID, data buffer.VectorisedView, ttl uint8, flags byte, seq, ack seqnum.Value, rcvWnd seqnum.Size, opts []byte) *tcpip.Error {
	optLen := len(opts)
	// Allocate a buffer for the TCP header.
	hdr := buffer.NewPrependable(header.TCPMinimumSize + int(r.MaxHeaderLength()) + optLen)

	if rcvWnd > 0xffff {
		rcvWnd = 0xffff
	}

	// Initialize the header.
	tcp := header.TCP(hdr.Prepend(header.TCPMinimumSize + optLen))
	tcp.Encode(&header.TCPFields{
		SrcPort:    id.LocalPort,
		DstPort:    id.RemotePort,
		SeqNum:     uint32(seq),
		AckNum:     uint32(ack),
		DataOffset: uint8(header.TCPMinimumSize + optLen),
		Flags:      flags,
		WindowSize: uint16(rcvWnd),
	})
	copy(tcp[header.TCPMinimumSize:], opts)

	// Only calculate the checksum if offloading isn't supported.
	if r.Capabilities()&stack.CapabilityChecksumOffload == 0 {
		length := uint16(hdr.UsedLength() + data.Size())
		// tcp伪首部校验和的计算
		xsum := r.PseudoHeaderChecksum(ProtocolNumber)
		for _, v := range data.Views() {
			xsum = header.Checksum(v, xsum)
		}

		// tcp的可靠性：校验和的计算，用于检测损伤的报文段
		tcp.SetChecksum(^tcp.CalculateChecksum(xsum, length))
	}

	r.Stats().TCP.SegmentsSent.Increment()
	if (flags & flagRst) != 0 {
		r.Stats().TCP.ResetsSent.Increment()
	}

	log.Printf("@传输层 tcp: send tcp %s segment to %s, seq: %d, ack: %d, rcvWnd: %d",
		flagString(flags), fmt.Sprintf("%s:%d", id.RemoteAddress, id.RemotePort),
		seq, ack, rcvWnd)

	return r.WritePacket(hdr, data, ProtocolNumber, ttl)
}

// makeOptions makes an options slice.
func (e *endpoint) makeOptions(sackBlocks []header.SACKBlock) []byte {
	options := getOptions()
	offset := 0

	// N.B. the ordering here matches the ordering used by Linux internally
	// and described in the raw makeOptions function. We don't include
	// unnecessary cases here (post connection.)
	if e.sendTSOk {
		// Embed the timestamp if timestamp has been enabled.
		//
		// We only use the lower 32 bits of the unix time in
		// milliseconds. This is similar to what Linux does where it
		// uses the lower 32 bits of the jiffies value in the tsVal
		// field of the timestamp option.
		//
		// Further, RFC7323 section-5.4 recommends millisecond
		// resolution as the lowest recommended resolution for the
		// timestamp clock.
		//
		// Ref: https://tools.ietf.org/html/rfc7323#section-5.4.
		offset += header.EncodeNOP(options[offset:])
		offset += header.EncodeNOP(options[offset:])
		offset += header.EncodeTSOption(e.timestamp(), uint32(e.recentTS), options[offset:])
	}
	if e.sackPermitted && len(sackBlocks) > 0 {
		offset += header.EncodeNOP(options[offset:])
		offset += header.EncodeNOP(options[offset:])
		offset += header.EncodeSACKBlocks(sackBlocks, options[offset:])
	}

	// We expect the above to produce an aligned offset.
	if delta := header.AddTCPOptionPadding(options, offset); delta != 0 {
		panic("unexpected option encoding")
	}

	return options[:offset]
}

// sendRaw sends a TCP segment to the endpoint's peer.
// 发送一个tcp段给对端
func (e *endpoint) sendRaw(data buffer.VectorisedView, flags byte, seq, ack seqnum.Value, rcvWnd seqnum.Size) *tcpip.Error {
	var sackBlocks []header.SACKBlock
	if e.state == stateConnected && e.rcv.pendingBufSize > 0 && (flags&flagAck != 0) {
		sackBlocks = e.sack.Blocks[:e.sack.NumBlocks]
	}
	options := e.makeOptions(sackBlocks)
	err := sendTCP(&e.route, e.id, data, e.route.DefaultTTL(), flags, seq, ack, rcvWnd, options)
	putOptions(options)
	return err
}

// 从发送队列中取出数据并发送出去
func (e *endpoint) handleWrite() *tcpip.Error {
	// Move packets from send queue to send list. The queue is accessible
	// from other goroutines and protected by the send mutex, while the send
	// list is only accessible from the handler goroutine, so it needs no
	// mutexes.
	e.sndBufMu.Lock()

	// 得到第一个tcp段
	first := e.sndQueue.Front()
	if first != nil {
		e.snd.writeList.PushBackList(&e.sndQueue)
		e.snd.sndNxtList.UpdateForward(e.sndBufInQueue)
		e.sndBufInQueue = 0
	}

	e.sndBufMu.Unlock()

	// Initialize the next segment to write if it's currently nil.
	if e.snd.writeNext == nil {
		e.snd.writeNext = first
	}

	// Push out any new packets.
	// 将数据发送出去
	e.snd.sendData()

	return nil
}

// 关闭连接的处理，最终会调用 sendData 来发送 fin 包
func (e *endpoint) handleClose() *tcpip.Error {
	// Drain the send queue.
	e.handleWrite()

	// Mark send side as closed.
	// 标记发送器关闭
	e.snd.closed = true

	return nil
}

// resetConnectionLocked sends a RST segment and puts the endpoint in an error
// state with the given error code. This method must only be called from the
// protocol goroutine.
// resetConnectionLocked 发送一个RST段，并将端点置于具有给定错误代码的错误状态。
// 只能从协议goroutine中调用此方法。
func (e *endpoint) resetConnectionLocked(err *tcpip.Error) {
	e.sendRaw(buffer.VectorisedView{}, flagAck|flagRst, e.snd.sndUna, e.rcv.rcvNxt, 0)

	e.state = stateError
	e.hardError = err
}

// completeWorkerLocked is called by the worker goroutine when it's about to
// exit. It marks the worker as completed and performs cleanup work if requested
// by Close().
func (e *endpoint) completeWorkerLocked() {
	e.workerRunning = false
	if e.workerCleanup {
		e.cleanupLocked()
	}
}

// handleSegments pulls segments from the queue and processes them. It returns
// no error if the protocol loop should continue, an error otherwise.
// handleSegments 从队列中取出 tcp 段数据，然后处理它们。
func (e *endpoint) handleSegments() *tcpip.Error {
	checkRequeue := true
	for i := 0; i < maxSegmentsPerWake; i++ {
		s := e.segmentQueue.dequeue()
		if s == nil {
			checkRequeue = false
			break
		}

		// Invoke the tcp probe if installed.
		// 如果已安装 则调用探针
		if e.probe != nil {
			e.probe(e.completeState())
		}

		if s.flagIsSet(flagRst) {
			// 如果收到 rst 报文
			if e.rcv.acceptable(s.sequenceNumber, 0) {
				// RFC 793, page 37 states that "in all states
				// except SYN-SENT, all reset (RST) segments are
				// validated by checking their SEQ-fields." So
				// we only process it if it's acceptable.
				s.decRef()
				return tcpip.ErrConnectionReset
			}
		} else if s.flagIsSet(flagAck) {
			// 处理正常的报文
			// Patch the window size in the segment according to the
			// send window scale.
			s.window <<= e.snd.sndWndScale

			// If the timestamp option is negotiated and the segment
			// does not carry a timestamp option then the segment
			// must be dropped as per
			// https://tools.ietf.org/html/rfc7323#section-3.2.
			if e.sendTSOk && !s.parsedOptions.TS {
				e.stack.Stats().DroppedPackets.Increment()
				s.decRef()
				continue
			}

			// RFC 793, page 41 states that "once in the ESTABLISHED
			// state all segments must carry current acknowledgment
			// information."
			// 处理tcp数据段，同时给接收器和发送器
			// 为何要给发送器传接收到的数据段呢？主要是为了滑动窗口的滑动和拥塞控制处理
			e.rcv.handleRcvdSegment(s)
			e.snd.handleRcvdSegment(s)
		}
		s.decRef()
	}

	// If the queue is not empty, make sure we'll wake up in the next
	// iteration.
	if checkRequeue && !e.segmentQueue.empty() {
		e.newSegmentWaker.Assert()
	}

	// Send an ACK for all processed packets if needed.
	// tcp可靠性：累积确认
	// 如果发送的最大ack不等于下一个接收的序列号，发送ack
	if e.rcv.rcvNxt != e.snd.maxSentAck {
		e.snd.sendAck()
	}

	e.resetKeepaliveTimer(true)

	return nil
}

// keepaliveTimerExpired is called when the keepaliveTimer fires. We send TCP
// keepalive packets periodically when the connection is idle. If we don't hear
// from the other side after a number of tries, we terminate the connection.
func (e *endpoint) keepaliveTimerExpired() *tcpip.Error {
	e.keepalive.Lock()
	if !e.keepalive.enabled || !e.keepalive.timer.checkExpiration() {
		e.keepalive.Unlock()
		return nil
	}

	if e.keepalive.unacked >= e.keepalive.count {
		e.keepalive.Unlock()
		return tcpip.ErrConnectionReset
	}

	// RFC1122 4.2.3.6: TCP keepalive is a dataless ACK with
	// seg.seq = snd.nxt-1.
	e.keepalive.unacked++
	e.keepalive.Unlock()
	e.snd.sendSegment(buffer.VectorisedView{}, flagAck, e.snd.sndNxt-1)
	e.resetKeepaliveTimer(false)
	return nil
}

// resetKeepaliveTimer restarts or stops the keepalive timer, depending on
// whether it is enabled for this endpoint.
func (e *endpoint) resetKeepaliveTimer(receivedData bool) {
	e.keepalive.Lock()
	defer e.keepalive.Unlock()
	if receivedData {
		e.keepalive.unacked = 0
	}
	// Start the keepalive timer IFF it's enabled and there is no pending
	// data to send.
	if !e.keepalive.enabled || e.snd == nil || e.snd.sndUna != e.snd.sndNxt {
		e.keepalive.timer.disable()
		return
	}
	if e.keepalive.unacked > 0 {
		e.keepalive.timer.enable(e.keepalive.interval)
	} else {
		e.keepalive.timer.enable(e.keepalive.idle)
	}
}

// disableKeepaliveTimer stops the keepalive timer.
func (e *endpoint) disableKeepaliveTimer() {
	e.keepalive.Lock()
	e.keepalive.timer.disable()
	e.keepalive.Unlock()
}

// protocolMainLoop is the main loop of the TCP protocol. It runs in its own
// goroutine and is responsible for sending segments and handling received
// segments.
// protocolMainLoop 是TCP协议的主循环。它在自己的goroutine中运行，负责握手、发送段和处理收到的段。
func (e *endpoint) protocolMainLoop(handshake bool) *tcpip.Error {
	var closeTimer *time.Timer
	var closeWaker sleep.Waker

	// 收尾的一些工作
	epilogue := func() {
		// e.mu is expected to be hold upon entering this section.

		if e.snd != nil {
			e.snd.resendTimer.cleanup()
		}

		if closeTimer != nil {
			closeTimer.Stop()
		}

		e.completeWorkerLocked()

		if e.drainDone != nil {
			close(e.drainDone)
		}

		e.mu.Unlock()

		// When the protocol loop exits we should wake up our waiters.
		e.waiterQueue.Notify(waiter.EventHUp | waiter.EventErr | waiter.EventIn | waiter.EventOut)
	}

	// 如果需要三次握手
	if handshake {
		// This is an active connection, so we must initiate the 3-way
		// handshake, and then inform potential waiters about its
		// completion.
		h, err := newHandshake(e, seqnum.Size(e.receiveBufferAvailable()))
		if err == nil {
			// 执行握手
			err = h.execute()
		}
		// 处理握手有错
		if err != nil {
			e.lastErrorMu.Lock()
			e.lastError = err
			e.lastErrorMu.Unlock()

			e.mu.Lock()
			e.state = stateError
			e.hardError = err
			// Lock released below.
			epilogue()

			return err
		}

		// Transfer handshake state to TCP connection. We disable
		// receive window scaling if the peer doesn't support it
		// (indicated by a negative send window scale).
		// 到这里就表示三次握手已经成功了，那么初始化发送器和接收器
		e.snd = newSender(e, h.iss, h.ackNum-1, h.sndWnd, h.mss, h.sndWndScale)

		e.rcvListMu.Lock()
		e.rcv = newReceiver(e, h.ackNum-1, h.rcvWnd, h.effectiveRcvWndScale())
		e.rcvListMu.Unlock()
	}

	e.keepalive.timer.init(&e.keepalive.waker)
	defer e.keepalive.timer.cleanup()

	// Tell waiters that the endpoint is connected and writable.
	e.mu.Lock()
	e.state = stateConnected
	drained := e.drainDone != nil
	e.mu.Unlock()
	if drained {
		close(e.drainDone)
		<-e.undrain
	}

	e.waiterQueue.Notify(waiter.EventOut)

	// Set up the functions that will be called when the main protocol loop
	// wakes up.
	// 触发器的事件，这些函数很重要
	funcs := []struct {
		w *sleep.Waker
		f func() *tcpip.Error
	}{
		{
			w: &e.sndWaker,
			f: e.handleWrite,
		},
		{
			w: &e.sndCloseWaker,
			f: e.handleClose,
		},
		{
			w: &e.newSegmentWaker,
			f: e.handleSegments,
		},
		{
			w: &closeWaker,
			f: func() *tcpip.Error {
				return tcpip.ErrConnectionAborted
			},
		},
		{
			w: &e.snd.resendWaker,
			f: func() *tcpip.Error {
				// 如果重传触发了，表示在rto时间内没有收到ack包
				// 也就是说假设它丢包了
				if !e.snd.retransmitTimerExpired() {
					return tcpip.ErrTimeout
				}
				return nil
			},
		},
		{
			w: &e.keepalive.waker,
			f: e.keepaliveTimerExpired,
		},
		{
			w: &e.notificationWaker,
			f: func() *tcpip.Error {
				n := e.fetchNotifications()
				if n&notifyNonZeroReceiveWindow != 0 {
					e.rcv.nonZeroWindow()
				}

				if n&notifyReceiveWindowChanged != 0 {
					e.rcv.pendingBufSize = seqnum.Size(e.receiveBufferSize())
				}

				if n&notifyMTUChanged != 0 {
					e.sndBufMu.Lock()
					count := e.packetTooBigCount
					e.packetTooBigCount = 0
					mtu := e.sndMTU
					e.sndBufMu.Unlock()

					e.snd.updateMaxPayloadSize(mtu, count)
				}

				if n&notifyReset != 0 {
					e.mu.Lock()
					e.resetConnectionLocked(tcpip.ErrConnectionAborted)
					e.mu.Unlock()
				}
				if n&notifyClose != 0 && closeTimer == nil {
					// Reset the connection 3 seconds after the
					// endpoint has been closed.
					closeTimer = time.AfterFunc(3*time.Second, func() {
						closeWaker.Assert()
					})
				}

				if n&notifyDrain != 0 {
					for !e.segmentQueue.empty() {
						if err := e.handleSegments(); err != nil {
							return err
						}
					}
					close(e.drainDone)
					<-e.undrain
				}

				if n&notifyKeepaliveChanged != 0 {
					e.resetKeepaliveTimer(true)
				}

				return nil
			},
		},
	}

	// Initialize the sleeper based on the wakers in funcs.
	s := sleep.Sleeper{}
	for i := range funcs {
		s.AddWaker(funcs[i].w, i)
	}

	// The following assertions and notifications are needed for restored
	// endpoints. Fresh newly created endpoints have empty states and should
	// not invoke any.
	// 恢复的端点需要以下断言和通知。新创建的新端点具有空状态，不应调用任何端点。
	e.segmentQueue.mu.Lock()
	if !e.segmentQueue.list.Empty() {
		e.newSegmentWaker.Assert()
	}
	e.segmentQueue.mu.Unlock()

	e.rcvListMu.Lock()
	if !e.rcvList.Empty() {
		e.waiterQueue.Notify(waiter.EventIn)
	}
	e.rcvListMu.Unlock()

	e.mu.RLock()
	if e.workerCleanup {
		e.notifyProtocolGoroutine(notifyClose)
	}
	e.mu.RUnlock()

	// Main loop. Handle segments until both send and receive ends of the
	// connection have completed.
	// 主循环，处理tcp报文
	// 要使这个主循环结束，也就是tcp连接完全关闭，得同时满足三个条件：
	// 1，接收器关闭了 2，发送器关闭了 3，下一个未确认的序列号等于添加到发送列表的下一个段的序列号
	for !e.rcv.closed || !e.snd.closed || e.snd.sndUna != e.snd.sndNxtList {
		e.workMu.Unlock()
		// s.Fetch 会返回事件的index，比如 v=0 的话，
		// funcs[v].f()就是调用 e.handleWrite
		// 所以这里的函数应该尽量不阻塞，否则会影响其他事件的接收
		v, _ := s.Fetch(true)
		e.workMu.Lock()
		if err := funcs[v].f(); err != nil {
			e.mu.Lock()
			e.resetConnectionLocked(err)
			// Lock released below.
			epilogue()

			return nil
		}
	}

	// Mark endpoint as closed.
	e.mu.Lock()
	if e.state != stateError {
		e.state = stateClosed
	}
	// Lock released below.
	epilogue()

	return nil
}
