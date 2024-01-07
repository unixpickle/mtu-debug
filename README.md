# mtu-debug

This is intended to be a tool to debug why path MTU discovery isn't working properly. It consists of a client and a server, where the client can send a payload and request a payload of a certain size back from the server. The server and client will both listen for ICMP "packet too big" responses and log this information.
