package main
import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
)

var portsDict = map[string]int{
    "vision":10001,
    "sensor":10002,
    "cmd":10003,
    "debug":10004,
    "clock":10005,
    "message":10006,
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

var done = make(chan struct{})

func read(socket *net.UDPConn, key string) {
	for {
		data := make([]byte, 65535)
		n, remoteAddr, err := socket.ReadFromUDP(data)
		if err != nil {
			fmt.Printf("error during read: %s", err)
			continue
		}
		// fmt.Printf("receive %s from <%s>\n", data[:n], remoteAddr)
		ip, _port, err := net.SplitHostPort(remoteAddr.String())
		port,err := strconv.Atoi(_port)
		if err != nil {
			fmt.Println(err)
			continue
		}
		var message Message
		if err := json.Unmarshal(data[:n], &message); err != nil {
			go relayNoId(data, n, port, key)
			//fmt.Println(err)
			continue
		}
		if message.Mtype == "register" {
			fmt.Printf("Register <%s:%s> %s\n", key, message.Id, message.Data)
			clientIPs[message.Id] = ip
			clientPorts[message.Id+":"+key] = port
			feedback := Message{
				Mtype: "register",
				Pri: 5,
				Id: "000000",
				Data: remoteAddr.String(),
			}
			feedbackStr, err := json.Marshal(feedback)
			if err != nil {
				fmt.Println(err)
				continue
			}
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

func main() {
	for key, port := range portsDict {
		clientAddr := &net.UDPAddr{IP: net.IPv4zero, Port: port}
		clientListener, err := net.ListenUDP("udp", clientAddr)
		if err != nil {
			fmt.Println(err)
			continue
		}

		clientSockets[key] = clientListener
		go read(clientListener, key)
	}
	fmt.Println("Start to work ...")
	<-done
}