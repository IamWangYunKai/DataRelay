package main
import (
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
}
var serverIP, clientIP string
var serverPorts = make(map[string]int)
var clientPorts = make(map[string]int)

var serverSockets = make(map[string]*net.UDPConn)
var clientSockets = make(map[string]*net.UDPConn)

var done = make(chan struct{})

func read(socket *net.UDPConn, key string) {
	for {
		data := make([]byte, 65535)
		n, remoteAddr, err := socket.ReadFromUDP(data)
		if err != nil {
			fmt.Printf("error during read: %s", err)
		}
		// fmt.Printf("receive %s from <%s>\n", data[:n], remoteAddr)
		ip, _port, err := net.SplitHostPort(remoteAddr.String())
		port,err := strconv.Atoi(_port)
		if err != nil {
			fmt.Println(err)
			return
		}
		ipType := string(data[:6])
		dataType := string(data[7:n])
		fmt.Printf("Register <%s:%s>\n", ipType, dataType)
		if ipType == "server" {
			serverIP = ip
			serverPorts[dataType] = port
			socket.WriteToUDP([]byte(remoteAddr.String()), remoteAddr)
			go relay(socket, key, true)
			return
		}else if ipType == "client" {
			clientIP = ip
			clientPorts[dataType] = port
			socket.WriteToUDP([]byte(remoteAddr.String()), remoteAddr)
			go relay(socket, key, false)
			return
		}else{
			fmt.Println("Error IP Type:", ipType)
			continue
		}
	}
}

func relay(socket *net.UDPConn, key string, isServer bool) {
	for {
		data := make([]byte, 65535)
		n, _, err := socket.ReadFromUDP(data)
		if err != nil {
			fmt.Printf("error during relay: %s", err)
		}
		if isServer{
			if _, ok := clientSockets[key]; ok {
				clientAddr := &net.UDPAddr{IP: net.ParseIP(clientIP), Port: clientPorts[key]}
				clientSockets[key].WriteToUDP(data[:n], clientAddr)
			}
		}else{
			if _, ok := serverSockets[key]; ok {
				serverAddr := &net.UDPAddr{IP: net.ParseIP(serverIP), Port: serverPorts[key]}
				serverSockets[key].WriteToUDP(data[:n], serverAddr)
			}
		}

	}
}

func main() {
	for key, port := range portsDict {
		serverAddr := &net.UDPAddr{IP: net.IPv4zero, Port: port}
		clientAddr := &net.UDPAddr{IP: net.IPv4zero, Port: port+10000}
		serverListener, err := net.ListenUDP("udp", serverAddr)
		if err != nil {
			fmt.Println(err)
			return
		}
		clientListener, err := net.ListenUDP("udp", clientAddr)
		if err != nil {
			fmt.Println(err)
			return
		}
		serverSockets[key] = serverListener
		clientSockets[key] = clientListener
		go read(serverListener, key)
		go read(clientListener, key)
	}
	fmt.Println("Start to work ...")
	<-done
}