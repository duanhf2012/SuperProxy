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
var localProxy string //用于非境外ip不翻墙
var ssl bool

var mapDomain Map

func main() {
	ssl = true
	if len(os.Args) < 2 {
		fmt.Printf("param is error!\n")
		return
	}

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	var l net.Listener
	var err error

	parts := strings.Split(os.Args[1], " ")
	for i := 2; i < len(os.Args); i++ {
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
			fmt.Printf("listen port is error!\n")
			return
		}

		l, err = net.Listen("tcp", localport)
		if err != nil {
			log.Panic(err)
		}
		remoteip = parts[2]

		if len(parts) > 3 {
			//
			localProxy = parts[3]
		}
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
				ret = XorEncodeStr(buf[0:nr], nil)
			} else if encodeType == 2 {
				ret = XorDecodeStr(buf[0:nr], nil)

			} else {
				ret = buf
			}

			//fmt.Printf("cpyb:[%s]", string(ret[0:nr]))
			nw, ew := dst.Write(ret[0:nr])

			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				//fmt.Printf("error1:%d,%s\n", encodeType, ew)

				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				//fmt.Printf("error2:%d,%s\n", encodeType, ew)

				break
			}
		}
		if er != nil {
			//fmt.Printf("error3:%d,%s\n", encodeType, er)
			if er != io.EOF {
				err = er
			}
			break
		}
	}
	return written, err
}

func handleFromWebClientRequest(client net.Conn) {
	var tmpRemoteip string
	if client == nil {
		return
	}

	defer client.Close()
	var b [4096]byte
	n, rerr := client.Read(b[:])
	if rerr != nil {
		log.Println(rerr)
		return
	}
    if n<4 {
	  return
	}
	//fmt.Printf("=====recv:[%s,%d,%+v]\n\n", string(b[:]),n,b)
	var method, host string
	fmt.Sscanf(string(b[:bytes.IndexByte(b[:n], '\n')]), "%s%s", &method, &host)

	if localProxy != "" && IsChinaHost(host) == true {
		//fmt.Printf("host:%s proxy is false\n", host)
		tmpRemoteip = localProxy
	} else {
		//fmt.Printf("host:%s proxy is true\n", host)
		tmpRemoteip = remoteip
	}

	var server net.Conn
	var err error
	if ssl == true {
		conf := &tls.Config{
			InsecureSkipVerify: true,
		}
		server, err = tls.Dial("tcp", tmpRemoteip, conf)
		if err != nil {
			log.Println(err)
			return
		}
	} else {
		//获得了请求的host和port，就开始拨号吧
		server, err = net.Dial("tcp", tmpRemoteip)
		if err != nil {
			log.Println(err)
			return
		}
	}

	defer server.Close()
	transMsg := XorEncodeStr(b[:n], nil)
	_, ew := server.Write(transMsg[:n])
	if ew != nil {
		log.Println(err)
		return
	}
	go copyBuffer(server, client, 1)
	copyBuffer(client, server, 2)
}

func IsChinaHost(host string) bool {
	parts := strings.Split(host, ":")
	host = parts[0]
	has := mapDomain.Get(host)
	if has != nil {
		return has.(bool)
	}

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
		mapDomain.Set(host, false)
		return false
	}
	mapDomain.Set(host, true)
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

	bdcode := XorDecodeStr(b[:n], nil)
	//	fmt.Printf("\n<<%s>>\n", string(bdcode[:]))
	var method, host string
	findId := bytes.IndexByte(bdcode[:], '\n')
	if findId == -1 {
		//		fmt.Printf("Data is error:%s\n", bdcode)
		return
	}
	fmt.Sscanf(string(bdcode[:findId]), "%s%s", &method, &host)

	strmsg := string(bdcode[:])
	//fmt.Printf("......%s\n", strmsg)
	if strings.Index(strmsg, "CONNECT") == -1 {
		start := strings.Index(strings.ToLower(strmsg), "host:")
		if start == -1 {
			//fmt.Printf("fail:%s\n", host)
			return
		}
		end := strings.Index(strmsg[start+5:], "\n")
		if end == -1 {
			//fmt.Printf("222fail:%s\n", host)
			return
		}
		start += 5
		//fmt.Printf("[%s,%d,%d]", strmsg, start, end)
		substr := strmsg[start : start+end]
		substr = strings.TrimSpace(substr)
		parts := strings.Split(substr, ":")
		if len(parts) < 2 {
			substr += ":80"
		}

		host = substr
		//fmt.Printf("..........%s...\n", substr)
	}

	//获得了请求的host和port，就开始拨号吧
	server, err := net.Dial("tcp", host)
	if err != nil {
		fmt.Printf("Connect %s is fail!\n", host)
		log.Println(err)
		return
	}

	defer server.Close()
	fmt.Printf("Connect %s is succ!\n", host)
	if method == "CONNECT" {
		ret := XorEncodeStr([]byte("HTTP/1.1 200 Connection established\r\n\r\n"), nil)
		fmt.Fprint(client, string(ret))
	} else {
		server.Write(bdcode[:n])
	}

	//进行转发
	go copyBuffer(server, client, 2)
	copyBuffer(client, server, 1)

}
