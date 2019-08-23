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

// renoState stores the variables related to TCP New Reno congestion
// control algorithm.
//
// +stateify savable
type renoState struct {
	s *sender
}

// newRenoCC initializes the state for the NewReno congestion control algorithm.
// 新建 reno 算法对象
func newRenoCC(s *sender) *renoState {
	return &renoState{s: s}
}

// updateSlowStart will update the congestion window as per the slow-start
// algorithm used by NewReno. If after adjusting the congestion window
// we cross the SSthreshold then it will return the number of packets that
// must be consumed in congestion avoidance mode.
// updateSlowStart 将根据NewReno使用的慢启动算法更新拥塞窗口。
// 如果在调整拥塞窗口后我们越过了 SSthreshold ，那么它将返回在拥塞避免模式下必须消耗的数据包的数量。
func (r *renoState) updateSlowStart(packetsAcked int) int {
	// Don't let the congestion window cross into the congestion
	// avoidance range.
	// 在慢启动阶段，每次收到ack，sndCwnd加上已确认的段数
	newcwnd := r.s.sndCwnd + packetsAcked
	// 判断增大过后的拥塞窗口是否超过慢启动阀值 sndSsthresh，
	// 如果超过 sndSsthresh ，将窗口调整为 sndSsthresh
	if newcwnd >= r.s.sndSsthresh {
		newcwnd = r.s.sndSsthresh
		r.s.sndCAAckCount = 0
	}
	// 是否超过 sndSsthresh， packetsAcked>0表示超过
	packetsAcked -= newcwnd - r.s.sndCwnd
	// 更新拥塞窗口
	r.s.sndCwnd = newcwnd
	return packetsAcked
}

// updateCongestionAvoidance will update congestion window in congestion
// avoidance mode as described in RFC5681 section 3.1
// updateCongestionAvoidance 将在拥塞避免模式下更新拥塞窗口，
// 如RFC5681第3.1节所述
func (r *renoState) updateCongestionAvoidance(packetsAcked int) {
	// Consume the packets in congestion avoidance mode.
	// sndCAAckCount 累计收到的tcp段数
	r.s.sndCAAckCount += packetsAcked
	// 如果累计的段数超过当前的拥塞窗口，那么 sndCwnd 加上 sndCAAckCount/sndCwnd 的整数倍
	if r.s.sndCAAckCount >= r.s.sndCwnd {
		r.s.sndCwnd += r.s.sndCAAckCount / r.s.sndCwnd
		r.s.sndCAAckCount = r.s.sndCAAckCount % r.s.sndCwnd
	}
}

// reduceSlowStartThreshold reduces the slow-start threshold per RFC 5681,
// page 6, eq. 4. It is called when we detect congestion in the network.
// 当检测到网络拥塞时，调用 reduceSlowStartThreshold。
// 它将 sndSsthresh 变为 outstanding 的一半。
// sndSsthresh 最小为2，因为至少要比丢包后的拥塞窗口（cwnd=1）来的大，才会进入慢启动阶段。
func (r *renoState) reduceSlowStartThreshold() {
	r.s.sndSsthresh = r.s.outstanding / 2
	if r.s.sndSsthresh < 2 {
		r.s.sndSsthresh = 2
	}
}

// Update updates the congestion state based on the number of packets that
// were acknowledged.
// Update implements congestionControl.Update.
// packetsAcked 表示已确认的tcp段数
func (r *renoState) Update(packetsAcked int) {
	// 当拥塞窗口没有超过慢启动阀值的时候，使用慢启动来增大窗口，
	// 否则进入拥塞避免阶段
	if r.s.sndCwnd < r.s.sndSsthresh {
		packetsAcked = r.updateSlowStart(packetsAcked)
		if packetsAcked == 0 {
			return
		}
	}
	// 进入拥塞避免阶段
	r.updateCongestionAvoidance(packetsAcked)
}

// HandleNDupAcks implements congestionControl.HandleNDupAcks.
// 当收到三个重复ack时，调用 HandleNDupAcks 来处理。
func (r *renoState) HandleNDupAcks() {
	// A retransmit was triggered due to nDupAckThreshold
	// being hit. Reduce our slow start threshold.
	// 减小慢启动阀值
	r.reduceSlowStartThreshold()
}

// HandleRTOExpired implements congestionControl.HandleRTOExpired.
// 当当发生重传包时，调用 HandleRTOExpired 来处理。
func (r *renoState) HandleRTOExpired() {
	// We lost a packet, so reduce ssthresh.
	// 减小慢启动阀值
	r.reduceSlowStartThreshold()

	// Reduce the congestion window to 1, i.e., enter slow-start. Per
	// RFC 5681, page 7, we must use 1 regardless of the value of the
	// initial congestion window.
	// 更新拥塞窗口为1，这样就会重新进入慢启动
	r.s.sndCwnd = 1
}

// PostRecovery implements congestionControl.PostRecovery.
func (r *renoState) PostRecovery() {
	// noop.
}
