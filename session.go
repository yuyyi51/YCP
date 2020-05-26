package YCP

import (
	"bytes"
	"code.int-2.me/yuyyi51/YCP/internal"
	"code.int-2.me/yuyyi51/YCP/packet"
	"container/list"
	"fmt"
	"net"
	"sync"
	"time"
)

type Session struct {
	conv uint32

	conn            net.PacketConn
	remoteAddr      net.Addr
	receivedPackets chan *packet.Packet
	sessMux         *sync.RWMutex

	writeBuffer    []byte
	writeBufferLen int
	writingMux     *sync.RWMutex
	writeSignal    chan struct{}

	dataBuffer []byte
	readingMux *sync.RWMutex
	readSignal chan struct{}

	dataManager *internal.DataManager
	ackManager  *ackManager
	closeSignal chan struct{}
	dataSignal  chan struct{}

	congestion CongestionAlgorithm

	bytesInFlight  uint64
	nextPktSeq     uint64
	nextDataOffset uint64
}

func (sess *Session) Read(b []byte) (n int, err error) {
	sess.readingMux.Lock()
	defer sess.readingMux.Unlock()
	remain := len(b)
	offset := 0
	if len(sess.dataBuffer) != 0 {
		if len(b) >= len(sess.dataBuffer) {
			copy(b, sess.dataBuffer)
			remain -= len(sess.dataBuffer)
			offset += len(sess.dataBuffer)
			sess.dataBuffer = nil
		} else {
			copy(b, sess.dataBuffer[:len(b)])
			sess.dataBuffer = sess.dataBuffer[len(b):]
			return len(b), nil
		}
	}
	if remain == 0 {
		return len(b), nil
	}
	for {
		data := sess.dataManager.PopData()
		//fmt.Printf("pop data, len %d\n%s\n", len(data), hex.EncodeToString(data))
		if remain >= len(data) {
			copy(b[offset:], data)
			offset += len(data)
		} else {
			copy(b[offset:], data[:remain])
			sess.dataBuffer = data[remain:]
			offset += remain
			break
		}
		if offset == 0 {
			// no data
			select {
			case <-sess.readSignal:
			}
		} else {
			break
		}
	}
	return offset, nil
}

func (sess *Session) Write(b []byte) (n int, err error) {
	//fmt.Printf("start Write, len %d\n", len(b))
	sess.writingMux.Lock()
	defer sess.writingMux.Unlock()

	sess.sessMux.Lock()
	if len(b)+sess.writeBufferLen <= len(sess.writeBuffer) {
		// enough space in buffer
		copy(sess.writeBuffer[sess.writeBufferLen:], b)
		sess.writeBufferLen += len(b)
		sess.SignalData()
		sess.sessMux.Unlock()
		//fmt.Printf("finish Write\n")
		return len(b), nil
	}
	n = 0
	for {
		canCopy := len(sess.writeBuffer) - sess.writeBufferLen
		if canCopy > len(b)-n {
			canCopy = len(b) - n
		}
		if canCopy > 0 {
			copy(sess.writeBuffer[sess.writeBufferLen:], b[n:n+canCopy])
			n += canCopy
			//sess.SignalData()
		}
		if n >= len(b) {
			break
		}
		sess.sessMux.Unlock()
		select {
		case <-sess.writeSignal:

		case <-sess.closeSignal:
			err = fmt.Errorf("connection closed")
			break
		}
		sess.sessMux.Lock()
	}
	sess.sessMux.Unlock()
	//fmt.Printf("finish Write\n")
	return
}

func (sess *Session) Close() error {
	return nil
}

func (sess *Session) LocalAddr() net.Addr {
	return sess.conn.LocalAddr()
}

func (sess *Session) RemoteAddr() net.Addr {
	return sess.remoteAddr
}

func (sess *Session) SetDeadline(t time.Time) error {
	return nil
}

func (sess *Session) SetReadDeadline(t time.Time) error {
	return nil
}

func (sess *Session) SetWriteDeadline(t time.Time) error {
	return nil
}

