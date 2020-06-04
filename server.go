package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	//"./syncmap"
)

var portsDict = map[string]int{
    //"vision":10001,
    "sensor":10002,
    "cmd":10003,
    "debug":10004,
    "clock":10005,
    "message":10006,
}

var portsDictTCP = map[string]int{
	"vision":10001,
}

type Message struct {
  Mtype string
  Pri int
  Id string
  Data string
}

var clientUDPIPs, clientUDPPorts, clientUDPSockets sync.Map
var clientTCPIPs, clientTCPPorts, clientTCPSockets sync.Map

var done = make(chan struct{})

func read(socket *net.UDPConn, key string) {
	for {
		data := make([]byte, 65535)
		n, remoteAddr, err := socket.ReadFromUDP(data)
		checkErr(err)
		ip, _port, err := net.SplitHostPort(remoteAddr.String())
		port,err := strconv.Atoi(_port)
		checkErr(err)
		var message Message
		if err := json.Unmarshal(data[:n], &message); err != nil {
			go relayUDPRaw(data, n, port, key)
			//fmt.Println(err)
			continue
		}
		if message.Mtype == "register" {
			fmt.Printf("Register UDP <%s:%s> %s\n", key, message.Id, message.Data)
			clientUDPIPs.Store(message.Id, ip)
			clientUDPPorts.Store(message.Id+":"+key, port)
			feedback := Message{
				Mtype: "register",
				Pri: 5,
				Id: "000000",
				Data: remoteAddr.String(),
			}
			feedbackStr, err := json.Marshal(feedback)
			checkErr(err)
			socket.WriteToUDP(feedbackStr, remoteAddr)
		} else {
			go relay(data, n, message.Id, key)
		}
	}
}

func relay(data []byte, n int, myID string, key string) {
	clientUDPIPs.Range(func(robotID,ip interface{})bool{
		if robotID.(string) == myID {
			return true
		}
		port, ok := clientUDPPorts.Load(robotID.(string)+":"+key)
		if !ok{
			fmt.Println("No UDP port in key", robotID.(string)+":"+key)
			return true
		}
		clientAddr := &net.UDPAddr{IP: net.ParseIP(ip.(string)), Port: port.(int)}
		socket, ok := clientUDPSockets.Load(key)
		if !ok {
			return true
		}
		socket.(*net.UDPConn).WriteToUDP(data[:n], clientAddr)
		return true
	})
}

func relayUDPRaw(data []byte, n int, myPort int, key string) {
	clientUDPIPs.Range(func(robotId,ip interface{})bool{
		_port, ok := clientUDPPorts.Load(robotId.(string)+":"+key)
		if !ok {
			fmt.Println("No key in UDP port", robotId.(string)+":"+key)
			return true
		}
		port := _port.(int)
		if port == myPort {
			return true
		}
		clientAddr := &net.UDPAddr{IP: net.ParseIP(ip.(string)), Port: port}
		socket, ok := clientUDPSockets.Load(key)
		if !ok {
			return true
		}
		socket.(*net.UDPConn).WriteToUDP(data[:n], clientAddr)
		return true
	})
}

func checkErr(err error){
	if err != nil {
		fmt.Println(err)
	}
}

func relayTCP(data []byte, n int, robotID string, key string) {
	clientTCPIPs.Range(func(otherID,value interface{})bool{
		if otherID == robotID {
			return true
		}
		conn, ok := clientTCPSockets.Load(otherID.(string)+":"+key)
		if !ok {
			return true
		}
		conn.(net.Conn).Write(data[:n])
		return true
	})
}

func relayTCPRaw(data []byte, n int, remoteAddr net.Addr, key string) {
	clientTCPIPs.Range(func(robotID,value interface{})bool{
		if _, ok := clientTCPPorts.Load(robotID.(string)+":"+key); ok {
			port, ok := clientTCPPorts.Load(robotID.(string)+":"+key)
			if !ok {
				fmt.Println("No key", robotID.(string)+":"+key, " in clientTCPPorts")
				return true
			}
			address := value.(string)+":"+strconv.Itoa(port.(int))
			if address == remoteAddr.String() {
				return true
			}
			conn, ok := clientTCPSockets.Load(address)
			if !ok {
				return true
			}
			conn.(net.Conn).Write(data[:n])
		}
		return true
	})
}

func handleTCP(conn net.Conn, ip string, port int, remoteAddr net.Addr, key string){
	defer conn.Close()
	for {
		data := make([]byte, 65535*5)
		n, err := conn.Read(data)
		if err == io.EOF || n == 0{
			conn.Close()
			conn = nil
			break
		}
		var message Message
		if err := json.Unmarshal(data[:n], &message); err != nil {
			go relayTCPRaw(data, n, remoteAddr, key)
		}
		checkErr(err)
		if message.Mtype == "register" {
			//fmt.Printf("Register <%s:%s> %s\n", key, message.Id, message.Data)
			clientTCPIPs.Store(message.Id, ip)
			clientTCPPorts.Store(message.Id+":"+key, port)
			feedback := Message{
				Mtype: "register",
				Pri:   5,
				Id:    "000000",
				Data:  remoteAddr.String(),
			}
			feedbackStr, err := json.Marshal(feedback)
			checkErr(err)
			_, err = conn.Write(feedbackStr)
			checkErr(err)
			clientTCPSockets.Store(remoteAddr.String(), conn)
		} else {
			go relayTCP(data, n, message.Id, key)
		}
	}
	fmt.Printf("Close TCP connection <%s:%d> as %s\n", ip, port, key)
}

func readTCP(socket *net.TCPListener, key string){
	for {
		conn, err := socket.Accept()
		fmt.Printf("Register TCP <%s> %s\n",conn.RemoteAddr().String(), key)
		checkErr(err)
		remoteAddr := conn.RemoteAddr()
		ip, _port, err := net.SplitHostPort(remoteAddr.String())
		port,err := strconv.Atoi(_port)
		checkErr(err)
		go handleTCP(conn, ip, port, remoteAddr, key)
	}
}

func main() {
	for key, port := range portsDict {
		clientAddr := &net.UDPAddr{IP: net.IPv4zero, Port: port}
		clientListener, err := net.ListenUDP("udp", clientAddr)
		checkErr(err)
		clientUDPSockets.Store(key, clientListener)
		go read(clientListener, key)
	}

	for key, port := range portsDictTCP {
		clientAddr, err := net.ResolveTCPAddr("tcp4", ":"+strconv.Itoa(port))
		checkErr(err)
		clientListener, err := net.ListenTCP("tcp", clientAddr)
		checkErr(err)
		go readTCP(clientListener, key)
	}
	<-done
}