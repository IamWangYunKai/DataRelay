package main
import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
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

var clientIPs = make(map[string]string)
var clientPorts = make(map[string]int)
var clientSockets = make(map[string]*net.UDPConn)

var clientTCPIPs = make(map[string]string)
var clientTCPPorts = make(map[string]int)
var clientTCPSockets = make(map[string]*net.Conn)

var done = make(chan struct{})

func read(socket *net.UDPConn, key string) {
	for {
		data := make([]byte, 65535)
		n, remoteAddr, err := socket.ReadFromUDP(data)
		checkErr(err)
		// fmt.Printf("receive %s from <%s>\n", data[:n], remoteAddr)
		ip, _port, err := net.SplitHostPort(remoteAddr.String())
		port,err := strconv.Atoi(_port)
		checkErr(err)
		var message Message
		if err := json.Unmarshal(data[:n], &message); err != nil {
			go relayNoId(data, n, port, key)
			//fmt.Println(err)
			continue
		}
		if message.Mtype == "register" {
			//fmt.Printf("Register <%s:%s> %s\n", key, message.Id, message.Data)
			clientIPs[message.Id] = ip
			clientPorts[message.Id+":"+key] = port
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

func relay(data []byte, n int, robotID string, key string) {
	for otherID, otherIP := range clientIPs{
		if otherID == robotID {
			continue
		}
		if _, ok := clientPorts[otherID+":"+key]; ok {
			clientAddr := &net.UDPAddr{IP: net.ParseIP(otherIP), Port: clientPorts[otherID+":"+key]}
			clientSockets[key].WriteToUDP(data[:n], clientAddr)
		}
	}
}

func relayNoId(data []byte, n int, port int, key string) {
	for otherID, otherIP := range clientIPs{
		if _, ok := clientPorts[otherID+":"+key]; ok {
			if clientPorts[otherID+":"+key] == port {
				continue
			}
			clientAddr := &net.UDPAddr{IP: net.ParseIP(otherIP), Port: clientPorts[otherID+":"+key]}
			clientSockets[key].WriteToUDP(data[:n], clientAddr)
		}
	}
}

func checkErr(err error){
	if err != nil {
		fmt.Println(err)
	}
}

func relayTCP(data []byte, n int, robotID string, key string) {
	for otherID, _ := range clientTCPIPs{
		if otherID == robotID {
			continue
		}
		if _, ok := clientTCPPorts[otherID+":"+key]; ok {
			(*clientTCPSockets[key]).Write(data[:n])
		}
	}
}

func relayTCPRaw(data []byte, n int, port int, key string) {
	fmt.Println("relayTCPRaw", n, port, key)
	for otherID, _ := range clientTCPIPs{
		if _, ok := clientTCPPorts[otherID+":"+key]; ok {
			if clientTCPPorts[otherID+":"+key] == port {
				continue
			}
			if _, ok := clientTCPPorts[otherID+":"+key]; ok {
				(*clientTCPSockets[key]).Write(data[:n])
				fmt.Println("write to", otherID, key)
			}
		}
	}
}

func readTCP(socket *net.TCPListener, key string){
	for {
		conn, err := socket.Accept()
		fmt.Println("accept tcp client",conn.RemoteAddr().String())
		checkErr(err)
		clientTCPSockets[key] = &conn
		remoteAddr := conn.RemoteAddr()
		ip, _port, err := net.SplitHostPort(remoteAddr.String())
		port,err := strconv.Atoi(_port)
		checkErr(err)
		for {
			data := make([]byte, 65535*5)
			n, err := conn.Read(data)
			fmt.Println("Get data", n)
			var message Message
			if err := json.Unmarshal(data[:n], &message); err != nil {
				go relayTCPRaw(data, n, port, key)
			}
			checkErr(err)
			if message.Mtype == "register" {
				fmt.Printf("Register <%s:%s> %s\n", key, message.Id, message.Data)
				clientIPs[message.Id] = ip
				clientPorts[message.Id+":"+key] = port
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
			} else {
				go relayTCP(data, n, message.Id, key)
			}
		}
	}
}

func main() {
	for key, port := range portsDict {
		clientAddr := &net.UDPAddr{IP: net.IPv4zero, Port: port}
		clientListener, err := net.ListenUDP("udp", clientAddr)
		checkErr(err)

		clientSockets[key] = clientListener
		go read(clientListener, key)
	}

	for key, port := range portsDictTCP {
		clientAddr, err := net.ResolveTCPAddr("tcp4", ":"+strconv.Itoa(port))
		checkErr(err)
		clientListener, err := net.ListenTCP("tcp", clientAddr)
		checkErr(err)
		fmt.Println("Start to read TCP")
		go readTCP(clientListener, key)
	}
	fmt.Println("Start to work ...")
	<-done
}