func (sess *Session) String() string {
	return fmt.Sprintf("[%d]%s,%s", sess.conv, sess.LocalAddr(), sess.RemoteAddr())
}

func NewSession(conn net.PacketConn, addr net.Addr, conv uint32) *Session {
	session := &Session{
		conv:            conv,
		conn:            conn,
		remoteAddr:      addr,
		receivedPackets: make(chan *packet.Packet, 500),
		dataManager:     internal.NewDataManager(),
		ackManager:      newAckManager(),
		sessMux:         new(sync.RWMutex),
		writeBuffer:     make([]byte, packet.SessionWriteBufferSize),
		writingMux:      new(sync.RWMutex),
		writeSignal:     make(chan struct{}),
		closeSignal:     make(chan struct{}),
		dataSignal:      make(chan struct{}),
		readSignal:      make(chan struct{}),
		congestion:      NewRenoAlgorithm(),
		readingMux:      new(sync.RWMutex),
	}
	go session.run()
	return session
}

func (sess *Session) run() {
	sendTimer := time.NewTimer(packet.SessionSendInterval)
	for {
		select {
		case pkt := <-sess.receivedPackets:
			sess.handlePacket(pkt)
		//case <-sess.dataSignal:
		//	//fmt.Printf("dataSignal fired\n")
		//	sess.sendData()
		//	sendTimer.Reset(packet.SessionSendInterval)
		case <-sendTimer.C:
			//fmt.Printf("sendTimer fired\n")
			sess.sendData()
			sendTimer.Reset(packet.SessionSendInterval)
		case <-sess.closeSignal:
		}
	}
}

func (sess *Session) sendData() {
	// 能发送多少数据
	cwd := sess.congestion.GetCongestionWindow()
	if cwd <= sess.bytesInFlight {
		// don't send
		return
	}
	canSend := cwd - sess.bytesInFlight
	if !sess.haveDataToSend() {
		//fmt.Println("sendData exit for have no data")
		return
	}
	packets := make([]PacketInfo, 0)
	for canSend > 0 && sess.haveDataToSend() {
		size := uint64(packet.MaxDataFrameDataSize)
		if canSend < size {
			size = canSend
		}
		dataFrame, remain := sess.popDataFrame(int(size), sess.nextDataOffset)
		sess.nextDataOffset += uint64(len(dataFrame.Data))
		pkt := packet.NewPacket(sess.conv, sess.nextPktSeq, 0)
		pkt.AddFrame(dataFrame)
		sess.nextPktSeq++
		pktInfo := PacketInfo{
			seq:  pkt.Seq,
			size: pkt.Size(),
		}
		packets = append(packets, pktInfo)
		err := sess.sendPacket(pkt)
		if err != nil {
			fmt.Printf("send packet error: %v", err)
		}
		canSend -= size
		if remain == 0 {
			//fmt.Println("sendData exit for remain no more data")
		}
	}
	sess.congestion.OnPacketsSend(packets)
	fmt.Printf("cwd: %d, inFlight: %d\n", cwd, sess.bytesInFlight)
}

func (sess *Session) handlePacket(packet *packet.Packet) {
	//fmt.Printf("handle pakcet %s\n", packet.String())
	sess.ackManager.addAckRange(packet.Seq, packet.Seq)
	//fmt.Println(sess.ackManager.printAckRanges())
	for _, frame := range packet.Frames {
		sess.handleFrame(frame)
	}
}

func (sess *Session) handleFrame(frame packet.Frame) error {
	switch frame.Command() {
	case packet.DataFrameCommand:
		dataFrame, ok := frame.(packet.DataFrame)
		if !ok {
			return fmt.Errorf("convert Frame to DataFrame eror")
		}
		if len(dataFrame.Data) == 0 {
			fmt.Printf("droped dataFrame for have no data\n")
			break
		}
		sess.dataManager.AddDataRange(dataFrame.Offset, dataFrame.Offset+uint64(len(dataFrame.Data))-1, dataFrame.Data)
		sess.SignalRead()
		//fmt.Printf("printint data ranges : %s\n", sess.dataManager.PrintDataRanges())
	}
	return nil
}

