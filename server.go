package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"text/template"
	"time"
)

var portsDict = map[string]int{
    "sensor":10002,
    "cmd":10003,
    "debug":10004,
    "clock":10005,
    "message":10006,
}

var portsDictTCP = map[string]int{
	"vision":10001,
}
// Comunication protocol
type Message struct {
  Mtype string
  Pri int
  Id string
  Data string
}
// Thread-safe maps
var clientUDPIPs, clientUDPPorts, clientUDPSockets sync.Map
var clientTCPIPs, clientTCPPorts, clientTCPSockets sync.Map
var UDPRecvCnt, TCPRecvCnt, UDPRelayCnt, TCPRelayCnt sync.Map
var UDPStateMap, TCPStateMap sync.Map
// Block the main function
var done = make(chan struct{})

// Web visualization information
type SocketState struct {
	Key string
	Type string
	IP string
	Port int
	RecvFPS int
	RelayFPS int
}

// index.html resource
var myTemplate *template.Template
var mutex sync.RWMutex
var socketStates []SocketState

func checkErr(err error){
	if err != nil {
		fmt.Println(err)
	}
}

func readUDP(socket *net.UDPConn, key string) {
	for {
		data := make([]byte, 65535)
		n, remoteAddr, err := socket.ReadFromUDP(data)
		checkErr(err)
		ip, _port, err := net.SplitHostPort(remoteAddr.String())
		port,err := strconv.Atoi(_port)
		checkErr(err)
		var message Message
		err = json.Unmarshal(data[:n], &message)
		if err != nil || message.Id == ""{
			go relayUDPRaw(data, n, ip, port, key)
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

			SocketState := SocketState{Key:key, Type:"UDP", IP: ip, Port: port, RecvFPS:0, RelayFPS:0}
			UDPStateMap.Store(message.Id+":"+key, SocketState)
			UDPRecvCnt.Store(message.Id+":"+key, 0)
			UDPRelayCnt.Store(message.Id+":"+key, 0)
			socket.WriteToUDP(feedbackStr, remoteAddr)
		} else {
			go relayUDP(data, n, message.Id, key)
		}
	}
}

func relayUDP(data []byte, n int, myID string, key string) {
	cnt, _ := UDPRecvCnt.Load(myID+":"+key)
	UDPRecvCnt.Store(myID+":"+key, cnt.(int)+1)
	clientUDPIPs.Range(func(robotID, ip interface{})bool{
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
		cnt, _ := UDPRelayCnt.Load(myID+":"+key)
		UDPRelayCnt.Store(myID+":"+key, cnt.(int)+1)
		return true
	})
}

func relayUDPRaw(data []byte, n int, myIP string, myPort int, key string) {
	var myID string
	// find my ID
	clientUDPIPs.Range(func(robotId,ip interface{})bool{
		// get port
		_port, ok := clientUDPPorts.Load(robotId.(string)+":"+key)
		if !ok {
			fmt.Println("No key in UDP port", robotId.(string)+":"+key)
			return true
		}
		port := _port.(int)
		// running on the same machine may have the same IP address
		// running on different machines may have the same port
		if myIP == ip.(string) && myPort == port{
			myID = robotId.(string)
		}
		return true
	})
	// recv cnt ++
	cnt, _ := UDPRecvCnt.Load(myID+":"+key)
	UDPRecvCnt.Store(myID+":"+key, cnt.(int)+1)

	clientUDPIPs.Range(func(robotId,ip interface{})bool{
		// get port
		_port, ok := clientUDPPorts.Load(robotId.(string)+":"+key)
		if !ok {
			fmt.Println("No key in UDP port", robotId.(string)+":"+key)
			return true
		}
		port := _port.(int)
		if myIP == ip.(string) && port == myPort {
			return true
		}
		clientAddr := &net.UDPAddr{IP: net.ParseIP(ip.(string)), Port: port}
		socket, ok := clientUDPSockets.Load(key)
		if !ok {
			return true
		}
		socket.(*net.UDPConn).WriteToUDP(data[:n], clientAddr)
		// relay cnt ++
		cnt, _ := UDPRelayCnt.Load(myID+":"+key)
		UDPRelayCnt.Store(myID+":"+key, cnt.(int)+1)
		return true
	})
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

			SocketState := SocketState{Key:key, Type:"TCP", IP: ip, Port: port, RecvFPS:0, RelayFPS:0}
			TCPStateMap.Store(remoteAddr.String(), SocketState)
			clientTCPSockets.Store(remoteAddr.String(), conn)
			TCPRecvCnt.Store(remoteAddr.String(), 0)
			TCPRelayCnt.Store(remoteAddr.String(), 0)
		} else {
			go relayTCP(data, n, message.Id, key, ip, port)
		}
	}
	fmt.Printf("Close TCP connection <%s:%d> as %s\n", ip, port, key)
}

