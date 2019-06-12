package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"os"
	"strings"
)

var remoteip string
var clientip string
var key string

func main() {
	key = "9"
	/*
		var str1 string

		str1 = "helloaf&93)+"

		ret1 := XorEncodeStr([]byte(str1), []byte(key))
		ret2 := XorDecodeStr(ret1, []byte(key))
		fmt.Printf("%s,%s", string(ret1), string(ret2))
	*/
	remoteip = "159.138.26.110:9001"
	//remoteip = "127.0.0.1:9001"
	clientip = "127.0.0.1:8081"

	if len(os.Args) < 2 {
		fmt.Printf("param is error!\n")
		return
	}

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	var l net.Listener
	var err error
	if os.Args[1] == "-s" {
		l, err = net.Listen("tcp", ":9001")
		if err != nil {
			log.Panic(err)
		}
	} else {
		l, err = net.Listen("tcp", ":8081")
		if err != nil {
			log.Panic(err)
		}
	}

	for {
		client, err := l.Accept()
		if err != nil {
			log.Panic(err)
		}

		if os.Args[1] == "-s" {
			go handleFromClientRequest(client)
		} else {

			go handleFromWebClientRequest(client)

		}

	}
}

func XorEncodeStr(msg, key []byte) []byte {
	//return msg
	ml := len(msg)
	//kl := len(key)

	var pwd []byte
	for i := 0; i < ml; i++ {
		//pwd = append(pwd, (key[i%kl])^(msg[i]))
		pwd = append(pwd, ((msg[i]) ^ 1))
	}

	return pwd
}

func XorDecodeStr(msg, key []byte) []byte {
	//return msg
	ml := len(msg)

	//kl := len(key)

	var pwd []byte
	for i := 0; i < ml; i++ {
		//pwd = append(pwd, (msg[i] ^ key[i%kl]))
		pwd = append(pwd, (msg[i] ^ 1))
	}

	return pwd

}

// copyBuffer is the actual implementation of Copy and CopyBuffer.
// if buf is nil, one is allocated.
func copyBuffer(dst io.Writer, src io.Reader, encodeType int) (written int64, err error) {

	buf := make([]byte, 4096)

	for {
		nr, er := src.Read(buf)
		if nr > 0 {

			var ret []byte
			if encodeType == 1 {
				fmt.Printf("<<read:Encode[[%s]]\n", string(buf[0:nr]))
				ret = XorEncodeStr(buf[0:nr], []byte(key))
			} else if encodeType == 2 {
				ret = XorDecodeStr(buf[0:nr], []byte(key))
				fmt.Printf("<<read:Decode[[%s]]\n", string(ret))
			} else {
				ret = buf
			}

			nw, ew := dst.Write(ret[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}
	return written, err
}

func handleFromWebClientRequest(client net.Conn) {
	if client == nil {
		return
	}

	defer client.Close()
	//获得了请求的host和port，就开始拨号吧
	server, err := net.Dial("tcp", remoteip)
	if err != nil {
		log.Println(err)
		return
	}

	//go Copy(server, client, 0)
	//Copy(client, server, 0)

	go copyBuffer(server, client, 1)
	copyBuffer(client, server, 2)
	//go io.Copy(server, client)
	//io.Copy(client, server)
}

func handleFromClientRequest(client net.Conn) {
	if client == nil {
		return
	}
	defer client.Close()

	var b [4096]byte
	n, err := client.Read(b[:])
	if err != nil {
		log.Println(err)
		return
	}

	bdcode := XorDecodeStr(b[:n], []byte(key))
	fmt.Printf("==>[[[%s]]]\n\n", string(bdcode))

	var method, host, address string
	fmt.Sscanf(string(bdcode[:bytes.IndexByte(bdcode[:], '\n')]), "%s%s", &method, &host)
	hostPortURL, err := url.Parse(host)
	if err != nil {
		log.Println(err)
		return
	}

	fmt.Printf("method:%s,host:%s\n", method, host)
	if hostPortURL.Opaque == "443" { //https访问
		address = hostPortURL.Scheme + ":443"
	} else { //http访问
		if strings.Index(hostPortURL.Host, ":") == -1 { //host不带端口， 默认80
			address = hostPortURL.Host + ":80"
		} else {
			address = hostPortURL.Host
		}
	}

	//获得了请求的host和port，就开始拨号吧
	server, err := net.Dial("tcp", address)
	if err != nil {
		log.Println(err)
		return
	}
	if method == "CONNECT" {

		ret := XorEncodeStr([]byte("HTTP/1.1 200 Connection established\r\n\r\n"), []byte(key))
		//fmt.Fprint(client, "HTTP/1.1 200 Connection established\r\n\r\n")
		fmt.Fprint(client, string(ret))
	} else {
		server.Write(bdcode[:n])
	}

	//进行转发
	go copyBuffer(client, server, 1)
	copyBuffer(server, client, 2)

	//go io.Copy(server, client)
	//io.Copy(client, server)
}
