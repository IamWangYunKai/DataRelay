import socket
from threading import Thread

register_ports = {
        'vision':10001,
        'sensor':10002,
        'cmd':10003,
        'debug':10004,
        'clock':10005,
        }

server_ip = None
client_ip = None
server_ports = {}
client_ports = {}

server_sockets = {}
client_sockets = {}
    
def server_setup(UDPSock, key):
    global server_ip,server_ports,server_sockets
    while True:
        data, addr = UDPSock.recvfrom(65535)
        data = str(data, encoding = "utf-8")
        try:
            ip_type, attr = data.split(":")
            ip = str(addr[0])
            port = int(addr[1])
        except:
            print('Data Error', data)
            continue
        if ip_type == 'server':
	        server_ip = ip
	        server_ports[attr] = port
	        print(addr , 'is connected as a server', attr)
        else:
	        print('Error data:', data, 'from', addr)

        if key in server_ports:
	        print('Ready to get server', key, 'data')
	        feed_bakc_data = server_ip + ":" + str(server_ports[key])
	        UDPSock.sendto(feed_bakc_data.encode("utf-8"), (server_ip, server_ports[key]))
	        break

    server_sockets[key] = UDPSock
    thread = Thread(target = server_transfer, args = (UDPSock,key,))
    thread.start()

def client_setup(UDPSock, key):
    global client_ip,client_ports,client_sockets
    while True:
        data, addr = UDPSock.recvfrom(65535)
        data = str(data, encoding = "utf-8")
        try:
            ip_type, attr = data.split(":")
            ip = str(addr[0])
            port = int(addr[1])
        except:
            print('Data Error', data)
            continue
        if ip_type == 'client':
	        client_ip = ip
	        client_ports[attr] = port
	        print(addr , 'is connected as a client', attr)
        else:
	        print('Error data:', data, 'from', addr)

        if key in client_ports:
	        print('Ready to send', key, 'data to client')
	        feed_bakc_data = client_ip + ":" + str(client_ports[key])
	        UDPSock.sendto(feed_bakc_data.encode("utf-8"), (client_ip, client_ports[key]))
	        break

    client_sockets[key] = UDPSock
    thread = Thread(target = client_transfer, args = (UDPSock,key,))
    thread.start()


def server_transfer(UDPSock, key):
    global server_ip,client_ip,client_ports,client_sockets
    while True:
        data, addr = UDPSock.recvfrom(65535)
        if addr[0] == server_ip:
            if key in client_sockets:
                client_sockets[key].sendto(data, (client_ip, client_ports[key]))
        else:
	        print('Error IP:', addr)
            
def client_transfer(UDPSock, key):
    global server_ip,client_ip,server_ports,server_sockets
    while True:
        data, addr = UDPSock.recvfrom(65535)
        if addr[0] == client_ip:
            if key in server_sockets:
                server_sockets[key].sendto(data, (server_ip, server_ports[key]))
        else:
	        print('Error IP:', addr)
            
            
for key in list(register_ports.keys()):
    server_socket = socket.socket(socket.AF_INET,socket.SOCK_DGRAM)
    listen_addr = ("", register_ports[key])
    server_socket.bind(listen_addr)
    server_thread = Thread(target = server_setup, args = (server_socket,key,))
    server_thread.start()
    
    client_socket = socket.socket(socket.AF_INET,socket.SOCK_DGRAM)
    listen_addr = ("", register_ports[key]+10000)
    client_socket.bind(listen_addr)
    client_thread = Thread(target = client_setup, args = (client_socket,key,))
    client_thread.start()