func relayTCP(data []byte, n int, robotID string, key string, ip string, port int) {
	cnt, _ := TCPRecvCnt.Load(ip+":"+strconv.Itoa(port))
	TCPRecvCnt.Store(ip+":"+strconv.Itoa(port), cnt.(int)+1)
	clientTCPIPs.Range(func(otherID,value interface{})bool{
		if otherID == robotID {
			return true
		}
		conn, ok := clientTCPSockets.Load(otherID.(string)+":"+key)
		if !ok {
			return true
		}
		conn.(net.Conn).Write(data[:n])
		cnt, _ := TCPRelayCnt.Load(ip+":"+strconv.Itoa(port))
		TCPRelayCnt.Store(ip+":"+strconv.Itoa(port), cnt.(int)+1)
		return true
	})
}

func relayTCPRaw(data []byte, n int, remoteAddr net.Addr, key string) {
	cnt, _ := TCPRecvCnt.Load(remoteAddr.String())
	TCPRecvCnt.Store(remoteAddr.String(), cnt.(int)+1)
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
			cnt, _ := TCPRelayCnt.Load(remoteAddr.String())
			TCPRelayCnt.Store(remoteAddr.String(), cnt.(int)+1)
		}
		return true
	})
}

func FPSCounter() {
	duration := time.Duration(time.Second)
	t := time.NewTicker(duration)
	defer t.Stop()
	for {
		<- t.C
		var states []SocketState
		clientUDPPorts.Range(func(key,socket interface{})bool{
			recvCnt, ok := UDPRecvCnt.Load(key.(string))
			if !ok {
				fmt.Println("No key in UDP socket", key.(string))
				return true
			}
			relayCnt, ok := UDPRelayCnt.Load(key.(string))
			if !ok {
				fmt.Println("No key in UDP socket", key.(string))
				return true
			}
			_state, ok := UDPStateMap.Load(key.(string))
			state := _state.(SocketState)
			state.RecvFPS = recvCnt.(int)
			state.RelayFPS = relayCnt.(int)
			states = append(states, state)
			newState := SocketState{Key:state.Key, Type:"UDP", IP: state.IP, Port: state.Port, RecvFPS:0, RelayFPS:0}
			UDPStateMap.Store(key.(string), newState)
			UDPRecvCnt.Store(key.(string), 0)
			UDPRelayCnt.Store(key.(string), 0)
			return true
		})
		clientTCPSockets.Range(func(key,socket interface{})bool{
			recvCnt, ok := TCPRecvCnt.Load(key.(string))
			if !ok {
				fmt.Println("No key in TCP socket", key.(string))
				return true
			}
			relayCnt, ok := TCPRelayCnt.Load(key.(string))
			if !ok {
				fmt.Println("No key in TCP socket", key.(string))
				return true
			}
			_state, ok := TCPStateMap.Load(key.(string))
			state := _state.(SocketState)
			state.RecvFPS = recvCnt.(int)
			state.RelayFPS = relayCnt.(int)
			states = append(states, state)
			newState := SocketState{Key:state.Key, Type:"TCP", IP: state.IP, Port: state.Port, RecvFPS:0, RelayFPS:0}
			TCPStateMap.Store(key.(string), newState)
			TCPRecvCnt.Store(key.(string), 0)
			TCPRelayCnt.Store(key.(string), 0)
			return true
		})
		mutex.Lock()
		socketStates = states
		mutex.Unlock()
	}
}

func initTemplate(fileName string) (err error) {
	myTemplate, err = template.ParseFiles(fileName)
	checkErr(err)
	return err
}

func webHandler(writer http.ResponseWriter, request *http.Request) {
	data := make(map[string]interface{})
	data["title"] = "Data Relay"
	mutex.RLock()
	data["states"] = socketStates
	mutex.RUnlock()
	myTemplate.Execute(writer, data)
}

func main() {
	for key, port := range portsDict {
		clientAddr := &net.UDPAddr{IP: net.IPv4zero, Port: port}
		clientListener, err := net.ListenUDP("udp", clientAddr)
		checkErr(err)
		clientUDPSockets.Store(key, clientListener)
		go readUDP(clientListener, key)
	}

	for key, port := range portsDictTCP {
		clientAddr, err := net.ResolveTCPAddr("tcp4", ":"+strconv.Itoa(port))
		checkErr(err)
		clientListener, err := net.ListenTCP("tcp", clientAddr)
		checkErr(err)
		go readTCP(clientListener, key)
	}
	go FPSCounter()
	initTemplate("./index.html")
	http.HandleFunc("/", webHandler)
	err := http.ListenAndServe("0.0.0.0:8080", nil)
	checkErr(err)
	<-done
}