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
	"strings"
	"sync/atomic"

	"github.com/brewlin/net-protocol/pkg/buffer"
	"github.com/brewlin/net-protocol/protocol/header"
	"github.com/brewlin/net-protocol/pkg/seqnum"
	"github.com/brewlin/net-protocol/stack"
)

// Flags that may be set in a TCP segment.
const (
	flagFin = 1 << iota
	flagSyn
	flagRst
	flagPsh
	flagAck
	flagUrg
)

func flagString(flags uint8) string {
	var s []string
	if (flags & flagAck) != 0 {
		s = append(s, "ack")
	}
	if (flags & flagFin) != 0 {
		s = append(s, "fin")
	}
	if (flags & flagPsh) != 0 {
		s = append(s, "psh")
	}
	if (flags & flagRst) != 0 {
		s = append(s, "rst")
	}
	if (flags & flagSyn) != 0 {
		s = append(s, "syn")
	}
	if (flags & flagUrg) != 0 {
		s = append(s, "urg")
	}
	return strings.Join(s, "|")
}

// segment represents a TCP segment. It holds the payload and parsed TCP segment
// information, and can be added to intrusive lists.
// segment is mostly immutable, the only field allowed to change is viewToDeliver.
//
// +stateify savable
type segment struct {
	segmentEntry
	refCnt int32
	id     stack.TransportEndpointID
	route  stack.Route
	data   buffer.VectorisedView
	// views is used as buffer for data when its length is large
	// enough to store a VectorisedView.
	views [8]buffer.View
	// viewToDeliver keeps track of the next View that should be
	// delivered by the Read endpoint.
	viewToDeliver  int
	sequenceNumber seqnum.Value
	ackNumber      seqnum.Value
	flags          uint8
	window         seqnum.Size

	// parsedOptions stores the parsed values from the options in the segment.
	parsedOptions header.TCPOptions
	options       []byte
}

func newSegment(r *stack.Route, id stack.TransportEndpointID, vv buffer.VectorisedView) *segment {
	s := &segment{
		refCnt: 1,
		id:     id,
		route:  r.Clone(),
	}
	s.data = vv.Clone(s.views[:])
	return s
}

func newSegmentFromView(r *stack.Route, id stack.TransportEndpointID, v buffer.View) *segment {
	s := &segment{
		refCnt: 1,
		id:     id,
		route:  r.Clone(),
	}
	s.views[0] = v
	s.data = buffer.NewVectorisedView(len(v), s.views[:1])
	return s
}

func (s *segment) clone() *segment {
	t := &segment{
		refCnt:         1,
		id:             s.id,
		sequenceNumber: s.sequenceNumber,
		ackNumber:      s.ackNumber,
		flags:          s.flags,
		window:         s.window,
		route:          s.route.Clone(),
		viewToDeliver:  s.viewToDeliver,
	}
	t.data = s.data.Clone(t.views[:])
	return t
}

func (s *segment) flagIsSet(flag uint8) bool {
	return (s.flags & flag) != 0
}

func (s *segment) decRef() {
	if atomic.AddInt32(&s.refCnt, -1) == 0 {
		s.route.Release()
	}
}

func (s *segment) incRef() {
	atomic.AddInt32(&s.refCnt, 1)
}

// logicalLen is the segment length in the sequence number space. It's defined
// as the data length plus one for each of the SYN and FIN bits set.
// 计算tcp段的逻辑长度，包括负载数据的长度，如果有控制标记，需要加1
func (s *segment) logicalLen() seqnum.Size {
	l := seqnum.Size(s.data.Size())
	if s.flagIsSet(flagSyn) {
		l++
	}
	if s.flagIsSet(flagFin) {
		l++
	}
	return l
}

// parse populates the sequence & ack numbers, flags, and window fields of the
// segment from the TCP header stored in the data. It then updates the view to
// skip the data. Returns boolean indicating if the parsing was successful.
// parse 解析存储在数据中的TCP头，填充tcp段的序列和确认号，标志和窗口字段。
// 返回 bool值，指示解析是否成功。
func (s *segment) parse() bool {
	h := header.TCP(s.data.First())

	// h is the header followed by the payload. We check that the offset to
	// the data respects the following constraints:
	// 1. That it's at least the minimum header size; if we don't do this
	//    then part of the header would be delivered to user.
	// 2. That the header fits within the buffer; if we don't do this, we
	//    would panic when we tried to access data beyond the buffer.
	//
	// N.B. The segment has already been validated as having at least the
	//      minimum TCP size before reaching here, so it's safe to read the
	//      fields.
	offset := int(h.DataOffset())
	if offset < header.TCPMinimumSize || offset > len(h) {
		return false
	}

	s.options = []byte(h[header.TCPMinimumSize:offset])
	s.parsedOptions = header.ParseTCPOptions(s.options)
	s.data.TrimFront(offset)

	s.sequenceNumber = seqnum.Value(h.SequenceNumber())
	s.ackNumber = seqnum.Value(h.AckNumber())
	s.flags = h.Flags()
	s.window = seqnum.Size(h.WindowSize())

	return true
}
