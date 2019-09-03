yum install zlib-devel
yum install zlib

服务端：注意目录下必需有cert.pem和key.pem
./sproxy -s :9001

客户端：
./sproxy -c :1081 remoteipaddress:9001

