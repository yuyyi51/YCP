package main

import (
	"bufio"
	"code.int-2.me/yuyyi51/YCP"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

func main() {
	Entrance2()
}
func server(host string, port int, wq *sync.WaitGroup) {
	address := fmt.Sprintf("%s:%d", host, port)
	server := YCP.NewServer(address)
	err := server.Listen()
	if err != nil {
		fmt.Printf("%v\n", err)
	}
	session := server.Accept()
	/*for i := 0; i < 200; i++ {
		fmt.Println(i)
		data := make([]byte, 0)
		for j := 0; j < 100; j++ {
			data = append(data, byte(j))
		}
		_, _ = session.Write(data)
	}*/
	_ = session
	for {
		buffer := make([]byte, 1000)
		n, _ := session.Read(buffer)
		if n != 0 {
			fmt.Println("reading data")
			fmt.Println(hex.EncodeToString(buffer[:n]))
		}
		time.Sleep(time.Second * 1)
	}

	wq.Wait()
}
func CreateUdpConn(remoteHost string, remotePort int) (conn net.Conn, err error) {
	remoteAddress := fmt.Sprintf("%s:%d", remoteHost, remotePort)
	conn, err = net.Dial("udp", remoteAddress)
	return
}

func CreateUdpListener(host string, port int) (listener net.PacketConn, err error) {
	address := fmt.Sprintf("%s:%d", host, port)
	listener, err = net.ListenPacket("udp", address)
	return
}

func client(host string, port int) {
	session, err := YCP.Dial(host, port)
	if err != nil {
		fmt.Printf("client dial error: %v", err)
		os.Exit(1)
	}
	for i := 0; i < 200; i++ {
		//fmt.Println(i)
		data := make([]byte, 0)
		for j := 0; j < 100; j++ {
			data = append(data, byte(j))
		}
		_, _ = session.Write(data)
	}
	/*
		conn, err := CreateUdpConn(host, port)
		if err != nil {
			fmt.Printf("client createUdpConn error: %v", err)
			os.Exit(1)
		}
		rand.Seed(time.Now().Unix())
		for i := 1; i <= 100; i++ {
			if rand.Int()%100 < 0 {
				fmt.Printf("lost packet %d\n", i)
				continue
			}
			p := packet.Packet{
				Conv: 1234,
				Seq:  uint64(i),
				Rcv:  100,
			}
			data := make([]byte, 100)
			dataFrame1 := packet.CreateDataFrame(data, 12)
			frames := make([]packet.Frame, 0)
			data2 := make([]byte, i)
			dataFrame2 := packet.CreateDataFrame(data2, 23)
			frames = append(frames, dataFrame1)
			frames = append(frames, dataFrame2)
			p.Frames = frames

			_, err := conn.Write(p.Pack())
			if err != nil {
				fmt.Printf("client write error: %v", err)
				break
			}
			//fmt.Printf("client write Packet length [%d]\n", n)
			time.Sleep(time.Millisecond * 20)
		}
		fmt.Println("client exit")
	*/
}

func Entrance2() {
	fmt.Println("input mode(0:server|1:client), host and port")
	s := bufio.NewScanner(os.Stdin)
	s.Scan()
	mode := s.Text()
	//s.Scan()
	//host := s.Text()
	//s.Scan()
	//port := s.Text()
	host := "127.0.0.1"
	port := "7654"
	portn, _ := strconv.ParseInt(port, 10, 32)
	if mode == "0" {
		address := fmt.Sprintf("%s:%s", host, port)
		server := YCP.NewServer(address)
		err := server.Listen()
		if err != nil {
			fmt.Printf("%v\n", err)
		}
		session := server.Accept()
		go func() {
			for {
				buffer := make([]byte, 1000)
				fmt.Printf("reading data\n")
				n, _ := session.Read(buffer)
				if n != 0 {
					fmt.Printf("%s\n", buffer)
				}
			}
		}()
		for {
			s.Scan()
			message := s.Text()
			fmt.Printf("get input\n")
			_, _ = session.Write([]byte(message))
		}
	} else {
		session, err := YCP.Dial(host, int(portn))
		if err != nil {
			fmt.Printf("client dial error: %v", err)
			os.Exit(1)
		}
		go func() {
			for {
				buffer := make([]byte, 1000)
				fmt.Printf("reading data\n")
				n, _ := session.Read(buffer)
				if n != 0 {
					fmt.Printf("%s\n", buffer)
				}
			}
		}()
		for {
			s.Scan()
			message := s.Text()
			fmt.Printf("get input\n")
			_, _ = session.Write([]byte(message))
		}
	}
}

func Entrance() {
	wq := &sync.WaitGroup{}
	func() {
		wq.Add(1)
		go server("127.0.0.1", 8796, wq)
	}()
	go client("127.0.0.1", 8796)
	wq.Wait()
}
