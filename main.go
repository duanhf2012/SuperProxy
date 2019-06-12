package main

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
)

var remoteip string
var key string

var ssl bool

func main() {
	ssl = true
	key = "9"
	remoteip = "159.138.26.110:9001"

	if len(os.Args) < 2 {
		fmt.Printf("param is error!\n")
		return
	}

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	var l net.Listener
	var err error

	parts := strings.Split(os.Args[1], " ")
	for i := 2; i < len(os.Args); i++ {
		//
		parts = append(parts, os.Args[i])
	}

	for i := 0; i < len(parts); i++ {
		parts[i] = strings.Trim(parts[i], "\"")
		parts[i] = strings.Trim(parts[i], " ")
	}

	if parts[0] == "-s" {
		if len(parts) < 2 {
			fmt.Printf("-s param is erro!\n")
			return
		}

		listenip := parts[1]
		if strings.Index(listenip, ":") == -1 {
			fmt.Printf("listen port is error!\n")
			return
		}

		if ssl == true {
			cer, err := tls.LoadX509KeyPair("cert.pem", "key.pem")
			if err != nil {
				log.Println(err)
				return
			}
			config := &tls.Config{Certificates: []tls.Certificate{cer}}
			l, err = tls.Listen("tcp", listenip, config)
			if err != nil {
				log.Panic(err)
			}
		} else {
			l, err = net.Listen("tcp", listenip)
			if err != nil {
				log.Panic(err)
			}
		}

		fmt.Printf("Server start listen %s.\n", listenip)
	} else if parts[0] == "-c" {
		if len(parts) < 3 {
			fmt.Printf("-c param is error\n")
			return
		}
		localport := parts[1]
		if strings.Index(localport, ":") == -1 {
			fmt.Printf("isten port is error!\n")
			return
		}

		l, err = net.Listen("tcp", localport)
		if err != nil {
			log.Panic(err)
		}
		remoteip = parts[2]
		fmt.Printf("Server start listen %s,remote is %s.\n", localport, remoteip)
	} else {
		fmt.Printf("param is error!\n")
		return
	}

	for {

		client, err := l.Accept()
		if err != nil {
			log.Panic(err)
		}

		if parts[0] == "-s" {
			go handleFromClientRequest(client)
		} else {

			go handleFromWebClientRequest(client)

		}

	}
}

func XorEncodeStr(msg, key []byte) []byte {
	ml := len(msg)
	var pwd []byte
	for i := 0; i < ml; i++ {
		pwd = append(pwd, ((msg[i]) ^ 1))
	}

	return pwd
}

func XorDecodeStr(msg, key []byte) []byte {
	ml := len(msg)
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
				ret = XorEncodeStr(buf[0:nr], []byte(key))
			} else if encodeType == 2 {
				ret = XorDecodeStr(buf[0:nr], []byte(key))

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
	var server net.Conn
	var err error
	if ssl == true {
		conf := &tls.Config{
			InsecureSkipVerify: true,
		}
		server, err = tls.Dial("tcp", remoteip, conf)
		if err != nil {
			log.Println(err)
			return
		}
	} else {
		//获得了请求的host和port，就开始拨号吧
		server, err = net.Dial("tcp", remoteip)
		if err != nil {
			log.Println(err)
			return
		}
	}

	//获得了请求的host和port，就开始拨号吧
	/*
		server, err := net.Dial("tcp", remoteip)
		if err != nil {
			log.Println(err)
			return
		}
	*/
	go copyBuffer(server, client, 1)
	copyBuffer(client, server, 2)
}

func IsChinaHost(host string) bool {
	parts := strings.Split(host, ":")
	host = parts[0]

	address, err := net.LookupHost(host)
	if err != nil || len(address) < 1 {
		return false
	}

	resp, errs := http.Get(fmt.Sprintf("http://ip.taobao.com/service/getIpInfo.php?ip=%s", address[0]))
	if errs != nil {
		return false
	}

	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false
	}

	bfind := strings.Index(string(b), "中国")
	if bfind == -1 {
		return false
	}

	return true
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

	var method, host string
	fmt.Sscanf(string(bdcode[:bytes.IndexByte(bdcode[:], '\n')]), "%s%s", &method, &host)

	//获得了请求的host和port，就开始拨号吧
	server, err := net.Dial("tcp", host)
	if err != nil {
		fmt.Printf("Connect %s is fail!", host)
		log.Println(err)
		return
	}

	fmt.Printf("Connect %s is succ!", host)
	if method == "CONNECT" {
		ret := XorEncodeStr([]byte("HTTP/1.1 200 Connection established\r\n\r\n"), []byte(key))
		fmt.Fprint(client, string(ret))
	} else {
		server.Write(bdcode[:n])
	}

	//进行转发
	go copyBuffer(client, server, 1)
	copyBuffer(server, client, 2)
}
