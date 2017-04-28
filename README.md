# http-proxy-amqp
[![Go Report](https://goreportcard.com/badge/github.com/iyidan/http-proxy-amqp)](https://goreportcard.com/badge/github.com/iyidan/http-proxy-amqp)
## [Intro]

This is an amqp connection pool implementation. 

It provides HTTP APIs to access MQ (eg, rabbitmq)
And maintains a long connection to MQ to improve performance.

![Alt text](https://github.com/iyidan/http-proxy-amqp/raw/master/intro.jpg)

## [Install]
go version: go1.8.1+
```shell
$ go get github.com/iyidan/http-proxy-amqp

$ http-proxy-amqp -h
Usage of http-proxy-amqp:
  -config string
    	The config file
  -debug
    	if true, will print pool stats per requests
  -dsn string
    	The amqp address
  -httpListenAddr string
    	http api listen address
  -maxChannelsPerConnection int
    	The max channels per connection
  -maxConnections int
    	The max connections for this process
  -maxIdleChannels int
    	The max idle channels for this process
  -minConnections int
    	The min connections keeped for this process

```

## [Start]
It is recommended to use supervisord to start
```go
./http-proxy-amqp -config=path_to_config_file.json
```
_path_to_config_file.json_
```json
{
    // DSN is the amqp address
    // The format is amqp://user:password@host:port/vhost
    // such as amqp://iyidan:123456@127.0.0.1:5672//aaa (the created vhost is /aaa not aaa)
    // Notice: user/password must be urlencoded if necessary
    // Notice: vhost maybe have a leading slash
    "dsn":"",

    "maxChannelsPerConnection":20000,
    "maxIdleChannels":500,
    "maxConnections":2000,
    "minConnections":5,

    // http api address
    "httpListenAddr":"127.0.0.1:35673"
}
```

## [APIs]
<ul>
    <li>
        <code>POST /confirm_send?exchange=$exchange&routingKey=$routingKey</code><br/>
        <p>send a persistent message with confirm mode</p>
        <p>The Response is <code>OK</code> if success</p>
    </li>
</ul>

## [Example]
`curl -XPOST 'http://127.0.0.1:35673/confirm_send?exchange={xx}&routingKey={xx}' -d 'msg'`<br/>
`OK`