func (sess *Session) SignalWrite() {
	select {
	case sess.writeSignal <- struct{}{}:
	default:
	}
}

func (sess *Session) SignalData() {
	select {
	case sess.dataSignal <- struct{}{}:
	default:
	}
}

func (sess *Session) SignalRead() {
	select {
	case sess.readSignal <- struct{}{}:
	default:
	}
}

func (sess *Session) haveDataToSend() bool {
	sess.sessMux.Lock()
	defer sess.sessMux.Unlock()
	return sess.writeBufferLen > 0
}

func (sess *Session) popDataFrame(size int, offset uint64) (packet.DataFrame, int) {
	dataSize := size
	sess.sessMux.Lock()
	if dataSize > sess.writeBufferLen {
		dataSize = sess.writeBufferLen
	}
	data := make([]byte, dataSize)
	copy(data, sess.writeBuffer[0:dataSize])
	copy(sess.writeBuffer[0:], sess.writeBuffer[dataSize:])
	sess.writeBufferLen -= dataSize
	remain := sess.writeBufferLen
	sess.SignalWrite()
	sess.sessMux.Unlock()
	return packet.CreateDataFrame(data, offset), remain
}

func (sess *Session) sendPacket(p packet.Packet) error {
	fmt.Printf("%s send packet seq: %d, size: %d\n", sess, p.Seq, p.Size())
	_, err := sess.conn.WriteTo(p.Pack(), sess.remoteAddr)
	if err == nil {
		sess.bytesInFlight += uint64(p.Size())
	}
	return err
}

type ackManager struct {
	rangeList *list.List
}

type ackRange struct {
	left  uint64
	right uint64
}

func (ran ackRange) String() string {
	return fmt.Sprintf("[%d,%d]", ran.left, ran.right)
}

func newAckManager() *ackManager {
	return &ackManager{
		rangeList: list.New(),
	}
}

func (manager *ackManager) printAckRanges() string {
	buffer := bytes.Buffer{}
	for cur := manager.rangeList.Front(); cur != nil; cur = cur.Next() {
		buffer.WriteString(fmt.Sprintf("%s ", cur.Value.(ackRange)))
	}
	return buffer.String()
}

func (manager *ackManager) addAckRange(left, right uint64) {
	if manager.rangeList.Back() == nil {
		ran := ackRange{
			left:  left,
			right: right,
		}
		manager.rangeList.PushBack(ran)
		return
	}
	head := manager.rangeList.Front()
	currentRange := ackRange{
		left:  left,
		right: right,
	}
	startMerge := false
	cur := head
	ccur := head
	for ; cur != nil; cur = ccur {
		ccur = cur.Next()
		if !canMerge(cur.Value.(ackRange), currentRange) {
			if !startMerge {
				continue
			} else {
				manager.rangeList.InsertBefore(currentRange, cur)
				//fmt.Printf("insert range %s\n", currentRange)
				break
			}
		}
		currentRange = merge(currentRange, cur.Value.(ackRange))
		manager.rangeList.Remove(cur)
		//fmt.Printf("removed range %s\n", cur.Value.(ackRange))
		startMerge = true
	}
	if cur == nil {
		manager.rangeList.PushBack(currentRange)
	}
}

func canMerge(range1, range2 ackRange) bool {
	if range1.left < range2.left-1 && range1.right < range2.left-1 && range2.left != 0 {
		// range1在range2的左侧
		return false
	} else if range2.left < range1.left-1 && range2.right < range1.left-1 && range1.left != 0 {
		// range2在range1的左侧
		return false
	}
	return true
}

func merge(range1, range2 ackRange) ackRange {
	merged := ackRange{}
	merged.left = min(range1.left, range2.left)
	merged.right = max(range1.right, range2.right)
	return merged
}

func max(a, b uint64) uint64 {
	if a > b {
		return a
	} else {
		return b
	}
}

func min(a, b uint64) uint64 {
	if a < b {
		return a
	} else {
		return b
	}
}
